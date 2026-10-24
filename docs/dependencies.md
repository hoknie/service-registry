# Dependencies between services

The registry keeps which service depends on which, as a list and a graph on every project. A dependency points to
another project of the registry or to an **external system** — a database, a queue, a SaaS — that is not in the
catalog.

## Seeing dependencies

- A project's **Dependencies** tab shows what it depends on and what depends on it, as a **List** or a **Graph**
  (depth 1 to 3).
- An organization or folder shows the graph of everything below it.
- You see only projects you can read. Others appear as *Unavailable project* or are only counted.
- AI agents read the same through MCP ([mcp.md](mcp.md)).

## Where dependencies come from

| Source | How to report it | Replaced when |
|---|---|---|
| CI | `payload.dependencies` of a deployment event ([ci-events.md](ci-events.md#dependencies-of-a-rollout)) | the next current rollout of the same service and environment reports them |
| Kubernetes | the annotation `svc-registry.io/depends-on: acme/backend/billing, external:kafka` on a workload ([kubernetes.md](kubernetes.md)) | the next poll or agent report of that workload |
| Repository | `.svc-registry.yaml` at the root of the default branch (below) | the next documentation collection |
| Manual | **Settings → Dependencies** of the project (editors) | never; delete by hand |

Each automatic source replaces only what it reported itself: other sources and manual entries stay.

How to name a dependency:
- another project — by its path (`acme/backend/billing`, any case) or id;
- something outside the catalog — `external:<name>` with lowercase letters, digits, `.`, `_`, `-`, for example
  `external:postgres-main`.

A path that matches no project yet is kept and resolved when that project appears. A dependency on a project keeps
working when the project is moved or renamed. When the project is deleted, the dependency goes back to its old path;
a manual note on it survives.

## `.svc-registry.yaml`

```yaml
dependencies:
  - acme/backend/billing
  - external:s3
```

- The file is read on every documentation collection of the default branch, whatever the collection patterns are.
- Other keys are ignored.
- If the file is over 64 KiB, is not valid YAML or `dependencies` is not a list, the previous dependencies stay and
  the scan history shows a warning. Wrong items are skipped with the same warning.
- Deleting the file deletes the dependencies it gave.
