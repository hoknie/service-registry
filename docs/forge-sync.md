# Importing repositories from a forge

An organization or folder can be connected to a **GitHub, GitLab, Forgejo or Gitea owner**
(organization, user or GitLab group) on its **Forge** tab. Every repository of the owner becomes a
project under that node (GitLab subgroups as folders, optional), and a synchronization keeps the
description, topics, README, languages, default branch, archived flag, visibility, stars and
licence current. Synchronized nodes are *managed*: they are moved, renamed and created by the sync
only, and their description and repository fields change on the forge. A repository that vanishes
marks its project *orphaned*; nothing is deleted.

- **Credentials** — a token, stored AES-256-GCM-encrypted (set `SECRETS_KEYS`, e.g.
  `k1:$(openssl rand -base64 32)`; rotate with `svc-registry secrets:rotate`), or a reference
  `env:NAME` / `file:/path` so no secret lives in the database. Read-only tokens are enough unless
  the registry registers the webhook itself.
- **When** — every `interval_secs` (default `FORGE_SYNC_INTERVAL_SECS`, 15 min), on **Sync now**,
  on a webhook delivery, or `svc-registry forge:sync <connection-id>`. Several replicas never run
  one connection twice; `BACKGROUND_JOBS_ENABLED=false` makes a replica API-only.
- **Webhooks** — `POST /api/v1/forge/hooks/<connection-id>`; the registry registers it on the
  forge or shows the address and secret for manual setup. Needs `PUBLIC_URL`.
- **Network** — outgoing HTTPS trusts the image's roots plus `HTTP_CA_FILE`; `HTTPS_PROXY` /
  `NO_PROXY` are honoured.

Spec: `openspec/specs/catalog/forge-sync/`.

## Branches

Every project keeps a list of its **branches**: from the forge sync (changed repositories only,
`branch_include` patterns per connection, at most `BRANCH_SYNC_MAX_PER_REPO` = 500 per repository),
from `service.deployed` events (`branch`, `commit_sha`) and the project's `default_branch`. Pick a
branch on the project page (`&branch=` in the address, the Deployments tab follows it) or manage
them on the **Branches** tab: pin a branch to keep it forever, delete a stale record. Branches gone
from the forge are deleted after `BRANCH_RETENTION_DAYS` (30); branches seen only in CI after
`BRANCH_RETENTION_DAYS + BRANCH_STALE_DAYS` (120) without activity; a branch is *stale* after
`BRANCH_STALE_DAYS` (90). Deployment history is never deleted with a branch. Spec:
`openspec/specs/catalog/branches/`.
