# Project documentation

The registry collects documentation files from your projects' repositories — README, guides, OpenSpec folders, API
descriptions — and shows them in one place:
- the **Documentation** tab of a project;
- the **Documentation** section in the sidebar, which has a tree of the whole catalog;
- search;
- AI agents through MCP.

Nothing has to run in your CI for this. Files are read through the forge's API or from a local folder.

## Sources

A project synchronized from a forge ([forge-sync.md](forge-sync.md)) is collected from its repository
automatically. Any project may also name its own **source**, which then wins over the forge. Set it in the new
project form ("Repository and documentation") or later under **Settings → Documentation settings → Source**:

- **Repository by address** — a repository on GitHub, GitLab, Gitea or Forgejo, read through the forge's API.
  - Pick a [secret](secrets.md) with a token, or none for a public repository; anonymous access hits low rate
    limits.
  - The address also becomes the project's repository link. SSH keys are not supported.
- **Local directory** — files as they are on the server's disk. There is one branch, `local`, and a new snapshot
  whenever the content changes.
- **Local git repository** — a working copy or a bare repository on the server's disk, read without the network.
  - **Include uncommitted files of the current branch** (on by default) reads the checked-out branch from the working
    copy: new and edited files are collected, deleted ones are not. Files ignored by `.gitignore` are skipped unless
    they are committed. Such a snapshot is marked *working copy*. Turn it off to collect commits only.
  - **Collect files hidden by .gitignore** (off by default, needs the first switch) also collects ignored files that
    match the include patterns. This is for documentation kept out of git, such as an `openspec/` folder in a
    repository whose `.gitignore` has `*.md`.
  - An ignored folder is entered only if a pattern can match inside it: with `openspec/**/*.md`, `node_modules/` is
    not read at all. Prefer narrow patterns: `**/*.md` also picks up ignored dependency folders unless you exclude
    them.

**Check** on the source form tries the source and shows how many branches it found.

Local paths must lie inside `KNOWLEDGE_LOCAL_ROOTS` (comma-separated absolute directories). The rule is checked by
the real path on save and on every collection. Without the variable, local sources are off.

## What is collected

Under **Settings → Documentation settings → Collection settings** of a project, folder or organization:

