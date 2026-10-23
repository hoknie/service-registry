# Dependencies between services

A project's dependencies are edges "project → target". A target is another project of the registry
(slug path such as `acme/backend/billing`, compared case-insensitively, or its UUID) or an external
system named `external:<name>` (`external:postgres-main`; 1–63 characters `a-z`, `0-9`, `.`, `_`, `-`).
A path that does not match a project is kept as an unresolved reference and resolved again on the next
report of the same source.

The project page has a **Dependencies** tab (list and graph, depth 1–3); organizations and folders show
the graph of their subtree. People see only projects they can read: other projects appear as
"Unavailable project" or are only counted. MCP clients use the `get_dependencies` tool.

## Sources

Every edge remembers its source and where it came from. An automatic source replaces the whole set of
its own place on every report and never touches other places or manual edges.

| Source | Where it comes from | Place |
|---|---|---|
| CI event | `payload.dependencies` of `service.deployed` (at most 50; only when the deployment became current; an absent field changes nothing, `[]` clears) | `<service>/<environment>` |
| Cluster | annotation `svc-registry.io/depends-on: acme/backend/billing, external:kafka` on the workload or its pod template | `<cluster id>/<namespace>/<kind>/<name>` |
| Repository | `dependencies` in `.svc-registry.yaml` (or `.svc-registry.yml`) at the root of the documentation's default branch | branch name |
| Manual | project settings → **Dependencies** (editors) or `POST /api/v1/catalog/nodes/{id}/dependencies` | — |

## `.svc-registry.yaml`

```yaml
dependencies:
  - acme/backend/billing
  - external:s3
```

The file is read on every collection of the default branch whatever the collection patterns are; it gets
into the documentation snapshot only if the patterns include it. Other keys are ignored. A file over
64 KiB, broken YAML or `dependencies` that is not a list keep the previous edges and add the warning
`collect.manifest_invalid` to the scan history; invalid items are skipped with the same warning. Removing
the file removes the repository edges.

## API

- `GET /api/v1/catalog/nodes/{id}/dependencies` — `depends_on`, `dependents`, `hidden_dependents` (projects).
- `POST …/dependencies` with `{"target": "…", "note": "…"}` — a manual edge (`catalog.write`); `DELETE …/dependencies/{dependency_id}` removes it.
- `GET …/dependencies/graph?depth=1..3` — nodes and edges around a project or of a container's subtree (at most 500 nodes).
