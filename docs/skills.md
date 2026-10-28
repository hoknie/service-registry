# AI skills

The registry keeps a catalog of AI skills: folders with a `SKILL.md` and the files it refers to (scripts, references,
templates), in the Agent Skills format. Agents install them through the MCP server; people and CI download them as
zip archives.

The catalog is on by default. `SKILLS_ENABLED=false` turns it off: the **Skills** page, its API and the skill tools of
the MCP server disappear, the stored skills stay and come back when it is turned on again.

## The catalog

**Skills** in the sidebar opens the catalog. It works like the service catalog: the breadcrumbs at the top open the
neighbours and the contents of a space or folder, and **Contents** shows folders and skills as cards, a table or a
tree — the view you pick is the same in the service catalog, skills and MCP. A skill card has the title, description,
tags, the latest version, the author, the update time and the number of downloads. Search, tags and the state filter
stay in the address, so a filtered list can be shared.

**Add** creates a skill (the upload page) or a folder. A folder and an organization's space have the tabs **Overview**
and **Settings**: a folder's settings hold **General** (name, slug, description and deleting the folder) and
**Sources**; an organization's space holds its description and version policy. A skill's **Overview** shows its description — your own
text in Markdown on top (set in **Settings → General**, kept across versions and git syncs) and the description from
`SKILL.md` below — with tags and details on the right; **View** shows its files. **Settings** has **General** (tags,
slug, your description, publishing), **Image** and **Version policy**.

**Move** in the danger zone of a folder's or skill's **General** settings moves it to another folder, to the top of an
organization or to the global space; the global space needs the global **skills** right.

**Settings → Access** of a space or folder says who sees it: everyone with a role in the organization (and its owners)
sees the published skills of its space, editors and above add and change them; the roles are edited right there and
are the same as the organization's access in the catalog. The global space is seen by every signed-in user and changed
by superadmins and groups with the global **skills** right.

Every top-level folder and skill has a **scope**:

