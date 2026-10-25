# Metrics

The registry can publish its metrics for Prometheus at `GET /metrics`: HTTP traffic, background jobs and their queues,
deployments reported from CI, MCP usage per organization and project, calls to external systems and figures of the
catalog.

## Turning metrics on

Metrics are off by default. Set the variables of the server:

| Variable | Default | Meaning |
|---|---|---|
| `METRICS_ENABLED` | `false` | `true` collects metrics and serves `/metrics`; `false` collects nothing, and `/metrics` is a page of the interface as before |
| `METRICS_TOKEN` | — | a token of 16–256 characters, or `env:NAME` / `file:/path` to read it from; when set, `/metrics` needs `Authorization: Bearer <token>` |
| `METRICS_REFRESH_SECS` | `60` | how often figures that need the database or the runtime are sampled, 10–3600 seconds |

Without `METRICS_TOKEN` the page is open to everyone who reaches the server, and the server logs a warning at start.
Set a token, or keep `/metrics` closed at the reverse proxy.

The page never computes anything when it is requested. Counters change when things happen; figures from the database,
the runtime and the connection pool are sampled in the background every `METRICS_REFRESH_SECS` and the page returns
the last sample. If the database is down, the page still answers at once with the last values.

Every server process publishes its own values. With several replicas, scrape each of them: counters start from zero
when a process restarts (Prometheus handles this in `rate` and `increase`), and figures from the database are the same
on every replica — use `max` over replicas for them.

## Scraping

```yaml
scrape_configs:
  - job_name: svc-registry
    scrape_interval: 30s
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/svc-registry-token
    static_configs:
      - targets: ["registry-1:8080", "registry-2:8080"]
```

## What is published

All names start with `svcr_`, except the standard `go_*` and `process_*` metrics of the Go runtime.

### Server and runtime

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_http_requests_total` | `method`, `route`, `status` | requests answered; `route` is the API route template (`/api/v1/catalog/nodes/:id`), `web` for pages of the interface, `unmatched` for unknown `/api` paths |
| `svcr_http_request_duration_seconds` | `method`, `route` | time to answer |
| `svcr_http_requests_in_flight` | — | requests being answered now |
| `svcr_db_pool_connections` | `state` | database connections: `acquired`, `idle`, `constructing` |
| `svcr_db_pool_max_connections` | — | the pool limit (`DATABASE_MAX_CONNECTIONS`) |
| `svcr_db_pool_acquires_total`, `svcr_db_pool_empty_acquires_total`, `svcr_db_pool_acquire_wait_seconds_total` | — | connections taken, takes that had to wait, time spent waiting |
| `svcr_build_info` | `version`, `go_version` | always `1` |
| `svcr_start_time_seconds` | — | when the process started |
| `svcr_metrics_sample_timestamp_seconds` | `source` | when figures were last sampled: `runtime`, `pool`, `catalog`, `jobs` |
| `svcr_metrics_sample_errors_total` | `source` | samples that failed; the previous values stay |

### Background jobs

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_job_runs_total` | `job`, `outcome` | runs: `succeeded`, `failed` or `panicked` |
| `svcr_job_run_duration_seconds` | `job` | duration of runs |
| `svcr_job_queue` | `job`, `state` | items waiting (`due`) and being worked on (`leased`) for `forge_sync`, `knowledge_collection`, `knowledge_index`, `link_checks`, `cluster_polls`, `notify_deliveries` |

Other values of `job` are the periodic clean-ups (`event_retention`, `branch_retention`, `notify_retention`, …) and
`link_refresh`, the hourly expansion of links of all projects.

### Deployments and events

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_ingest_events_total` | `type`, `outcome`, `code` | events from CI: `accepted`, `replayed` (the same idempotency key again) or `rejected` with the error code |
| `svcr_deployments_total` | `source` | deployments recorded: `event` or `cluster` |
| `svcr_agent_reports_total` | `outcome` | reports of cluster agents |
| `svcr_forge_webhooks_total` | `outcome` | webhooks from forges |
| `svcr_deployments_recent` | `window` | deployments in the last `1d` and `7d` |

### Catalog

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_catalog_nodes` | `kind` | organizations, folders and projects |
| `svcr_projects_covered` | `aspect` | projects with `owners`, collected `docs`, `deployments`, `links`, `dependencies`, or imported from a `forge` |
| `svcr_project_info` | `project_id`, `organization_id`, `path` | one series per project with its slug path, for joining |
| `svcr_users` | `state` | `active`, `disabled` and `service` users |
| `svcr_users_active` | `window` | users who used a session or a personal token in the last `1d`, `7d`, `30d` |
| `svcr_groups`, `svcr_sessions_active` | — | groups, open sessions |
| `svcr_credentials_active` | `kind` | usable personal tokens, project keys, agent tokens, OAuth grants |
| `svcr_service_environments`, `svcr_drifts` | — | current versions, and those that differ from what clusters run |
| `svcr_clusters` | `mode`, `state` | clusters that are `ok`, `failed` or `silent` (an agent stopped reporting) |
| `svcr_cluster_workloads` | — | workloads observed in clusters |
| `svcr_branches` | `state` | `active`, `stale` and `gone` branches |
| `svcr_forge_connections` | `forge`, `state` | forge connections whose last run is `ok` or `failed` |
| `svcr_link_targets` | `state` | link addresses that are `ok`, `broken` or not checked yet |
| `svcr_notification_subscriptions` | `channel` | `webhook` and `slack` subscriptions |
| `svcr_knowledge_snapshots`, `svcr_knowledge_files_pending` | — | documentation snapshots, files waiting for embeddings |
| `svcr_knowledge_searches_total` | `mode`, `source`, `outcome` | documentation searches from the API and from MCP |
| `svcr_logins_total` | `method`, `outcome` | sign-ins by password or `oauth:<provider>`: `succeeded`, `failed`, `throttled` |

### MCP

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_mcp_calls_total` | `kind`, `name`, `outcome` | tool calls and resource reads by name, `ok` or `error` |
| `svcr_mcp_call_duration_seconds` | `kind`, `name` | duration of calls |
| `svcr_mcp_project_calls_total` | `organization_id`, `project_id` | calls addressed to a project the caller can read |

### External systems

| Metric | Labels | Meaning |
|---|---|---|
| `svcr_outbound_requests_total` | `target`, `result` | requests to `forge`, `oidc`, `embeddings`, `qdrant`, `meilisearch`, `kubernetes`, `notify`, `link_check`, `client_metadata`; `result` is `2xx`…`5xx` or `error` when there was no answer |
| `svcr_outbound_request_duration_seconds` | `target` | duration of requests |

Labels never carry email addresses, tokens, URLs or request paths.

## Useful queries

```promql
# Share of server errors on the API
sum(rate(svcr_http_requests_total{status=~"5..",route!="web"}[5m])) / sum(rate(svcr_http_requests_total{route!="web"}[5m]))

# Rejected CI events by reason
sum by (code) (increase(svcr_ingest_events_total{outcome="rejected"}[1h]))

# Figures from the database are stale
time() - max(svcr_metrics_sample_timestamp_seconds{source="catalog"}) > 300

# MCP calls per project over a day, with project paths
sum by (path) (increase(svcr_mcp_project_calls_total[1d]) * on (project_id) group_left (path) max by (project_id, path) (svcr_project_info))

# Failing external systems
sum by (target) (rate(svcr_outbound_requests_total{result=~"5xx|error"}[5m]))
```
