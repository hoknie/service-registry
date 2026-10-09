#!/usr/bin/env bash
# `just dev-check`: the preparation steps of `just dev` are idempotent (spec `platform/build`).
# Runs `scripts/dev.sh db` and `scripts/dev.sh web` twice: the first pass may start the database
# or build the export; the second must change nothing — same running container (same id and
# start time) and an untouched web/out/404.html.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

container() {
  local id
  id="$(docker compose ps -q pgsql)"
  docker inspect -f '{{.Id}} {{.State.StartedAt}}' "$id"
}
stamp="$(mktemp -t svc-registry-dev-check.XXXXXX)"
trap 'rm -f "$stamp"' EXIT

scripts/dev.sh db
scripts/dev.sh web
before_db="$(container)"
sleep 1
touch "$stamp"

scripts/dev.sh db
scripts/dev.sh web
after_db="$(container)"

status=0
if [[ "$before_db" != "$after_db" ]]; then
  echo "dev-check: FAIL — pgsql was restarted ($before_db → $after_db)" >&2
  status=1
fi
if [[ -n "$(find web/out/404.html -newer "$stamp")" ]]; then
  echo "dev-check: FAIL — web/out was rebuilt although it was up to date" >&2
  status=1
fi
if (( status == 0 )); then
  echo "dev-check: ok — pgsql kept running (${before_db:0:12}), web/out not rebuilt"
fi
exit "$status"
