# Skills: find, choose a version, install

A skill is a folder with `SKILL.md` (YAML frontmatter with `name` and `description`, then instructions) and
any other files it refers to. The registry keeps skills with Semantic Versioning versions; you only see
published skills and the versions their policy allows.

## 1. Find

Call `list_skills` with `query` (words of the name, title, description or tags), `tag` (all must match) or
`folder`. Each result has `name`, `path`, `version` (the latest available) and `versions`.

## 2. Get

Call `get_skill` with `skill` (id, path or name) and, if the user asked for one, `version`: an exact
version (`1.4.0`) or a range (`^1.2`, `~1.4`). Without `version` you get the latest available one.
If the name is ambiguous, the error lists the paths to choose from.

The result has the full `SKILL.md`, the list of files and the text of the other files while they fit.
Fetch the rest, and every binary file, with `get_skill_file` (`skill`, `path`, `version`).

## 3. Install

Create a folder named after the skill's `name` in the skills directory of your agent and write every file
at its path inside it, for example:

- Claude Code: `~/.claude/skills/<name>/` (user) or `.claude/skills/<name>/` (project);
- other agents that read Agent Skills: their skills directory, one folder per skill.

Keep the files byte for byte; do not rewrite `SKILL.md`.

## Archive over HTTP

A token with the `read` scope can download a skill as a zip with the folder `<name>/` inside:

```
curl -fsSL -H "Authorization: Bearer $SVCR_TOKEN" {{base}}/api/v1/skills/<id>/archive?version=<version> -o <name>.zip
unzip <name>.zip -d ~/.claude/skills
```

The response header `X-Skill-Version` names the version you got.