| Field | Meaning | Default |
|---|---|---|
| Include | glob patterns of files to collect, e.g. `docs/**/*.md`, `openspec/**/*.md` | only the README at the root |
| Exclude | patterns to skip | nothing |
| Extra branches | patterns of branches besides the default one, e.g. `release/*` | none |
| API descriptions | collect API description files without naming them in Include (below) | on |
| API description paths | glob patterns of API description files, case-insensitive ([below](#api-descriptions)) | `**/openapi*.{yaml,yml,json}`, `**/api-docs/**` |
| Refresh | **On a timer** or **On events** ([below](#when-documentation-is-refreshed)) | on a timer |
| Events | which events start a collection **On events** ([below](#when-documentation-is-refreshed)) | all four |

- **Inheritance.** Each field is inherited from the nearest node that sets it: a folder sets patterns for all its
  projects, and a project overrides one field with **Override**. **Reset** returns a field to inheritance. Every
  field shows where its value comes from.
- **Pattern syntax.** `*` (anything but `/`), `**` (any number of folders), `?`, `[...]` and `{a,b}`. Patterns are
  case-sensitive.
- **Saving.** Saving queues a new collection of every project below.

## When documentation is refreshed

**Refresh** picks one of two modes, inherited like the other fields:

- **On a timer** (the default). Every project is checked every `KNOWLEDGE_INTERVAL_SECS` (5 minutes). A branch whose
  newest commit changed gets a new snapshot. Events do not trigger anything extra.
- **On events.** There is no timer. A collection starts within seconds after an event selected in **Events**:
  - **Deployment from CI** — CI reports a new deployment of the project ([ci-events.md](ci-events.md)); sending the
    same event again does not count;
  - **Deployment seen in a cluster** — a cluster records a version CI never reported ([kubernetes.md](kubernetes.md));
  - **Forge webhook** — a forge synchronization started by a push webhook finds a new commit in one of the project's
    branches;
  - **Manual forge sync** — the same for **Sync now** or the command line.

  Scheduled synchronizations never count. **Events** is inherited like the other fields and kept while the mode is
  **On a timer**. With no event selected, only the actions below start a collection.

In both modes these start a collection too:
- **Collect now** on the **Documentation** tab;
- changing the documentation settings;
- saving the source.

Events that arrive during a collection queue exactly one more. A project on events that gets no events is not
refreshed: a local directory without CI is collected only by **Collect now**.

## Reading

- The **Documentation** tab has a file tree and renders Markdown, with links between collected files staying inside
  the tab. JSON and YAML open as collapsible trees with a **Source** switch.
- The **Documentation** section in the sidebar shows the documentation of the whole catalog. It has a tree of
  organizations and projects and an outline of the open page.
- Files of a local git working copy that are not in git are marked: **U** (untracked) and **I** (ignored).

## API descriptions

OpenAPI 3.x, Swagger 2.0 and `.proto` files are shown as an **API** view: operations or services and messages, with
**Tree** and **Source** views next to it.

Files matching **API description paths** are collected even if the include patterns do not name them. By default
these are:
- every `openapi*` file (`.yaml`, `.yml`, `.json`), in any folder;
- everything under `api-docs/`.

Swagger files, `.proto` files and `apidocs/` are collected only when you add them: **Override** the paths on an
organization or folder and use the example buttons (`**/swagger*.{yaml,yml,json}`, `**/*.proto`, `**/apidocs/**`), or
**Fill in default paths** to start again from the default list. An empty list collects no API descriptions. Paths match
regardless of case; your other patterns stay case-sensitive. Exclusions and limits still apply. Turn the switch
**API descriptions** off to stop collecting them for a node and everything below.

Descriptions under `third_party/`, `vendor/` or `node_modules/`, and well-known `.proto` packages (`google/`,
`buf/validate/`, `gogoproto/`, …) are collected, so imports resolve, but hidden from the list. **Show vendored**
reveals them. A type in a `.proto` file that comes from another file links to its definition.

## Limits

| Variable | Default | Meaning |
|---|---|---|
| `KNOWLEDGE_MAX_FILES` | 2000 | files in one snapshot |
| `KNOWLEDGE_MAX_SNAPSHOT_BYTES` | 16 MiB | total size of one snapshot |
| `KNOWLEDGE_MAX_FILE_BYTES` | 512 KiB | size of one file |
| `KNOWLEDGE_KEEP` | 5 | older snapshots kept per branch |
| `KNOWLEDGE_INTERVAL_SECS` | 300 | the timer of the **On a timer** mode |
| `KNOWLEDGE_RETRY_SECS` | 900 | pause before a failed collection is retried |
| `KNOWLEDGE_LOCAL_ROOTS` | — | folders local sources may use |

Files beyond the limits are skipped, and the snapshot is marked incomplete. The **Documentation** tab warns about
it differently for each audience:
- superadmins see which variable to raise;
- editors see the same variables and are advised to narrow the patterns or ask an administrator;
- other readers are told to contact an administrator.

After raising a limit, restart the server. The snapshot is rebuilt with the next commit or on **Collect now**.

More `KNOWLEDGE_*` tuning: [`.env.example`](../.env.example). Search: [search-engines.md](search-engines.md).

## Scan history

Every collection and every search indexing run is recorded:
- **Administration → Documentation → Scans** shows all projects (superadmins and groups with rights on **Scans**);
- **Settings → Scan history** of a project shows that project's runs to its editors.

Every run shows:
- its result: success, no changes, warning or error;
- per branch: the commit, the collected files and the skipped ones with the reason;
- for an error: an explanation and a hint what to do;
- warnings, with details for up to 20 files and a count per warning.

The common warnings:

| Warning | What to do |
|---|---|
| no files matched | the include patterns match nothing; for a local git source check the two switches above |
| files skipped | some files were too large, binary or over the limit; see the limits |
| truncated | the snapshot hit a limit |
| no branches, no source | the project has nothing to collect from |

Filters (project, kind, result, how it started — on schedule, by hand, by event) live in the page address, so a
filtered view can be shared. Both tables can refresh themselves every 10, 30 or 60 seconds. Repeated runs without
changes collapse into one row with a counter. The last `KNOWLEDGE_SCAN_HISTORY` (20) runs per project and kind are
kept.
