package types

import "svc-registry/internal/platform/apperr"

var (
	InvalidFolder = apperr.Invalid("validation.invalid_mcp_folder",
		"folder fields: slug is 1 to 63 of a-z, 0-9 and - starting with a letter or digit, name is 1 to 100 characters, description at most 2000")
	InvalidScope        = apperr.Invalid("validation.invalid_mcp_scope", "organization_id must be an organization at the top level of the catalog")
	FolderTooDeep       = apperr.Invalid("validation.mcp_folder_too_deep", "MCP folders nest at most 8 levels")
	InvalidSpace        = apperr.Invalid("validation.invalid_mcp_space", "the description of an MCP space is at most 2000 characters")
	SlugFixed           = apperr.Invalid("validation.mcp_server_slug_fixed", "the slug of a server names its tools and cannot change")
	InvalidBuiltin      = apperr.Invalid("validation.invalid_builtin_tools", "tools is a list of at most 100 distinct names of a-z, 0-9 and _ starting with a letter")
	InvalidTools        = apperr.Invalid("validation.invalid_mcp_tools", "tools is a list of at most 500 distinct tool names")
	InvalidAccess       = apperr.Invalid("validation.invalid_mcp_access", "access: at most 100 groups and 100 users, existing, groups global or of the server's organization")
	InvalidServers      = apperr.Invalid("validation.invalid_mcp_servers", "servers: distinct ids of MCP servers you can see, at most the allowed number")
	InvalidSet          = apperr.Invalid("validation.invalid_mcp_servers", `set: {"mode": "all"|"selected", "servers": [ids of global servers or servers of the organization]}, at most 200`)
	InvalidFilter       = apperr.Invalid("validation.invalid_filter", "filters: state ok, failing or unknown, status draft, published or disabled, tag of a-z, 0-9, ., +, # and -")
	InvalidPagination   = apperr.Invalid("validation.invalid_pagination", "limit is 1 to 200 and cursor comes from a previous page")
	ConflictSlugTaken   = apperr.Conflicting("conflict.mcp_server_slug_taken", "the slug is used by another MCP server")
	ConflictFolderSlug  = apperr.Conflicting("conflict.mcp_folder_slug_taken", "the slug is used by another folder in the same place")
	ConflictNotEmpty    = apperr.Conflicting("conflict.mcp_folder_not_empty", "the folder has folders or servers")
	ConflictCycle       = apperr.Conflicting("conflict.mcp_folder_cycle", "a folder cannot move into itself or its descendant")
	CheckRateLimited    = &apperr.Error{Kind: apperr.UpstreamRateLimited, Code: "gateway.check_rate_limited", Message: "a server is checked at most once in 10 seconds", RetryAfterSecs: 10}
	ErrGatewayDisabled  = apperr.InternalDetail("the MCP gateway is disabled")
	ErrUnknownOperation = apperr.InternalDetail("unknown gateway operation")
)

func InvalidServer(fields ...apperr.Field) *apperr.Error {
	return &apperr.Error{Kind: apperr.Validation, Code: "validation.invalid_mcp_server",
		Message: "server fields: slug 1 to 32 of a-z, 0-9 and -, title 1 to 100, description at most 4000, at most 20 tags, " +
			"url http(s) without credentials, transport streamable or sse, auth none, bearer or header with a secret",
		Fields: fields}
}

func InvalidProxy(fields ...apperr.Field) *apperr.Error {
	return &apperr.Error{Kind: apperr.Validation, Code: "validation.invalid_mcp_proxy",
		Message: `proxy: {"mode": "direct"|"environment"|"proxy"}; proxy needs url http(s)://host:port without credentials, no_proxy at most 2000, credentials a secret`,
		Fields:  fields}
}
