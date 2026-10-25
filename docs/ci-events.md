# Reporting deployments from CI

Add one step to your deploy pipeline and the registry learns what was deployed where. It keeps the current version
of every service in every environment, the history of releases, and, optionally, links and dependencies of the
rollout.

## What you need

- **The project key** (`svcr_…`). It is shown once when the project is created; issue a new one under
  **Settings → Keys** of the project. Store it as a secret of your CI, e.g. `SVCR_PROJECT_KEY`.
- **The project id** and **the registry address**, e.g. `SVCR_PROJECT_ID` and `SVCR_URL`.

**Settings → Sending events** of the project shows ready snippets with your project id filled in.

## Ready snippets

- [`examples/ci/github-actions.yml`](../examples/ci/github-actions.yml) — a GitHub Actions step;
- [`examples/ci/gitlab-ci.yml`](../examples/ci/gitlab-ci.yml) — a GitLab CI job;
- [`examples/ci/curl.sh`](../examples/ci/curl.sh) — plain `curl`, driven by environment variables. `CI_RUN_URL`
  becomes a `runbook` link and `DEPENDENCIES` is a JSON array of dependencies.

All of them read `SVCR_URL`, `SVCR_PROJECT_ID` and `SVCR_PROJECT_KEY`.

## What to send

Send a `POST` with `Content-Type: application/json` and the header `Authorization: Bearer <project key>` to

```
<SVCR_URL>/api/v1/ingest/projects/<SVCR_PROJECT_ID>/events
```

The body is one event, at most 64 KiB:

```json
{
  "type": "service.deployed",
  "version": 1,
  "occurred_at": "2026-06-01T12:00:00Z",
  "idempotency_key": "pipeline-4711",
  "payload": {
    "service": "api",
    "version": "1.4.2",
    "environment": "production",
    "commit_sha": "1a2b3c4",
    "branch": "main"
  }
}
```

| Field | Required | What to put |
|---|---|---|
| `type`, `version` | yes | `service.deployed` and `1` |
| `occurred_at` | yes | when the rollout happened, with a time zone; not more than 5 minutes ahead of the registry's clock |
| `idempotency_key` | yes | a unique id of this rollout in the project, e.g. the pipeline id; up to 200 visible characters |
| `payload.service` | yes | the service name; lowercase letters, digits, `.`, `_`, `-` |
| `payload.version` | yes | the version, tag or commit you deployed |
| `payload.environment` | yes | the environment, same rules as the service |
| `payload.commit_sha`, `payload.branch` | no | the commit and branch; the branch becomes a branch of the project |
| `payload.cluster`, `payload.namespace` | no | where it runs |
| `payload.url` | no | the address of the deployed service |
| `payload.deployed_by` | no | who started the rollout |
| `payload.metadata` | no | up to 32 string pairs of your own |
| `payload.links` | no | links of this rollout (below) |
| `payload.dependencies` | no | what this service depends on (below) |

Good to know:
- **Retries are safe.** Send the same event with the same `idempotency_key` again and nothing is duplicated. Compute
  `occurred_at` once per rollout, before the retries. Reusing the key for a *different* event is refused.
- **Late events.** The current version of an environment is the rollout with the latest `occurred_at`, so an event
  that arrives late only extends the history.
- **Errors.** When the registry refuses an event, the CI step fails, and the answer names every wrong field.

| Variable | Default | Meaning |
|---|---|---|
| `INGEST_RATE_LIMIT_PER_MINUTE` | 600 | events a minute per project key |
| `INGEST_EVENT_RETENTION_DAYS` | 90 | how long raw events are kept; the release history stays with the project |

## Links of a rollout

`payload.links` holds up to 20 links, for example the pipeline run and the logs of this version:

```json
"links": [
  {"kind": "runbook", "url": "https://ci.example/runs/42", "title": "Pipeline"},
  {"kind": "logs", "key": "logs-raw", "url": "https://logs.example/?q=api"}
]
```

- `kind` — a link kind such as `logs`, `grafana`, `runbook`. An unknown kind gets a generic icon.
- `url` — an `http://` or `https://` address.
- `key` — optional, defaults to the kind. Two links of the same kind need different keys.
- `title` — optional, shown instead of the kind name.

The links appear in the release history. While the rollout is the current one for its service and environment,
they also join the project's [observability links](observability-links.md). There they replace the template link
with the same key for that service and environment. When another rollout becomes current, they disappear.

## Dependencies of a rollout

`payload.dependencies` lists up to 50 things this service depends on:

```json
"dependencies": ["acme/backend/billing", "external:postgres-main"]
```

- A dependency is another project by its path or id, or an external system `external:<name>`.
- A path that matches no project yet is kept and resolved later.
- The list replaces the dependencies previously reported for this service and environment, but only when the
  rollout became the current one.
- Leaving the field out changes nothing; `[]` clears the list.
- Dependencies from other sources and manual ones are not touched ([dependencies.md](dependencies.md)).

## Refreshing documentation on deployment

A project whose documentation refreshes **On events** with **Deployment from CI** selected (the default) collects it
right after every new event; see
[project-documentation.md](project-documentation.md#when-documentation-is-refreshed).
