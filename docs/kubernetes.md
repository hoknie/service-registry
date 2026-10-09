# Kubernetes clusters

A superadmin adds Kubernetes clusters (Administration → Clusters): the API server address, its CA
and a read-only ServiceAccount token (or "the registry runs in this cluster"). The registry polls
Deployments, StatefulSets, DaemonSets and CronJobs every `interval_secs` (60) and matches them to
projects by annotations (`svc-registry.io/project: acme/backend/api` or the project's UUID, plus
optional `service`, `environment`, `branch`, `commit`) or by cluster rules. The Deployments tab then
shows next to each CI deployment what really runs — versions, ready replicas, rollout state,
restarts — and flags **drift**. A version seen in a cluster that CI never reported becomes a
deployment with the source `cluster` after `K8S_HISTORY_CONFIRM_SECS` (600). A project can opt out
("Observe in clusters"). The order and names of environments come from a directory
(Administration → Environments). RBAC manifest and annotation examples: `examples/k8s/`; settings:
`K8S_*` in `.env.example`. Specs: `openspec/specs/deploy/`.
