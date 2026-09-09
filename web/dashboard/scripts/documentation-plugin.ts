import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import type { Plugin } from 'vite';

// Shared by both local frontend entrypoints; generated data is identical.
export function documentationPlugin(): Plugin {
  const directory = fileURLToPath(new URL('../content/docs/', import.meta.url));
  const script = fileURLToPath(new URL('./documentation.mjs', import.meta.url));
  const generate = () =>
    execFileSync(process.execPath, [script], { stdio: 'pipe' });
  return {
    name: 'emisell-public-documentation',
    buildStart() {
      generate();
    },
    configureServer(server) {
      server.watcher.add(directory);
      const update = (path: string) => {
        if (!path.startsWith(directory)) return;
        try {
          generate();
          server.ws.send({ type: 'full-reload' });
        } catch (error) {
          server.ws.send({
            type: 'error',
            err: {
              message:
                error instanceof Error
                  ? error.message
                  : 'Dokumentasi tidak valid.',
              stack: '',
            },
          });
        }
      };
      for (const event of ['add', 'change', 'unlink'])
        server.watcher.on(event, update);
      server.httpServer?.once('close', () => {
        for (const event of ['add', 'change', 'unlink'])
          server.watcher.off(event, update);
      });
    },
  };
}
