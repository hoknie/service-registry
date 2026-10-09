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
```

Without annotations a cluster rule can map a label to a project, e.g.
`app.kubernetes.io/name=acme/{namespace}/{label:app.kubernetes.io/name}`. Workloads that match
no project are listed under the cluster's "Unmatched".

A version the cluster runs that CI never reported is recorded as a deployment with the source
`cluster` after `K8S_HISTORY_CONFIRM_SECS` (600) without a matching event. Turn observation off
for a single project in its settings ("Observe in clusters").
