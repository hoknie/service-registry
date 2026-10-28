# Documentation: search and read

The registry collects documentation from project repositories: README files, OpenSpec specifications,
changes and ADRs, API descriptions and other files the collection settings select.

1. Find a project with `list_projects`, or search everything you can read with `search_docs`
   (`mode`: `text`, `semantic` or `hybrid`; `project`, `branch` and `path` narrow it).
2. List the files of a snapshot with `list_docs` and read one with `get_doc`; the resource
   `svcr://projects/{project}/docs/{+path}` returns the same file.
3. For OpenSpec use `list_specs` / `get_spec` (by capability, e.g. `ingest/events`), `list_changes`,
   `list_adrs` / `get_adr`.
4. For APIs use `list_apis`, then `get_api` with `path` and, for one operation, `item` (`GET /pets/{id}`
   or an operationId).

Without `branch` the default branch is used; `tag` or `release: current` select a collected tag.