| Scope | Who reads | Who changes |
|---|---|---|
| Global | everyone who is signed in | superadmins and members of groups with the **Global AI skills** right ([access.md](access.md#administration-rights-for-groups)) |
| An organization | everyone who sees the organization in the catalog, also through a role on a project inside it | editors and admins of the organization, its owners, superadmins |

Folders inside a folder take its scope. Moving a folder to another scope moves everything inside it.

A skill is a **draft** or **published**. A draft is seen only by those who can change it; agents and downloads never
get it.

## The skill format

```markdown
---
name: go-testing
description: Table-driven tests in Go, with helpers and race detection.
license: MIT
allowed-tools: Bash Read
metadata:
  title: Go testing
  tags: [go, testing]
  author: Platform team
  version: 1.2.0
  image: assets/cover.png
---
# Go testing

Use `t.Run` for every case and see [the reference](reference.md).
```

- `name` (a-z, 0-9 and `-`, up to 64) and `description` (up to 1024 characters) are required.
- `metadata.title`, `tags`, `author` and `image` fill the card; `metadata.version` names the version.
- The folder may hold any other files; `.git`, `.DS_Store` and `__MACOSX` are dropped.

## Uploading

**Add → Skill** takes a zip archive or a folder picked in the browser. Over HTTP:

```bash
curl -fsS -H "Authorization: Bearer $SVCR_TOKEN" \
  -F archive=@go-testing.zip -F status=published -F version=1.2.0 \
  -F organization_id=<organization id> \
  https://registry.example.com/api/v1/skills
```

A new version of an existing skill goes to `PUT /api/v1/skills/<id>/content` with the same form. The name in
`SKILL.md` must stay the same.

Limits, all changeable:

| Variable | Default | Meaning |
|---|---|---|
| `SKILLS_MAX_UPLOAD_BYTES` | 10 MiB | the request body |
| `SKILLS_MAX_TOTAL_BYTES` | 50 MiB | the unpacked skill |
| `SKILLS_MAX_FILE_BYTES` | 5 MiB | one file |
| `SKILLS_MAX_FILES` | 500 | files in a skill |

## Versions

Every version has a [Semantic Versioning](https://semver.org) number. It comes from the `version` field of the upload,
then from `metadata.version` of `SKILL.md`, otherwise it is the next patch of the highest version (the first one is
`0.1.0`). Uploading the same content again does not make a new version; the same number with other content is refused.
An older line can get fixes: `1.4.1` after `2.0.0` is fine.

**Versions** on the skill page lists them: every row has **Download**, and those who can change the skill also see
**Yank** / **Restore** and **Upload version** above the table. A yanked version stays, but nobody can install it.
`SKILLS_KEEP_VERSIONS` (default `0` — all) keeps only that many of the newest versions, never dropping the newest one
and the versions the policy allows.

### Version policy

The policy decides which versions readers, agents and downloads get:

- **Version range** — npm syntax: `^1.2`, `~1.4.0`, `>=1.0.0 <3.0.0`, `1.x`, `1.2.3 || 2.x`;
- **Allow pre-releases** — `2.0.0-rc.1` and similar, off by default;
- **Excluded versions** — exact versions to hide.

Set it for a whole organization under **Settings → Skills** of the organization's space in the skills section, and for one skill under the skill's
**Settings**. Each field of a skill overrides the same field of its organization; an empty field inherits it. Editors
see every version with marks, and the skill settings show which versions the values being typed would allow.

## Git sources

A folder can follow git repositories: **Settings → Sources** on the folder page. Every folder with `SKILL.md` under the chosen
path becomes a skill of the folder; a folder inside another skill belongs to that skill.

| Field | Meaning |
|---|---|
| Kind | a repository on GitHub, GitLab, Forgejo or Gitea (with a secret for its token), a local git repository or a local directory inside `KNOWLEDGE_LOCAL_ROOTS` |
| Branch | empty — the default branch |
| Folder in the repository | empty — the whole repository |
| Interval | seconds between runs; empty — `SKILLS_SYNC_INTERVAL_SECS` (1 h) |
| New skills | draft or published |
| Versions | from the branch (`metadata.version` or the next patch on every change) or from tags |

With versions from tags every tag that matches the pattern becomes a version: `v{version}` for one skill per
repository, `{name}/v{version}` when the repository holds several skills. Removing a tag does not remove its version.

A run reads the branch only when its commit moved. Skills whose folder disappeared are removed; a skill from git
is changed only in git (its content, name and tags), its status and image can be changed in the registry.
**Sync now** queues a run at once. The last run lists what was created, updated, removed and skipped, and why.

## Connecting to an agent

**Connect** on the skill page shows three ways, for the latest available version or a chosen one:

- **MCP** — connect the agent to `/api/mcp` ([mcp.md](mcp.md)) and ask it to install the skill by name; it uses
  `get_skill`. Each such call counts as a download.
- **Archive** — `GET /api/v1/skills/<id>/archive?version=<version>` with a token that has the `read` scope; the zip holds
  the folder `<name>/` for the skills directory of the agent, e.g. `~/.claude/skills`.
- **Download** in a row of **Versions** — the same archive from the browser.

## Storage

Skill files are kept once per content, by their SHA-256:

| `SKILLS_STORAGE` | Where |
|---|---|
| `local` (default) | the directory `SKILLS_DIR` (default `data/skills`) |
| `s3` | a bucket of any S3-compatible storage: `SKILLS_S3_ENDPOINT`, `SKILLS_S3_BUCKET`, `SKILLS_S3_REGION`, `SKILLS_S3_PREFIX`, `SKILLS_S3_ACCESS_KEY` and `SKILLS_S3_SECRET_KEY` (values or `env:`/`file:` references; without them the `AWS_*` variables or the instance role), `SKILLS_S3_PATH_STYLE=true` for MinIO-like servers |
| `off` | nothing is stored; uploads and git sources are refused |

To move from a directory to S3, set the `SKILLS_S3_*` variables and run

```bash
svc-registry skills:storage-copy
```

then restart with `SKILLS_STORAGE=s3`. The command copies only what is missing and can be run again. Files nobody uses
are deleted an hour after they have been unused for a day. If the storage does not answer, the catalog still works,
only files and downloads report that the storage is unavailable.
