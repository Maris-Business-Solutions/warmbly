#!/bin/sh
# Installs the harness: dependencies, Playwright's Chromium and the small images every stack runs. Then checks it all.
set -eu
cd "$(dirname "$0")/.."
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
for image in redis:7-alpine nats:2.10-alpine axllent/mailpit:latest; do
  docker image inspect "$image" >/dev/null 2>&1 || docker pull "$image"
done
exec node scripts/doctor.ts
