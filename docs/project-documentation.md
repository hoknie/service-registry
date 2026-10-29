# Project documentation

The registry collects documentation files from your projects' repositories — README, guides, OpenSpec folders, API
descriptions — and shows them in one place:
- the **Documentation** tab of a project;
- the **Documentation** section in the sidebar, which has a tree of the whole catalog;
- search;
- AI agents through MCP.

Nothing has to run in your CI for this. Files are read through the hosting's API or from a local folder.

## Sources

A project imported by repository sync ([repo-sync.md](repo-sync.md)) is collected from its repository
automatically. Any project may also name its own **source**, which then wins over repository sync. Set it in the new
project form ("Repository and documentation") or later under **Settings → Documentation settings → Source**:

- **Repository** — a repository on GitHub, GitLab, Gitea or Forgejo, read through the hosting's API.
  - Pick a [secret](secrets.md) with a token, or none for a public repository; anonymous access hits low rate
    limits.
  - The address also becomes the project's repository link. SSH keys are not supported.
- **Local folder** — files as they are on the server's disk. There is one branch, `local`, and a new snapshot
  whenever the content changes.
- **Local repository** — a working copy or a bare repository on the server's disk, read without the network.
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
| Branches and tags | patterns of branches and tags to collect besides the default branch ([below](#branches-tags-and-the-current-release)) | only the default branch |
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

## Branches, tags and the current release

Each row of **Branches and tags** is a pattern with a kind:
- **Branches** — `feat/*` collects every branch starting with `feat/`, `master` collects the branch `master`. The
  default branch is always collected.
- **Tags** — `v*` or `v*-release` collects tags matching the mask. Tags come from remote and local git sources and
  from projects imported by repository sync; a local directory has none.

Only the `KNOWLEDGE_MAX_TAGS` newest matching tags are collected (10 by default, `0` turns tags off). A tag is newer
when the first number group in its name is larger: `v0.10.0` is newer than `v0.9.3`. Tags without numbers count as
older than any tag with a version. A tag is collected once and again only when it moves to another commit; tags that
drop out of the newest ones or disappear lose their snapshots.

Mark a tag pattern **Release** to get a current release: the newest tag matching such a pattern. When
`v0.2.0-release` appears after `v0.1.0-release`, it becomes the current release and `v0.1.0-release` stops being one.
The current release is collected even outside the newest tags and is marked **Current release** in the version
picker.

The **Documentation** tab and the documentation section show branches and tags in one version picker. A tag is kept
in the address as `?tag=v1.2.0`. The API takes `tag=<name>` or `release=current` instead of `branch` on files, trees,
API descriptions and search (`400 validation.invalid_ref` when more than one is given).

Settings saved before tags existed keep their extra branches as branch patterns. Clients that still send the old
`branches` field get `400 validation.knowledge_branches_replaced` and should send `refs`:
`[{"pattern": "release/*", "kind": "branch"}, {"pattern": "v*-release", "kind": "tag", "release": true}]`.

## When documentation is refreshed

**Refresh** picks one of two modes, inherited like the other fields:

- **On a timer** (the default). Every project is checked once per polling period of the project
  ([repository-polling.md](repository-polling.md); `KNOWLEDGE_INTERVAL_SECS`, 5 minutes, by default). A branch whose
  newest commit changed gets a new snapshot. Events do not trigger anything extra.
- **On events.** There is no timer. A collection starts within seconds after an event selected in **Events**:
  - **Deployment from CI** — CI reports a new deployment of the project ([ci-events.md](ci-events.md)); sending the
    same event again does not count;
  - **Deployment seen in a cluster** — a cluster records a version CI never reported ([kubernetes.md](kubernetes.md));
  - **Hosting webhook** — a repository sync started by a push webhook finds a new commit in one of the project's
    branches;
  - **Manual repository sync** — the same for **Sync now** or the command line.

  Scheduled synchronizations never count. **Events** is inherited like the other fields and kept while the mode is
  **On a timer**. With no event selected, only the actions below start a collection.

In both modes these start a collection too:
- **Collect now** on the **Documentation** tab;
- changing the documentation settings;
- saving the source.

Polling turned off for the project, or **Only published documentation** for a project that is not published, stops both
modes; only **Collect now** collects then ([repository-polling.md](repository-polling.md)).

Events that arrive during a collection queue exactly one more. A project on events that gets no events is not
refreshed: a local directory without CI is collected only by **Collect now**.

## Reading

- The **Documentation** tab has a file tree and renders Markdown, with links between collected files staying inside
  the tab. JSON and YAML open as collapsible trees with a **Source** switch.
- The **Documentation** section in the sidebar shows the documentation of the whole catalog. It has a tree of
  organizations and projects and an outline of the open page.
- Files of a local git working copy that are not in git are marked: **U** (untracked) and **I** (ignored).
- Markdown files are named by their first `# ` heading, with the file name in the tooltip. The breadcrumbs go on
  through the folders of the project, and every folder opens a page with its subfolders and files.

## Publication

Documentation can be shown without signing in. Under **Settings → Publication** of an organization, folder or project:

| Field | Meaning | Default |
|---|---|---|
| Publish | show this node and what is below it | no |
| Published versions | patterns of branches and tags, like the collection patterns | none |
| Default branch | publish the default branch of the documentation | yes |
| Current release | publish the current release tag | yes |
| Public name | folders and projects: the address `/public/docs/<name>` | — |
| Host | serve the public documentation of this node on its own host | — |

- **Inheritance.** The first four fields are inherited from the nearest node that sets them, like the collection
  settings. A project of a published organization is hidden with **Publish: No**; a hidden folder hides everything
  below it. The name and the host work only on their own node.
- **Addresses.** A published organization is at `/public/docs/<organization slug>`, a folder or project published on
  its own at `/public/docs/<public name>`. A node with a host is served at the root of that host; set up DNS, TLS
  and the proxy as described in [install.md](install.md#behind-a-reverse-proxy). **Public addresses** in the
  settings list where the node can be read.
- **What readers see.** The tree, folder pages and files of the published projects, a project picker, the published
  versions with the current release marked, and a text search over the published files. There is no organization
  picker, no link to the catalog and no API view of OpenAPI and protobuf files (they open as trees or source).
- **Who changes it.** Superadmins and members of a group with the **Publication** right
  ([access.md](access.md#administration-rights-for-groups)). Everyone who can read a node sees its settings.
- **Limits.** Anonymous requests are limited to `DOCS_PUBLIC_RATE_PER_MIN` (300) a minute per client address; above
  it the server answers 429.

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
| `KNOWLEDGE_KEEP` | 5 | older snapshots kept per branch or tag |
| `KNOWLEDGE_MAX_TAGS` | 10 | newest tags collected per project, `0` — no tags |
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
