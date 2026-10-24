# Kubernetes clusters

The registry looks into your clusters and shows, next to every deployment CI reported, what really runs: versions,
ready replicas, rollout state, restarts. When the two disagree, it flags **drift**.

There are two ways to connect a cluster:
- **polling** — the registry reaches the cluster's API server itself;
- **agent** — for clusters the registry cannot reach, a small agent inside the cluster sends reports out.

Clusters are managed under **Administration → Infrastructure → Clusters** by superadmins and groups with rights on
**Clusters**. Environment names and their order come from **Administration → Catalog → Environments**.

## Connecting a polled cluster

1. Apply [`examples/k8s/rbac.yaml`](../examples/k8s/rbac.yaml), adjusting the namespaces. It creates a
   ServiceAccount that may only `get` and `list` Deployments, StatefulSets, DaemonSets, ReplicaSets, CronJobs, Jobs
   and Pods, and its token.
2. Put the token into a global [secret](secrets.md).
3. Add the cluster: the API server address, its CA certificate (PEM), the secret, the default environment. A
   registry running in the same cluster may pick **The registry runs in this cluster** and use its own
   ServiceAccount instead.
4. Press **Test connection**: it lists missing permissions.

The registry then polls Deployments, StatefulSets, DaemonSets and CronJobs, every 60 seconds by default.

## Telling the registry which project a workload is

Annotate the workload, on the object or on its pod template (Helm charts usually set the latter):

```yaml
metadata:
  annotations:
    svc-registry.io/project: acme/backend/api   # path of the project, or its id
    svc-registry.io/service: api                # default: the workload's name
    svc-registry.io/environment: production     # default: the cluster's environment
    svc-registry.io/branch: release/1.4         # optional: becomes a branch of the project
    svc-registry.io/commit: 1a2b3c4             # optional: commit of a version seen only here
    svc-registry.io/depends-on: acme/backend/billing, external:kafka   # optional: dependencies
```

Without annotations, a **cluster rule** can map a label to a project, for example
`app.kubernetes.io/name=acme/{namespace}/{label:app.kubernetes.io/name}`. Workloads that match no project are listed
under the cluster's **Unmatched**.

- **Versions CI never reported.** A version a cluster runs without a matching CI event is recorded as a release
  with the source *cluster* after `K8S_HISTORY_CONFIRM_SECS` (10 minutes).
- **Opting out.** A project can turn this off with **Observe in clusters** in its settings.

## Isolated clusters: the agent

When the registry cannot reach a cluster's API server (another network, no inbound access), run the agent inside the
cluster. It is the same binary and image: `svc-registry agent` lists workloads with its own ServiceAccount and sends a
compressed report to the registry. Matching, drift and releases work exactly as for a polled cluster.

1. Add the cluster with the mode **Agent in the cluster**. No address, CA or secret is needed. The window shown
   once has the agent token (`svca_…`), the cluster id and a ready piece of the manifest.
2. Put the token into [`examples/k8s/agent.yaml`](../examples/k8s/agent.yaml) (the `svc-registry-agent` Secret),
   replace `REGISTRY_URL` and `CLUSTER_ID`, and apply it. It creates a ServiceAccount with the same read rights plus
   the permission check, and one replica of the agent.
3. The cluster turns **ok** after the first report. **Test connection** shows the Kubernetes version and missing
   permissions from the last report.

The agent connects to the registry address you gave it; the cluster needs outbound access only.

**How often it reports.** The registry spaces reports by their size, so large clusters do not flood it. The gap is
the largest of:
- 15 seconds;
- half the cluster's interval;
- one second per `K8S_AGENT_WORKLOADS_PER_GAP_SEC` (2000) workloads of the last report.

That is 50 seconds for 100 000 workloads. The clusters page shows the gap. A cluster that sends nothing for three
periods is marked as an error: *the agent is silent*.

**Rotating the token.** **New agent token** issues a token and keeps the old one valid for 24 hours: update the Secret
and restart the agent within that time. **Revoke tokens** stops the agent at once.

**Health checks.** `agent.yaml` starts the agent with `--health-addr :8081` (off by default). It then answers two
probes:
- `/healthz` — fine while the agent's loop keeps going. A registry that is down does not fail it, so the pod is not
  restarted in a loop.
- `/readyz` — fine while the registry accepted a report within three periods.

**One report by hand.** `svc-registry agent --url <registry> --cluster <id> --token-env SVCR_AGENT_TOKEN --once`
sends one report and exits with `0` when it was accepted.

**Limits.** A report lists at most 100 000 workloads. It may be at most 16 MiB compressed and
`K8S_AGENT_REPORT_MAX_BYTES` unpacked. The agent needs no database. It reads only `LOG_LEVEL`, the proxy and CA
variables, `K8S_REQUEST_TIMEOUT_SECS`, `K8S_LIST_LIMIT` and its token variable.

## Settings

| Variable | Default | Meaning |
|---|---|---|
| `K8S_HISTORY_CONFIRM_SECS` | 600 | how long a version seen only in a cluster waits for a CI event before it becomes a release |
| `K8S_POLL_CONCURRENCY` | 2 | clusters polled at the same time |
| `K8S_WORKLOAD_RETENTION_DAYS` | 7 | how long workloads that vanished stay listed |
| `K8S_REQUEST_TIMEOUT_SECS` | 10 | timeout of one request to a cluster (also used by the agent) |
| `K8S_LIST_LIMIT` | 500 | page size when listing objects (also used by the agent) |
| `K8S_AGENT_REPORT_MAX_BYTES` | 64 MiB | largest unpacked agent report |
| `K8S_AGENT_WORKLOADS_PER_GAP_SEC` | 2000 | workloads per second of gap between agent reports |
