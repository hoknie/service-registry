# Project documentation

For every project synchronized from a forge, the registry collects documentation files straight
from the repository through the forge's API — no clone, no CI step. By default only the README at
the root of the default branch is collected. The **Documentation settings** tab of a project, folder or
organization sets glob patterns (`docs/**/*.md`), exclusions
and extra branches (`release/*`); each field is inherited from the nearest node that sets it, so a
folder can set the patterns for all its projects and a project can override one field. The
**Documentation** tab is for reading: a collapsible file tree, Markdown rendered for every project
(links between collected files stay in the tab), JSON and YAML as collapsible trees with a
**Source** switch. Search (the search page and the MCP `search_docs` tool) matches other forms of
Russian and English words ("ветки" finds "ветка", "deploy" finds "deployments") and words
by their beginning ("Репозит" finds "репозиторий"); without a branch it looks at each project's default
documentation branch (`local` for a local directory). The migration that added the word forms rebuilds
the index of every stored file. A new snapshot is taken within `KNOWLEDGE_INTERVAL_SECS` after a branch gets a new
head (forge sync or push webhook), or at once with **Collect now**. Content is stored once per
SHA-256; `KNOWLEDGE_KEEP` older snapshots of a branch are kept, and they go when the branch record
goes. Limits and timing: `KNOWLEDGE_*` in
`.env.example`.

A project can also name its own **documentation source** — right in the Add/Edit project dialogs
("Repository and documentation") or later on the Documentation settings tab → Source. It wins over the forge
synchronization and works for projects created by hand; a repository by address also becomes the
project's repository link:

- **Repository by address** — a repository on GitHub, GitLab, Gitea or Forgejo, read through the
  forge's API with a token of the source (encrypted with `SECRETS_KEYS` or an `env:`/`file:`
  reference) or anonymously for public repositories (anonymous requests hit low API rate limits).
  SSH keys are not supported.
- **Local directory** — files as they are on disk; one branch `local`, a new snapshot when the
  content changes.
- **Local git repository** — a working copy or a bare repository read without the network.
  Branches are read from their commits. The checked-out branch is read from the working copy by
  default ("Include uncommitted files of the current branch"):
  - new and edited files are collected, deleted ones are not;
  - files excluded by `.gitignore` or `.git/info/exclude` are skipped;
  - such a snapshot shows its commit with the mark "working copy".

  Turn the flag off to collect commits only.

Local paths must lie inside `KNOWLEDGE_LOCAL_ROOTS` (absolute directories; empty — local sources
are off), checked by the real path on save and on every collection.

Searching the collected documentation by words or by meaning: [search-engines.md](search-engines.md).

## Scan history

Every collection run and every search indexing run of a project is recorded and shown to superadmins
under **Administration → Scans** (`/<locale>/admin/scans`, API `GET /api/v1/knowledge/scans`). A run
has:

- a result: success, no changes, warning or error;
- per branch: the commit, collected and skipped files with the reason;
- an error with an explanation and a hint what to do;
- warnings:
  - `collect.no_files_matched` — nothing fits the include patterns; a local git source with
    "Include uncommitted files" off reads only committed files;
  - `collect.files_skipped`, `collect.truncated`, `collect.no_branches`, `collect.no_source`.

Filters (project, kind, result, how it started) live in the page address. Repeated runs without
changes collapse into one row with a counter. The last `KNOWLEDGE_SCAN_HISTORY` (20) runs are kept
per project for collection and for indexing. Spec: `openspec/specs/knowledge/scan-history/`.

## Activity indicators

A project shows what its background jobs are doing right now: documentation collection, indexing
for search (with the number of files waiting for embeddings), forge sync and cluster polling. Each
job is:

- **running** — a worker holds it now;
- **queued** — it is due (or "Collect now" was pressed) and waits for a worker. With
  `BACKGROUND_JOBS_ENABLED=false` nothing picks it up, so it stays queued;
- **failed** — its last run ended with an error; the hint explains the code;
- idle otherwise — nothing is shown.

Organizations and folders show how many readable projects below them are running, queued or failed.
The page refreshes this every 5 seconds while something runs or waits, otherwise every minute, and
not while the browser tab is hidden. API: `GET /api/v1/catalog/nodes/{id}/activity`; the catalog table
view uses `GET /api/v1/catalog/table`. Spec: `openspec/specs/catalog/activity/`.
