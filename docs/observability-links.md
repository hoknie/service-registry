# Observability links

Links to logs, Grafana, Sentry, Alertmanager, traces, runbooks — any system — are **generated**
for every project from templates. A superadmin keeps the **link kinds** (Administration → Link
kinds: key, a name in each of the four languages, an icon, the order; `logs`, `grafana`, `sentry`,
`alertmanager`, `traces`, `runbook`, `docs` come built in). Editors put **templates** on an
organization, folder or project (the **Links** tab); a template on an organization applies to every
project below it, a folder or project may override it, disable it or add another link of the same
kind under its own key:

```
https://grafana.example/d/svc?var-ns={namespace}&var-env={environment}
https://logs.example/{path|raw}?q=service%3A{service}
```

Variables: `{project.id|slug|name}`, `{path}`, `{node.N}`, `{labels.<key>}`, `{vars.<key>}` (node
variables, inherited the same way), `{repo.full_path}`, `{branch}`, and from the current deployment
of each environment `{service}`, `{environment}`, `{version}`, `{commit}`, `{cluster}`,
`{namespace}`, `{url}` — a template using them gives a link per environment. Values are
percent-encoded unless `|raw`; `|lower` lowers the case; `{{`/`}}` are literal braces. A link whose
variable has no value is shown as "missing: …", not broken.

Links are **checked** in the background (`HEAD`, then `GET` for `405/501`; statuses `ok`,
`auth_required`, `not_found`, `server_error`, `timeout`, `blocked`, …) every
`LINK_CHECK_INTERVAL_SECS` (3600) and on demand ("Check", at most once per 30 s per address).
Checks never use the proxy variables and never connect to loopback, link-local (cloud metadata),
multicast or unspecified addresses; private networks are allowed unless
`LINK_CHECK_ALLOW_PRIVATE=false` (then only `LINK_CHECK_ALLOW_HOSTS`); `LINK_CHECK_DENY_HOSTS` are
never checked. See `.env.example` for `LINK_CHECK_*`. Specs: `openspec/specs/links/`.

**Title and icon.** A template may carry a title (shown instead of the kind name) and an icon: the
kind's built-in icon, an `https://` address, or an uploaded PNG, WebP, ICO or SVG file up to 64 KiB.
Both are inherited with the template, so icons set on an organization show up on every project
below. Projects show their link icons in the header, in cards, tables and the tree view; a link
without an address (a variable has no value) is grey. Uploaded files are stored on the server's disk
in `UPLOADS_DIR` (default `data/uploads` next to the working directory; `off` turns uploads off),
named by the SHA-256 of their content, and served with a sandboxing `Content-Security-Policy`. In
Docker the image keeps them in `/data/uploads`; mount a volume there (`docker-compose.yml` does).
Specs: `openspec/specs/links/templates/`, `openspec/specs/links/icons/`.
