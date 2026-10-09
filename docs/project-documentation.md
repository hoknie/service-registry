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
- **Local git repository** — a working copy or a bare repository read without the network:
  branches and commits only, uncommitted changes are not collected.

Local paths must lie inside `KNOWLEDGE_LOCAL_ROOTS` (absolute directories; empty — local sources
are off), checked by the real path on save and on every collection.

Searching the collected documentation by words or by meaning: [search-engines.md](search-engines.md).
