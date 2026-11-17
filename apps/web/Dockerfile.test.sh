#!/bin/sh
set -eu

dockerfile=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/Dockerfile

# The build must stay on the official bun toolchain: bun.lock pins the
# resolution, so no registry forwarding exists anymore. The image must never
# reintroduce corepack/pnpm — GitLab BuildKit used to time out on
# registry.npmmirror.com while corepack fetched pnpm.
if grep -Eq 'corepack|pnpm|NPM_REGISTRY' "$dockerfile"; then
  echo "frontend/Dockerfile still references the pnpm/corepack toolchain" >&2
  exit 1
fi

grep -Fq 'oven/bun' "$dockerfile"
grep -Fq 'bun install --frozen-lockfile' "$dockerfile"
grep -Fq 'bun --bun run build' "$dockerfile"
