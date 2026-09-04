import { sites } from '@openai/sites-vite-plugin';
import tailwindcss from '@tailwindcss/postcss';
import vinext from 'vinext';
import { defineConfig, loadEnv, type Plugin } from 'vite';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import hostingConfig from './.openai/hosting.json';

const SITE_CREATOR_PLACEHOLDER_DATABASE_ID =
  '00000000-0000-4000-8000-000000000000';

const { d1, r2 } = hostingConfig;

// macOS Seatbelt blocks FSEvents, so Codex previews need polling for HMR.
const isCodexSeatbeltSandbox = process.env.CODEX_SANDBOX === 'seatbelt';
const isDocker = process.env.VITE_DOCKER === 'true';
const usePolling =
  isCodexSeatbeltSandbox || process.env.VITE_USE_POLLING === 'true';

const localBindingConfig = {
  main: 'vinext/server/app-router-entry',
  compatibility_flags: ['nodejs_compat'],
  d1_databases: d1
    ? [
        {
          binding: d1,
          database_name: 'site-creator-d1',
          database_id: SITE_CREATOR_PLACEHOLDER_DATABASE_ID,
        },
      ]
    : [],
  r2_buckets: r2
    ? [
        {
          binding: r2,
          bucket_name: 'site-creator-r2',
        },
      ]
    : [],
};

export default defineConfig(async ({ mode }) => {
  const frontendEnv = loadEnv(mode, process.cwd(), '');
  const allowedHosts = (
    process.env.VITE_ALLOWED_HOSTS ??
    frontendEnv.VITE_ALLOWED_HOSTS ??
    'apps-platform.emisell.com'
  )
    .split(',')
    .map((host) => host.trim())
    .filter(Boolean);
  // Keep Wrangler and Miniflare state project-local. These are non-secret tool
  // settings; application environment belongs in ignored `.env*` files.
  process.env.WRANGLER_WRITE_LOGS ??= 'false';
  process.env.WRANGLER_LOG_PATH ??= '.wrangler/logs';
  process.env.MINIFLARE_REGISTRY_PATH ??= '.wrangler/registry';

  // Wrangler snapshots its log path while the Cloudflare plugin is imported.
  const { cloudflare } = await import('@cloudflare/vite-plugin');

  return {
    css: { postcss: { plugins: [tailwindcss()] } },
    server: {
      // Keep Vite host-header protection enabled and allow only explicitly
      // configured deployment hostnames. Never use allowedHosts: true.
      allowedHosts,
      // Preserve Vite's default sensitive-file deny rules and block raw docs
      // URLs (including /@fs and ?raw/?import) from bypassing operator auth.
      // Server module loading can still read the source contracts internally.
      fs: { deny: ['.env', '.env.*', '*.{crt,pem}', '**/.git/**', `${process.cwd().replaceAll('\\', '/')}/docs/**`, `${process.cwd().replaceAll('\\', '/')}/lib/documentation/developer-guide.mjs`] },
      ...(isDocker ? { host: '0.0.0.0', port: 3003 } : {}),
      ...(usePolling ? { watch: { useFsEvents: false, usePolling: true } } : {}),
    },
    plugins: [
      {
        name: 'emisell:private-documentation',
        enforce: 'pre',
        resolveId(id) {
          if (id === 'virtual:emisell-documentation') return '\0virtual:emisell-documentation';
          if (id === 'virtual:emisell-developer-guide') return '\0virtual:emisell-developer-guide';
        },
        load(id) {
          if (id === '\0virtual:emisell-developer-guide') {
            if (this.environment.config.consumer === 'client') throw new Error('Developer guide source is server-only.');
            const path = resolve(process.cwd(), 'lib/documentation/developer-guide.mjs');
            this.addWatchFile(path);
            return readFileSync(path, 'utf8');
          }
          if (id !== '\0virtual:emisell-documentation') return;
          if (this.environment.config.consumer === 'client') throw new Error('Internal documentation is server-only.');
          // A server-only module bridge lets workerd load the source without
          // opening Vite's raw-file serving permission for the docs directory.
          return Object.entries({ platform: 'openapi.json', provider: 'provider-openapi.json', resource: 'emisell-resource-openapi.json', resourceBlueprint: 'emisell-resource-blueprint.openapi.json', shippingProvider: 'shipping-provider.openapi.json' })
            .map(([name, file]) => {
              const path = resolve(process.cwd(), 'docs', file);
              this.addWatchFile(path);
              return `export const ${name} = ${JSON.stringify(JSON.parse(readFileSync(path, 'utf8')))};`;
            }).join('\n');
        },
      } satisfies Plugin,
      vinext(),
      sites(),
      cloudflare({
        viteEnvironment: { name: 'rsc', childEnvironments: ['ssr'] },
        config: {
          ...localBindingConfig,
          // Non-secret server bindings. Host process env is not automatically
          // available inside the local workerd runtime used by Vinext.
          vars: {
            APP_GATEWAY_INTERNAL_URL: process.env.APP_GATEWAY_INTERNAL_URL ?? frontendEnv.APP_GATEWAY_INTERNAL_URL ?? 'http://localhost:8081',
            AUTH_SESSION_COOKIE_NAME: process.env.AUTH_SESSION_COOKIE_NAME ?? frontendEnv.AUTH_SESSION_COOKIE_NAME ?? 'emisell_session',
          },
        },
      }),
    ],
  };
});
