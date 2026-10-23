# Observing Kubernetes clusters

The registry polls the clusters a superadmin adds (Administration → Clusters) and shows what
really runs next to the deployments CI reports (`service.deployed`). Specs: `openspec/specs/deploy/`.

1. Apply `rbac.yaml` (adjust the namespaces): a ServiceAccount with `get`/`list` on
   Deployments, StatefulSets, DaemonSets, ReplicaSets, CronJobs, Jobs and Pods, and its token.
2. Add the cluster: API server address, its CA (PEM), the token (stored encrypted with
   `SECRETS_KEYS`, or a reference `env:NAME` / `file:/path`), the default environment. A registry
   running in the same cluster can use "The registry runs in this cluster" with its own
   ServiceAccount instead.
3. "Test connection" lists missing permissions.
4. Annotate workloads (on the object or its pod template — Helm charts usually set the latter):

```yaml
metadata:
  annotations:
    svc-registry.io/project: acme/backend/api   # slug path of the project, or its UUID
    svc-registry.io/service: api                # default: the workload's name
    svc-registry.io/environment: production     # default: the cluster's environment
    svc-registry.io/branch: release/1.4         # optional: becomes a branch of the project
    svc-registry.io/commit: 1a2b3c4             # optional: commit of a deployment seen only here
    svc-registry.io/depends-on: acme/backend/billing, external:kafka   # optional: dependencies of the project
```

Without annotations a cluster rule can map a label to a project, e.g.
`app.kubernetes.io/name=acme/{namespace}/{label:app.kubernetes.io/name}`. Workloads that match
no project are listed under the cluster's "Unmatched".

A version the cluster runs that CI never reported is recorded as a deployment with the source
`cluster` after `K8S_HISTORY_CONFIRM_SECS` (600) without a matching event. Turn observation off
for a single project in its settings ("Observe in clusters").

## Agent mode for isolated clusters

When the registry cannot reach a cluster's API server (a separate network, no inbound access),
run the agent inside the cluster instead (spec `deploy/agent`, ADR-0076). The agent is the same
binary and image: `svc-registry agent` lists workloads with its own ServiceAccount and pushes a
compressed report to `POST /api/v1/ingest/clusters/<id>/reports` every interval; matching,
state and deployments are computed by the registry exactly as for a polled cluster.

1. Add the cluster with the mode "Agent in the cluster" (no address, CA or secret). The window
   shown once has the agent token (`svca_…`), the cluster id and a ready fragment of the manifest.
2. Put the token into `agent.yaml` (the `svc-registry-agent` Secret), replace `REGISTRY_URL` and
   `CLUSTER_ID`, and apply it: a ServiceAccount with `get`/`list` on the same resources as above
   plus `create` on `selfsubjectaccessreviews` (the permission check), and one replica of the agent.
3. The cluster turns `ok` after the first report; "Test connection" shows the Kubernetes version and
   missing permissions from the last report. The agent reports every `max(interval, report gap)`:
   the gap grows with the size of the last report — `max(15, interval/2, ⌈workloads/K8S_AGENT_WORKLOADS_PER_GAP_SEC⌉)` seconds
   (2000 by default, set on the registry's `serve`), 50 s for 100 000 workloads; the clusters page shows it — and comes in the registry's answers as `report_gap_secs`. Three such periods without a
   report make the cluster `error` with `k8s.agent_silent`.
4. Rotate: "New agent token" issues a token and keeps the old one valid for 24 hours — update the
   Secret and restart the agent within that time. "Revoke tokens" stops the agent at once (it exits
   with `the registry refused the agent token`).

`svc-registry agent --url <registry> --cluster <id> --token-env SVCR_AGENT_TOKEN --once` sends one
report and exits with `0` when the registry accepted it.

Health checks (ADR-0091): with `--health-addr :8081` (used by `agent.yaml`, off by default, not with
`--once`) the agent answers `GET /healthz` — `200 {"status":"ok"}` while its loop keeps stepping (at most
`max(3 × period, 10 min)` between steps, the period being `max(interval, report gap)`; a registry that is down does not fail it, so the pod is not
restarted in a loop) — and `GET /readyz` — `200` while the registry accepted a report (or answered that
the cluster is disabled) within three periods, `503 {"status":"unavailable"}` otherwise.

Report size: the agent streams the report gzip-compressed. The registry accepts up to 16 MiB as sent on
this route (other routes keep the 2 MiB server limit) and up to `K8S_AGENT_REPORT_MAX_BYTES` (64 MiB by
default) after decompression; a larger report is refused with `413 ingest.payload_too_large`. The agent reads only `LOG_LEVEL`, the
outgoing proxy and CA variables, `K8S_REQUEST_TIMEOUT_SECS`, `K8S_LIST_LIMIT` and the token
variable — no database.
