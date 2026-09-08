import { request } from 'node:http';

// Probe this container only. No environment-provided remote URL or credential.
const port = Number(process.env.PORT || 3000);
if (!Number.isInteger(port) || port < 1 || port > 65535) process.exit(1);
const req = request({ hostname: '127.0.0.1', port, path: '/health/ready', method: 'GET', agent: false }, res => {
  res.resume();
  res.once('end', () => { process.exitCode = res.statusCode === 200 ? 0 : 1; });
});
req.setTimeout(3000, () => req.destroy());
req.once('error', () => { process.exitCode = 1; });
req.end();
