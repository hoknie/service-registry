#!/usr/bin/env bash
# `just skills-seed`: imports three public skill repositories into the global skill folder `examples` of a
# running server (local development only). Repeated runs skip folders that already exist.
set -euo pipefail

BASE="${SVCR_URL:-http://localhost:8080}"
EMAIL="${BOOTSTRAP_ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD:-change-me-please}"
jar="$(mktemp -t svc-registry-seed.XXXXXX)"
trap 'rm -f "$jar"' EXIT

api() {
  curl -fsS -b "$jar" -c "$jar" -H 'Content-Type: application/json' -H "Origin: $BASE" "$@"
}

id_of_slug() {
  tr '{' '\n' | { grep "\"slug\":\"$1\"" || true; } | sed -E 's/.*"id":"([0-9a-f-]+)".*/\1/' | head -n 1
}

api -o /dev/null -X POST "$BASE/api/v1/auth/login" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"

examples="$(api "$BASE/api/v1/skills/folders" | id_of_slug examples)"
if [ -z "$examples" ]; then
  examples="$(api -X POST "$BASE/api/v1/skills/folders" \
    -d '{"parent_id":null,"organization_id":null,"slug":"examples","name":"Examples"}' | id_of_slug examples)"
fi
echo "examples: $examples"

existing="$(api "$BASE/api/v1/skills/folders?parent_id=$examples")"

seed() {
  local slug="$1" body="$2"
  if [ -n "$(printf '%s' "$existing" | id_of_slug "$slug")" ]; then
    echo "skip $slug: already imported"
    return
  fi
  api -o /dev/null -X POST "$BASE/api/v1/skills/imports" \
    -d "{\"parent_id\":\"$examples\",\"slug\":\"$slug\",\"kind\":\"remote\",\"hosting\":\"github\",$body}"
  echo "imported $slug"
}

seed apollo '"name":"Apollo GraphQL","url":"https://github.com/apollographql/skills/tree/main/skills"'
seed jeffallan '"name":"Claude Skills (Jeffallan)","url":"https://github.com/Jeffallan/claude-skills/tree/main/skills"'
seed cybersecurity '"name":"Cybersecurity","url":"https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills","layout":"field","group_field":"subdomain","status":"disabled"'
