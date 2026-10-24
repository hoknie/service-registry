# MCP server for AI agents

AI agents and coding assistants can read the registry through MCP. An agent sees exactly what its user can read,
and can change nothing:
- projects and their owners;
- branches;
- current deployments with cluster drift;
- observability links and dependencies;
- API descriptions;
- the collected documentation, including OpenSpec specs, changes and architecture decision records found in
  repositories.

The server address is

```
<PUBLIC_URL>/api/mcp
```

## Connecting with a token

1. Open **Account → Tokens → Connect an MCP client** and issue a token with the scope `mcp`
   ([access-tokens.md](access-tokens.md)). You may limit it to some catalog nodes.
2. Add the server to your client, for example:

```sh
claude mcp add --transport http registry https://registry.example.com/api/mcp \
  --header "Authorization: Bearer $SVCR_TOKEN"
```

Clients that speak only stdio run the bridge, which keeps the token in an environment variable:

```sh
SVCR_TOKEN=svcp_… svc-registry mcp:stdio --url https://registry.example.com --token-env SVCR_TOKEN
```

## Tools

| Tool | Gives |
|---|---|
| `list_projects`, `get_project` | projects; one project with its description, labels, repository, default branch, owners and collected documentation |
| `list_branches` | branches of a project with their head commits |
| `get_deployments` | current versions per environment, with what clusters run |
| `get_links` | observability links of a project per environment, with their last check |
| `get_dependencies` | what a project depends on and what depends on it |
| `list_docs`, `get_doc`, `search_docs` | documentation files and search over them |
| `list_specs`, `get_spec`, `list_changes` | OpenSpec specs and changes collected from a repository |
| `list_adrs`, `get_adr` | architecture decision records collected from a repository |
| `list_apis`, `get_api` | OpenAPI, Swagger and protobuf descriptions |

Documentation files are also available as resources `svcr://projects/{project}/docs/{path}`. Long results are cut
at `KNOWLEDGE_MCP_MAX_RESULT_BYTES` (256 KiB).

## Connecting without a token (OAuth)

With `MCP_OAUTH=on` clients that support the MCP authorization flow connect without a pasted token:

```sh
claude mcp add --transport http registry https://registry.example.com/api/mcp
```

1. The client opens a browser.
2. You sign in as usual.
3. On the consent page you allow access to the whole catalog or to selected nodes only.

The client then gets short-lived tokens that work only for MCP. Connected apps are listed under **Account → Apps**
and can be revoked there. A superadmin sees and revokes a person's apps on the person's page. Disabling a person or
resetting their password revokes all their apps.

How a client identifies itself:
- **By a client metadata document** — the client's id is an `https://` address of a small JSON document describing
  it. The registry accepts documents only from hosts listed in `MCP_OAUTH_CLIENT_METADATA_HOSTS`.
- **By registering itself.** The consent page then warns that the registry has not verified the app.

When a client is refused because its host is not listed, or its document could not be fetched, a superadmin who
opens the request sees which host to add or a short reason. Other people are told to contact an administrator.

| Variable | Default | Meaning |
|---|---|---|
| `MCP_OAUTH` | `off` | `on` turns the OAuth flow on; needs `PUBLIC_URL` |
| `MCP_OAUTH_ACCESS_TTL_SECS` | 3600 | lifetime of an app's access token (300–86400) |
| `MCP_OAUTH_REFRESH_DAYS` | 30 | how long an app stays connected without use (1–365) |
| `MCP_OAUTH_CLIENT_METADATA` | `on` | accept client metadata documents |
| `MCP_OAUTH_CLIENT_METADATA_HOSTS` | — (none) | hosts allowed to publish documents: `host` or `*.domain`, comma-separated; exactly `*` allows any public host and is logged as a warning |
| `MCP_OAUTH_CLIENT_METADATA_USE_PROXY` | `false` | fetch documents through `HTTPS_PROXY`; the address behind the proxy is then not checked |

Good to know:
- **Fetching documents.** The registry never fetches them from private or loopback addresses and follows no
  redirects. It waits at most 5 seconds, reads at most 5 KiB and keeps a document for up to a day.
- **Behind a reverse proxy**, a few paths must reach the server; see
  [install.md](install.md#behind-a-reverse-proxy).
- **Turning `MCP_OAUTH` off** stops app tokens at once; personal tokens keep working.
