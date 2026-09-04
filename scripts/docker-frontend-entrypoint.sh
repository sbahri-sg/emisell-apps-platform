#!/bin/sh
set -eu

# Vinext records the dev-server PID in the project cache. Container recreation
# can leave that PID behind even though the previous process no longer exists.
rm -f /workspace/.vinext/dev/lock.json

exec npm run dev -- --host 0.0.0.0 --port 3003
