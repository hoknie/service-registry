package http

import "svc-registry/internal/presentation/http/handlers"

func (r routes) gateway(api *handlers.Handlers) {
	if api.Gateway == nil {
		return
	}
	r.add("/api/v1/mcp/folders", true, get(api.ListMCPFolders), post(api.CreateMCPFolder))
	r.add("/api/v1/mcp/folders/:id", true, get(api.GetMCPFolder), patch(api.UpdateMCPFolder), del(api.DeleteMCPFolder))
	r.add("/api/v1/mcp/folders/:id/proxy", true, get(api.MCPFolderProxy), put(api.PutMCPFolderProxy), del(api.DeleteMCPFolderProxy))
	r.add("/api/v1/mcp/servers", true, get(api.ListMCPServers), post(api.CreateMCPServer))
	r.add("/api/v1/mcp/servers/:id", true, get(api.GetMCPServer), patch(api.UpdateMCPServer), del(api.DeleteMCPServer))
	r.add("/api/v1/mcp/servers/:id/check", true, post(api.CheckMCPServer))
	r.add("/api/v1/mcp/servers/:id/icon", true, get(api.MCPServerIcon))
	r.add("/api/v1/mcp/servers/:id/tools", true, get(api.MCPServerTools))
	r.add("/api/v1/mcp/servers/:id/tools/disabled", true, put(api.DisableMCPServerTools))
	r.add("/api/v1/mcp/servers/:id/access", true, get(api.MCPServerAccess), put(api.PutMCPServerAccess))
	r.add("/api/v1/mcp/servers/:id/calls", true, get(api.MCPServerCalls))
	r.add("/api/v1/mcp/servers/:id/proxy", true, get(api.MCPServerProxy), put(api.PutMCPServerProxy), del(api.DeleteMCPServerProxy))
	r.add("/api/v1/mcp/organizations/:id", true, get(api.MCPOrganizationSpace), put(api.PutMCPOrganizationSpace))
	r.add("/api/v1/mcp/organizations/:id/servers", true, get(api.MCPOrganizationSet), put(api.PutMCPOrganizationSet))
	r.add("/api/v1/mcp/organizations/:id/proxy", true, get(api.MCPOrganizationProxy), put(api.PutMCPOrganizationProxy),
		del(api.DeleteMCPOrganizationProxy))
	r.add("/api/v1/mcp/visibility", true, get(api.MCPVisibility))
	r.add("/api/v1/account/tokens/:id/mcp-servers", true, put(api.SetOwnTokenMCPServers))
	r.add("/api/v1/users/:id/tokens/:tid/mcp-servers", true, put(api.SetUserTokenMCPServers))
	r.add("/api/v1/account/apps/:id/mcp-servers", true, put(api.SetOwnAppMCPServers))
}
