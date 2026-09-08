import { readProject } from '../src/project.mjs';
import { startBoundary } from '../templates/react-router/server/http.mjs';

// Isolated tests exercise the actual HTTP/security boundary, not Vite or a store.
// The separate framework smoke test builds/runs the generated React application.
export async function startTestServer(root, port = 0, adapters = {}) {
  return startBoundary({ config: await readProject(root), port, adapters, handler(req, res) {
    if (req.url === '/' || req.url === '/products') {
      res.writeHead(200, { 'Content-Type': 'text/html' }); res.end('<h1>Framework fixture</h1>');
    } else { res.writeHead(404); res.end(); }
  } });
}
