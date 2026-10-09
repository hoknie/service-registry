#!/usr/bin/env bash
# `just web-watch`: rebuild the static web export (web/out) whenever a web/ source changes, so a
# running `just dev` serves the new UI without a restart — reload the page by hand,
# there is no HMR. Polls once a second with `find -newer` (no fswatch/watchexec needed).
# Limitation: deleting a file without touching any other one is not noticed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PNPM="${PNPM:-pnpm}"
SOURCES=(web/src web/public web/next.config.ts web/package.json web/pnpm-lock.yaml web/tsconfig.json)

stamp="$(mktemp -t svc-registry-web-watch.XXXXXX)"
trap 'rm -f "$stamp"' EXIT

scripts/dev.sh web
touch "$stamp"
echo "web-watch: watching ${SOURCES[*]} (Ctrl-C to stop)" >&2

while sleep 1; do
  changed="$(find "${SOURCES[@]}" -newer "$stamp" -type f -print -quit 2>/dev/null || true)"
  [[ -z "$changed" ]] && continue
  touch "$stamp"
  echo "web-watch: $changed changed — pnpm build" >&2
  if (cd web && "$PNPM" build); then
    echo "web-watch: export rebuilt — reload the page" >&2
  else
    echo "web-watch: build failed (see above); waiting for the next change" >&2
  fi
done
