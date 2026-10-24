# Kubernetes manifests

Manifests for connecting a cluster to svc-registry. The guide — adding clusters, annotations, the agent, health
checks — is [docs/kubernetes.md](../../docs/kubernetes.md).

| File | What it is for |
|---|---|
| [`rbac.yaml`](rbac.yaml) | a polled cluster: a ServiceAccount with read-only access to workloads and pods, and its token for the registry |
| [`agent.yaml`](agent.yaml) | an isolated cluster: the agent's ServiceAccount, its rights, the Secret with the agent token and one replica of `svc-registry agent` with health checks |

Before applying:
- `rbac.yaml` — adjust the namespaces the registry may read.
- `agent.yaml` — put the agent token into the `svc-registry-agent` Secret and replace `REGISTRY_URL` and `CLUSTER_ID`.
  The cluster's window in the registry shows all three once.
