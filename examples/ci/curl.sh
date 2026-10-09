#!/usr/bin/env bash
# Report a deployment to svc-registry (spec ingest/events, ingest/service-deployed).
#
# Required environment:
#   SVCR_URL           registry address, e.g. https://registry.example.com
#   SVCR_PROJECT_ID    the project's id (catalog page → Events → How to send events)
#   SVCR_PROJECT_KEY   a key of that project (svcr_…) — keep it in your CI secrets
#   SERVICE, ENVIRONMENT, VERSION   what was deployed where
# Optional: COMMIT_SHA, BRANCH, DEPLOY_URL, DEPLOYED_BY, IDEMPOTENCY_KEY.
#
# occurred_at is computed ONCE, before the retries: a retry then resends the identical event,
# which the registry answers with 200 and the original result instead of a duplicate.
# Values are put into JSON as is: they must not contain double quotes or backslashes.
set -euo pipefail

: "${SVCR_URL:?}" "${SVCR_PROJECT_ID:?}" "${SVCR_PROJECT_KEY:?}"
: "${SERVICE:?}" "${ENVIRONMENT:?}" "${VERSION:?}"

OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
IDEMPOTENCY_KEY="${IDEMPOTENCY_KEY:-deploy-$SERVICE-$ENVIRONMENT-$VERSION-$OCCURRED_AT}"

curl --fail-with-body --silent --show-error --retry 3 --retry-all-errors -X POST \
  "$SVCR_URL/api/v1/ingest/projects/$SVCR_PROJECT_ID/events" \
  -H "Authorization: Bearer $SVCR_PROJECT_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "type": "service.deployed",
  "version": 1,
  "occurred_at": "$OCCURRED_AT",
  "idempotency_key": "$IDEMPOTENCY_KEY",
  "payload": {
    "service": "$SERVICE",
    "version": "$VERSION",
    "environment": "$ENVIRONMENT",
    "commit_sha": "${COMMIT_SHA:-}",
    "branch": "${BRANCH:-}",
    "url": "${DEPLOY_URL:-}",
    "deployed_by": "${DEPLOYED_BY:-}"
  }
}
JSON
echo
