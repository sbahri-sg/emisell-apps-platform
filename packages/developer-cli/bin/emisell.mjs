#!/usr/bin/env node
import { run } from '../src/cli.mjs';

try {
  const code = await run(process.argv.slice(2));
  if (Number.isInteger(code)) process.exitCode = code;
} catch (error) {
  console.error(`Emisell: ${error.message}`);
  process.exitCode = 1;
}
