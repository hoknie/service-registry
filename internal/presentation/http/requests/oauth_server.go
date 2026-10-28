package requests

type ApproveOAuth struct {
	Nodes      *[]string `json:"nodes"`
	MCPServers *[]string `json:"mcp_servers"`
}

type MCPServerList struct {
	Servers *[]string `json:"servers"`
}
