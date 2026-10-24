# Observability links

Every project gets links to its logs, dashboards, alerts, traces and runbooks — in any system. You describe a link
once as a **template** on an organization or folder, and every project below gets its own link. The registry checks
the links in the background and shows which ones are broken.

## Link kinds

A kind is what a link is: `logs`, `grafana`, `sentry`, `alertmanager`, `traces`, `runbook`, `docs` come built in.
Superadmins and groups with rights on **Link kinds** manage kinds under **Administration → Catalog → Link kinds**:
the key, a name in each of the four languages, an icon and the order.

## Templates

1. Open an organization, folder or project, **Settings → Links** (editors).
2. Add a template: the kind, the address with variables, optionally a title and an icon.
3. Watch the preview: it shows the address for a sample project as you type.

```
https://grafana.example/d/svc?var-ns={namespace}&var-env={environment}
https://logs.example/{path|raw}?q=service%3A{service}
```

A template applies to every project below its node.
- A folder or project may **override** it with its own template under the same key, or **disable** it.
- It may also add another link of the same kind under its own key.

**Variables:**

| Variable | Value |
|---|---|
| `{project.id}`, `{project.slug}`, `{project.name}`, `{path}`, `{node.N}` | the project and its place in the tree |
| `{labels.<key>}` | a label of the project |
| `{vars.<key>}` | a variable set on the node or above, under **Settings → Links** |
| `{repo.full_path}`, `{branch}` | the repository and the selected branch |
| `{service}`, `{environment}`, `{version}`, `{commit}`, `{cluster}`, `{namespace}`, `{url}` | from the current deployment; a template using them gives one link per environment |

- **Encoding.** Values are percent-encoded. `{x|raw}` inserts them as they are, `{x|lower}` lowercases them, and
  `{{` and `}}` are literal braces.
- **Missing values.** A link whose variable has no value is shown as *missing*, greyed out, not as broken.

**Links from CI.** A deployment may bring its own links ([ci-events.md](ci-events.md#links-of-a-rollout)). While
it is current, they replace the template link with the same key for that service and environment.

**Title and icon.** A template may carry a title, shown instead of the kind name, and an icon:
- the kind's built-in icon;
- an `https://` address;
- an uploaded PNG, WebP, ICO or SVG file up to 64 KiB.

Both are inherited with the template. Projects show their link icons in the header, in cards, in tables and in the
tree.

## Checks

Links are checked in the background every `LINK_CHECK_INTERVAL_SECS` (1 hour) and on **Check** (at most once per 30
seconds per address). A link shows its state: *Available*, *Sign-in required*, *Not found*, *Server error*, *Timed out*,
*Certificate error*, *Not checked: address forbidden* and so on.

Checks never go through the proxy. They never connect to:
- loopback addresses;
- link-local addresses, including cloud metadata;
- multicast or unspecified addresses.

Private networks are allowed unless you turn them off.

| Variable | Default | Meaning |
|---|---|---|
| `LINK_CHECK_INTERVAL_SECS` | 3600 | how often every address is checked |
| `LINK_CHECK_ALLOW_PRIVATE` | `true` | `false` — check private-network addresses only if listed in `LINK_CHECK_ALLOW_HOSTS` |
| `LINK_CHECK_ALLOW_HOSTS`, `LINK_CHECK_DENY_HOSTS` | — | hosts always allowed, hosts never checked |
| `LINK_CHECK_TIMEOUT_SECS` | 5 | timeout of one check |

More `LINK_CHECK_*` tuning: [`.env.example`](../.env.example).

## Uploaded icons

Uploaded icons are stored on the server's disk and served with a strict sandbox.

| Variable | Default | Meaning |
|---|---|---|
| `UPLOADS_DIR` | `data/uploads` | where uploaded files go; `off` turns uploads off. The container image uses `/data/uploads` — mount a volume there |
| `UPLOADS_UNREFERENCED_TTL_SECS` | 86400 | a file no template uses is deleted after this long (600 seconds to 30 days) |
