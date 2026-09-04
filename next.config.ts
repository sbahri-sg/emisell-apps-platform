import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Vinext emits a self-contained Node.js server under dist/standalone. The
  // production image copies only that runtime instead of the source tree and
  // development dependencies.
  output: 'standalone',
};

export default nextConfig;
