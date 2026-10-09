# svc-registry

A registry of services: organizations and folders down to repositories, with their branches,
deployments, Kubernetes state, links to observability tools and collected documentation — in one
web UI (English, Spanish, Russian, Chinese) and one API.

One Go binary serves the JSON API under `/api` and the web UI (a static export of the Next.js app
in `web/`) on a single origin, with no Node.js at runtime. Data lives in Postgres.

## What it does

- **Catalog** — nested organizations, folders and projects; roles `viewer`, `editor`, `admin`
  inherited down the tree; users, groups, a global superadmin.
- **Deployments from CI** — pipelines report `service.deployed` events with a project key; the
  registry keeps the current version per environment and the history. → [docs/ci-events.md](docs/ci-events.md)
- **Repositories from a forge** — GitHub, GitLab, Forgejo or Gitea owners imported as projects
  and kept in sync, with their branches. → [docs/forge-sync.md](docs/forge-sync.md)
- **Kubernetes** — what really runs in clusters next to what CI reported, with drift.
  → [docs/kubernetes.md](docs/kubernetes.md)
- **Observability links** — links to logs, dashboards, alerting and runbooks generated from
  templates and checked for availability. → [docs/observability-links.md](docs/observability-links.md)
- **Project documentation** — docs collected from repositories or local sources, searchable by
  words or by meaning. → [docs/project-documentation.md](docs/project-documentation.md),
  [docs/search-engines.md](docs/search-engines.md)
- **MCP server** — AI agents read projects, deployments, links and docs with the user's rights.
  → [docs/mcp.md](docs/mcp.md)
- **Access** — email and password, OIDC providers and GitLab, personal access tokens for scripts.
  → [docs/sign-in.md](docs/sign-in.md), [docs/access-tokens.md](docs/access-tokens.md)

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
```

More in [docs/development.md](docs/development.md).

## Documentation

- [docs/](docs/README.md) — guides for every feature and for development.
- [`.env.example`](.env.example) — every environment variable with its default.
- `ARCHITECTURE.md` — code layout and rules (Russian).
- `openspec/specs/` — behavior specs; `openspec/decisions/` — architecture decision records.
- `CLAUDE.md` — a concise guide for AI coding agents.
