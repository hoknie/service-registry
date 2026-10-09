#!/usr/bin/env bash
# Creates an empty reversible migration pair under migrations/.
set -euo pipefail
name="${1:?usage: new-migration.sh <snake_case_name>}"
if [[ ! "$name" =~ ^[a-z0-9_]+$ ]]; then
  echo "error: name must be snake_case ([a-z0-9_]+), got '$name'" >&2
  exit 2
fi
ts=$(date -u +%Y%m%d%H%M%S)
up="migrations/${ts}_${name}.up.sql"
down="migrations/${ts}_${name}.down.sql"
printf -- '-- %s: one table per migration; ids are uuid v7 from the service (no DEFAULT).\n' "$name" > "$up"
printf -- '-- Undo everything %s did.\n' "$(basename "$up")" > "$down"
echo "created $up"
echo "created $down"
