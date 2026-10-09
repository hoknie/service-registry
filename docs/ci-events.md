# Sending events from CI

Target repositories report rollouts to
`POST /api/v1/ingest/projects/{project_id}/events`, authenticated by a **project key**
(`Authorization: Bearer svcr_…`; issued when the project is created, rotated under the project's
“Ingest keys”). Every key failure — missing, malformed, revoked, expired, another project's — is
the same `401 auth.invalid_project_key`. The body is a versioned JSON envelope (at most 64 KiB):

```json
{
  "type": "service.deployed",
  "version": 1,
  "occurred_at": "2025-06-01T12:00:00Z",
  "idempotency_key": "run-1",
  "payload": {"service": "API", "version": "1.4.2", "environment": "Production"}
}
```

Optional payload fields: `commit_sha`, `branch`, `cluster`, `namespace`, `url`, `deployed_by`,
`metadata` (string → string). `201` — accepted and applied; `200` with `"replayed": true` — the
same event was sent before (same `idempotency_key` and content), nothing duplicated; `409
conflict.idempotency_key_reused` — that key was used for a different event; `422` —
`ingest.unknown_event_type`, `ingest.unsupported_version` or `ingest.invalid_event` with
`fields: [{"path", "code"}]`; `429 ingest.rate_limited` (`INGEST_RATE_LIMIT_PER_MINUTE` per key).
The current version of an environment is the deployment with the latest `occurred_at`, so late
events only extend the history. Compute `occurred_at` once per event, before retries.

Ready-made snippets (the project page shows them with your project id under
**Events → How to send events**):

- [`examples/ci/github-actions.yml`](../examples/ci/github-actions.yml) — a GitHub Actions step;
- [`examples/ci/gitlab-ci.yml`](../examples/ci/gitlab-ci.yml) — a GitLab CI job;
- [`examples/ci/curl.sh`](../examples/ci/curl.sh) — plain `curl`, driven by environment variables.

All of them read `SVCR_URL`, `SVCR_PROJECT_ID` and the secret `SVCR_PROJECT_KEY` from CI
settings. Accepted raw events are kept `INGEST_EVENT_RETENTION_DAYS` (90) days; the deployment
history is kept with the project. Specs: `openspec/specs/ingest/`, `openspec/specs/catalog/services/`.
