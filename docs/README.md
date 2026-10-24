# Documentation

## Installing and running

| Page | What it covers |
|---|---|
| [install.md](install.md) | packages, archives, macOS, the container image; main settings; running behind a reverse proxy; upgrading |

## Using the catalog

| Page | What it covers |
|---|---|
| [catalog.md](catalog.md) | organizations, folders and projects; roles; branches; what background jobs are doing |
| [owners.md](owners.md) | who owns a node, inheritance, "no owners", group contacts and on-call |
| [dependencies.md](dependencies.md) | which services depend on which: from CI, clusters, the repository or by hand |
| [observability-links.md](observability-links.md) | links to logs, dashboards and runbooks from templates, and their checks |
| [project-documentation.md](project-documentation.md) | collecting documentation and API descriptions, refresh modes, limits, scan history |
| [notifications.md](notifications.md) | webhook and Slack notifications, what a receiver gets, signatures, delivery |

## Integrations

| Page | What it covers |
|---|---|
| [ci-events.md](ci-events.md) | reporting deployments from CI: what to send, links and dependencies of a rollout |
| [forge-sync.md](forge-sync.md) | importing repositories from GitHub, GitLab, Forgejo or Gitea; webhooks |
| [kubernetes.md](kubernetes.md) | polling clusters, annotations, drift, the agent for isolated clusters |
| [sign-in.md](sign-in.md) | signing in through OIDC providers and GitLab; groups from the provider |
| [mcp.md](mcp.md) | the MCP server for AI agents, with a token or over OAuth |

## Administration

| Page | What it covers |
|---|---|
| [access.md](access.md) | superadmins, users, groups, administration rights for groups, service accounts |
| [access-tokens.md](access-tokens.md) | personal access tokens: scopes, global rights, tokens for selected nodes |
| [secrets.md](secrets.md) | named secrets for forges, sources, clusters and notifications; encryption keys |
| [search-engines.md](search-engines.md) | choosing and setting up documentation search: Postgres, pgvector, Qdrant, Meilisearch, embeddings |

## Development

| Page | What it covers |
|---|---|
| [development.md](development.md) | running locally, the local environment, quality gates, packages, releases |

Every environment variable with its default: [`.env.example`](../.env.example). The HTTP API reference is published
separately.
