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
`metadata` (string → string), `links` and `dependencies` (see [below](#links-and-dependencies)). `201` — accepted and applied; `200` with `"replayed": true` — the
same event was sent before (same `idempotency_key` and content), nothing duplicated; `409
conflict.idempotency_key_reused` — that key was used for a different event; `422` —
`ingest.unknown_event_type`, `ingest.unsupported_version` or `ingest.invalid_event` with
`fields: [{"path", "code"}]`; `429 ingest.rate_limited` (`INGEST_RATE_LIMIT_PER_MINUTE` per key).
The current version of an environment is the deployment with the latest `occurred_at`, so late
events only extend the history. Compute `occurred_at` once per event, before retries.

## Links and dependencies

Two more optional payload fields describe the rollout beyond its version:

```json
"payload": {
  "service": "api", "version": "1.4.2", "environment": "production",
  "links": [
    {"kind": "runbook", "url": "https://ci.example/runs/42", "title": "Pipeline"},
    {"kind": "logs", "key": "logs-raw", "url": "https://logs.example/?q=api"}
  ],
  "dependencies": ["acme/backend/billing", "external:postgres-main"]
}
```

**`links`** — at most 20 links of this rollout; `[]` means none. Each link is an object:
- `kind` — required; a link kind key (`logs`, `grafana`, `runbook`, …; trimmed and lower-cased,
  1–63 characters `a-z`, `0-9`, `-`). The kind does not have to exist: an unknown kind gets a generic
  icon and is sorted last.
- `url` — required; an absolute `http://` or `https://` address up to 2048 characters.
- `key` — optional, defaults to `kind`. Keys must differ within one event, so two links of the same kind
  need their own keys.
- `title` — optional, 1–100 characters; shown instead of the kind name.

Links are stored with the deployment in that form (`{"key", "kind", "url", "title"}`, `title` is
`null` when not given) and shown in the deployment history and on the environment. While the deployment is the
current one for its service and environment, its links also appear among the project's
[observability links](observability-links.md) with `"source": "event"`. Such a link replaces the
template link with the same key for that service and environment only. Links of a service-agnostic template and
of other environments stay. They are checked like any other link. When another deployment becomes current,
including one seen only in a cluster, the event's links stop showing.

**`dependencies`** — at most 50 references to what this service depends on: another project by slug path
(`acme/backend/billing`, any case) or UUID, or an external system `external:<name>`. Repeats collapse into
one. A path that matches no project is not an error; it is kept unresolved and resolved again on the next
report. The list replaces the edges of this service and environment (`<service>/<environment>`), but only
when the deployment became current. A late event changes nothing, an absent field changes nothing, and
`[]` clears them. Other sources and manual edges are not touched. See
[dependencies.md](dependencies.md#sources).

Errors come back as `422 ingest.invalid_event` with paths:
- `payload.links` — not an array (`invalid_type`) or more than 20 links (`invalid_value`);
- `payload.links.<i>` — not an object (`invalid_type`);
- `payload.links.<i>.<field>`, e.g. `payload.links.0.url` — `required`, `invalid_value`; a repeated key is
  reported on `key` of the second and later links;
- `payload.dependencies` — not an array (`invalid_type`) or more than 50 entries (`invalid_value`);
- `payload.dependencies.<i>` — not a string (`invalid_type`) or not a valid reference (`invalid_value`).

## Snippets

Ready-made snippets (the project page shows them with your project id under
**Settings → Sending events**):

- [`examples/ci/github-actions.yml`](../examples/ci/github-actions.yml) — a GitHub Actions step;
- [`examples/ci/gitlab-ci.yml`](../examples/ci/gitlab-ci.yml) — a GitLab CI job;
- [`examples/ci/curl.sh`](../examples/ci/curl.sh) — plain `curl`, driven by environment variables (`CI_RUN_URL` becomes a `runbook` link, `DEPENDENCIES` a JSON array of dependencies).

All of them read `SVCR_URL`, `SVCR_PROJECT_ID` and the secret `SVCR_PROJECT_KEY` from CI
settings. Accepted raw events are kept `INGEST_EVENT_RETENTION_DAYS` (90) days; the deployment
history is kept with the project. A project whose documentation refreshes **On events** collects it right after
every new event ([project-documentation.md](project-documentation.md#when-documentation-is-refreshed)).
