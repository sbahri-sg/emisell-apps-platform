import { reactRouter } from '@react-router/dev/vite';
import { defineConfig } from 'vite';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('.', import.meta.url));

export default defineConfig({
  plugins: [reactRouter()],
  // Backend .env is loaded by server/backend.mjs, never by the frontend bundler.
  envDir: false,
  envPrefix: 'EMISELL_PUBLIC_',
  server: {
    host: '127.0.0.1', strictPort: true, cors: false,
    fs: {
      strict: true,
      allow: [resolve(root, 'app'), resolve(root, 'public'), resolve(root, 'node_modules')],
      deny: ['.env*', '.git/**', '.local/**', 'server/**', 'private/**', 'emisell.app.json']
        .map(pattern => root + pattern)
        .concat(['**/*.server.*', '**/*.secret', '**/*.pem', '**/*.key']),
    },
  },
});
