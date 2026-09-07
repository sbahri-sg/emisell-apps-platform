#!/usr/bin/env node
import { run } from '../src/cli.mjs';

try {
  await run(process.argv.slice(2));
} catch (error) {
  console.error(`Emisell: ${error.message}`);
  process.exitCode = 1;
}
