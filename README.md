# svc-registry

A registry of services: organizations and folders down to repositories, with their branches, deployments,
Kubernetes state, owners, dependencies, links to observability tools and collected documentation — in one web
interface, and for AI agents through MCP.

## What it does

- **Catalog** — nested organizations, folders and projects; roles `viewer`, `editor`, `admin` inherited down the
  tree. → [docs/catalog.md](docs/catalog.md)
- **Deployments from CI** — one pipeline step reports what was deployed where; the registry keeps the current version
  per environment and the history. → [docs/ci-events.md](docs/ci-events.md)
- **Repositories from a forge** — GitHub, GitLab, Forgejo or Gitea owners imported as projects and kept in sync, with
  their branches. → [docs/forge-sync.md](docs/forge-sync.md)
- **Kubernetes** — what really runs in clusters next to what CI reported, with drift; isolated clusters report
  through an agent. → [docs/kubernetes.md](docs/kubernetes.md)
- **Owners** — groups and people responsible for each node, with contacts and on-call links.
  → [docs/owners.md](docs/owners.md)
- **Dependencies** — which services depend on which, as a list and a graph.
  → [docs/dependencies.md](docs/dependencies.md)
- **Notifications** — webhook and Slack messages on deployments, drift, broken links and failed forge
  synchronizations, and when they recover. → [docs/notifications.md](docs/notifications.md)
- **Observability links** — links to logs, dashboards, alerts and runbooks from templates, checked for availability.
  → [docs/observability-links.md](docs/observability-links.md)
- **Project documentation** — docs and API descriptions (OpenAPI, Swagger, protobuf) collected from repositories or
  local folders, on a timer or on events, searchable by words or by meaning.
  → [docs/project-documentation.md](docs/project-documentation.md), [docs/search-engines.md](docs/search-engines.md)
- **MCP server** — AI agents read projects, deployments, links, dependencies and docs with their user's rights.
  → [docs/mcp.md](docs/mcp.md)
- **Access** — email and password or a company identity provider; groups with administration rights; personal
  tokens for scripts. → [docs/access.md](docs/access.md), [docs/sign-in.md](docs/sign-in.md),
  [docs/access-tokens.md](docs/access-tokens.md)

## Installing

Releases ship deb, rpm, macOS `.pkg`, archives and the image `ghcr.io/hoknie/service-registry`
with an external Postgres → [docs/install.md](docs/install.md).

## Quick start

Requirements: Go, Node 24, pnpm 12, Docker and [`just`](https://github.com/casey/just).

```sh
cp .env.example .env
just dev
```

Open <http://localhost:8080> and sign in as **`admin@example.com` / `change-me-please`** (created on
an empty database; change the password afterwards).

The whole stack in containers instead: `just docker-up`.

## Everyday commands

```sh
just             # list all recipes
just dev         # dev Postgres + migrations + web export + server
just web-watch   # rebuild the UI on change (second terminal; reload the page)
just docker-vdb  # Qdrant and Meilisearch for search development
just lint        # Go checks
just test        # Go tests with throwaway Postgres, Qdrant and Meilisearch
just web-check   # web checks and static export build
just security    # govulncheck, gosec, pnpm audit, actionlint/shellcheck/hadolint
```

More in [docs/development.md](docs/development.md).

## Documentation

- [docs/](docs/README.md) — guides for every feature and for development.
- [`.env.example`](.env.example) — every environment variable with its default.
