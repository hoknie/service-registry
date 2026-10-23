# Documentation

| Page | What it covers |
|---|---|
| [install.md](install.md) | installing a release: deb, rpm, archives, macOS `.pkg`, the container image |
| [development.md](development.md) | running locally, database commands, dev search engines, quality gates |
| [ci-events.md](ci-events.md) | reporting deployments from CI: the event format, answers, ready-made snippets |
| [access-tokens.md](access-tokens.md) | personal access tokens for scripts, CI and MCP clients |
| [forge-sync.md](forge-sync.md) | importing repositories from GitHub, GitLab, Forgejo, Gitea; branches |
| [observability-links.md](observability-links.md) | link kinds, templates and variables, availability checks |
| [kubernetes.md](kubernetes.md) | polling clusters, matching workloads to projects, drift, the in-cluster agent |
| [owners.md](owners.md) | owners of nodes, inheritance, "no owners", group contacts, on-call links and contact editors |
| [notifications.md](notifications.md) | webhook and Slack notifications: deployments, drift, broken links, forge sync, recoveries |
| [dependencies.md](dependencies.md) | dependencies between services: CI events, cluster annotations, `.svc-registry.yaml`, manual edits |
| [project-documentation.md](project-documentation.md) | collecting docs from repositories and local sources, refresh on a timer or on events, API descriptions, limits, scan history |
| [search-engines.md](search-engines.md) | `KNOWLEDGE_SEARCH_ENGINE`: Postgres, pgvector, Qdrant, Meilisearch, embeddings |
| [mcp.md](mcp.md) | the MCP server for AI agents, MCP over OAuth and the stdio bridge |
| [sign-in.md](sign-in.md) | sign-in through OIDC providers and GitLab |

Every environment variable with its default: [`.env.example`](../.env.example).
