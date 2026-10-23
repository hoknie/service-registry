# MCP server for AI agents

The registry is an MCP server at `/api/mcp` (Streamable HTTP, JSON answers, no sessions). An agent
sees exactly what its user can read: projects, branches, current deployments with cluster drift,
observability links, dependencies, API descriptions and the collected documentation — 16 read-only tools
(`list_projects`, `get_project` with owners, `list_branches`, `get_deployments`, `get_links`, `get_dependencies`,
`list_docs`, `get_doc`, `search_docs`, `list_specs`, `get_spec`, `list_changes`, `list_adrs`, `get_adr`, `list_apis`,
`get_api`) and documentation
files as resources `svcr://projects/{project}/docs/{path}`. Issue a personal access token with the
scope `mcp` (**Account → Tokens → Connect an MCP client**), then:

```sh
claude mcp add --transport http registry https://registry.example.com/api/mcp \
  --header "Authorization: Bearer $SVCR_TOKEN"
```

Clients that speak only stdio run the bridge, which keeps the token in an environment variable:

```sh
SVCR_TOKEN=svcp_… svc-registry mcp:stdio --url https://registry.example.com --token-env SVCR_TOKEN
```

## MCP over OAuth

With `MCP_OAUTH=on` the registry is also an OAuth 2.1 authorization server for `/api/mcp`, so clients that support
the MCP authorization flow connect without a pasted token:

```sh
claude mcp add --transport http registry https://registry.example.com/api/mcp
```

The client finds the server from the `401` answer (`WWW-Authenticate: Bearer resource_metadata=…`), identifies itself
— by the address of its client metadata document or by registering — opens a browser where you sign in as usual and allow access on the consent page — to the whole catalog or to selected
nodes only — and then receives short-lived `svco_` access tokens and rotating `svcf_` refresh tokens. The app sees no
more than you can read, and only through `/api/mcp`; the REST API refuses its tokens.

- `PUBLIC_URL` is required: it is the issuer and the resource is `<PUBLIC_URL>/api/mcp`.
- `MCP_OAUTH_ACCESS_TTL_SECS` (default 3600) and `MCP_OAUTH_REFRESH_DAYS` (default 30) set the lifetimes.
- Connected apps are listed under **Account → Apps** and can be revoked there; a superadmin sees and revokes a user's
  apps on the user's page. Disabling a user or resetting their password revokes all their apps.
- Clients identify themselves in one of two ways:
  - **Client ID Metadata Document** (`MCP_OAUTH_CLIENT_METADATA=on`, the default, and the document's host listed in
    `MCP_OAUTH_CLIENT_METADATA_HOSTS` — `host` or `*.domain`, comma-separated, or exactly `*` for any host, which
    `serve` warns about at start): the `client_id` is an `https://`
    address of a JSON document with `client_id` (the same address), `client_name` and `redirect_uris`. With an empty
    list no documents are accepted (`oauth.client_not_allowed`) and the server metadata says
    `client_id_metadata_document_supported: false`; removing a host also stops token requests of its clients. The
    registry fetches the document on authorization never from private, loopback or link-local addresses, follows no
    redirects, waits at most 5 s and reads at most 5 KiB, and keeps it for its `Cache-Control: max-age` (at most 24 h,
    1 h without the header, not at all with `no-store`); fetches that miss the cache are limited to 30 per hour per
    address. It connects directly, checking the IP at dial; with `MCP_OAUTH_CLIENT_METADATA_USE_PROXY=true` it goes
    through `HTTPS_PROXY`/`HTTP_PROXY` instead, and then the IP behind the proxy is **not** checked: only hosts written
    as an IP address, `localhost` and `*.localhost` are refused before the request, hosts in `NO_PROXY` are reached
    directly with the full check, and `serve` logs a warning at start. When a document is refused because its host is not
    listed (or documents are off), a superadmin who opens the request sees which host to add to
    `MCP_OAUTH_CLIENT_METADATA_HOSTS`; when the document could not be fetched or failed the checks, a superadmin sees a
    short reason (`timeout`, `dns`, `tls`, `blocked_address`, `redirect`, the HTTP status, `too_large`, a missing
    `client_name`, a `client_secret` in the document, …) — never the remote server's answer, which only the server log
    carries. Other users are only told to contact an administrator. The consent page shows the document's host;
    when the app returns to `localhost`, it also warns that any program on this computer could pretend to be the app.
  - **Dynamic registration** (`POST /api/oauth/register`): the consent page shows the host the app returns to and
    that the registry has not verified the app; registrations are limited to 10 per hour per address.
- Clients without any grant are deleted after 30 days; a client of a document is fetched again on its next use.
- Turning `MCP_OAUTH` off stops app tokens at once; personal tokens keep working.

Tool results are capped at `KNOWLEDGE_MCP_MAX_RESULT_BYTES`.
