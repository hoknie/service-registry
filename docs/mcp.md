# MCP server for AI agents

The registry is an MCP server at `/api/mcp` (Streamable HTTP, JSON answers, no sessions). An agent
sees exactly what its user can read: projects, branches, current deployments with cluster drift,
observability links and the collected documentation — 13 read-only tools (`list_projects`,
`get_project`, `list_branches`, `get_deployments`, `get_links`, `list_docs`, `get_doc`,
`search_docs`, `list_specs`, `get_spec`, `list_changes`, `list_adrs`, `get_adr`) and documentation
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

Tool results are capped at `KNOWLEDGE_MCP_MAX_RESULT_BYTES`. Spec: `openspec/specs/knowledge/mcp-server/`.
