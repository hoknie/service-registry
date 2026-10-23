# Kubernetes clusters

A superadmin adds Kubernetes clusters (Administration → Clusters): the API server address, its CA
and a global [secret](forge-sync.md#secrets) holding a read-only ServiceAccount token (Administration →
Secrets; or "the registry runs in this cluster"; an inline token is refused). The registry polls
Deployments, StatefulSets, DaemonSets and CronJobs every `interval_secs` (60) and matches them to
projects by annotations (`svc-registry.io/project: acme/backend/api` or the project's UUID, plus
optional `service`, `environment`, `branch`, `commit`) or by cluster rules. The Deployments tab then
shows next to each CI deployment what really runs — versions, ready replicas, rollout state,
restarts — and flags **drift**. A version seen in a cluster that CI never reported becomes a
deployment with the source `cluster` after `K8S_HISTORY_CONFIRM_SECS` (600). A project can opt out
("Observe in clusters"). The order and names of environments come from a directory
(Administration → Environments). RBAC manifest and annotation examples: `examples/k8s/`; settings:
`K8S_*` in `.env.example`.

**Isolated clusters.** When the registry cannot reach a cluster's API server, add it in the mode "Agent in the
cluster" and run `svc-registry agent` inside it ([examples/k8s/README.md](../examples/k8s/README.md#agent-mode-for-isolated-clusters)):
the agent lists workloads with its own ServiceAccount and pushes a gzip-streamed report (up to 100 000 workloads,
16 MiB as sent, `K8S_AGENT_REPORT_MAX_BYTES` unpacked) to the registry. The registry spaces reports by their size —
the report gap `max(15, interval/2, ⌈workloads/K8S_AGENT_WORKLOADS_PER_GAP_SEC⌉)` seconds, shown on the clusters
page — and marks a cluster that stays silent for three periods. With `--health-addr :8081` the agent answers
`/healthz` and `/readyz` for Kubernetes probes.
