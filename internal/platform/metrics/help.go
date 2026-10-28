package metrics

var helps = map[string]string{ // #nosec G101 -- help texts of metrics, not credentials
	"svcr_catalog_nodes":                      "Catalog nodes by kind.",
	"svcr_users":                              "Users by state.",
	"svcr_groups":                             "Groups.",
	"svcr_sessions_active":                    "Unexpired sessions.",
	"svcr_users_active":                       "Users with a session or personal token used within the window.",
	"svcr_credentials_active":                 "Usable credentials by kind.",
	"svcr_clusters":                           "Clusters by mode and state.",
	"svcr_cluster_workloads":                  "Observed workloads in clusters.",
	"svcr_service_environments":               "Current versions: pairs of service and environment.",
	"svcr_drifts":                             "Current pairs whose cluster version differs from the event.",
	"svcr_branches":                           "Branches by state.",
	"svcr_repo_sync_connections":              "Repository sync connections by hosting and state.",
	"svcr_link_targets":                       "Link addresses by check state.",
	"svcr_notification_subscriptions":         "Notification subscriptions by channel.",
	"svcr_knowledge_snapshots":                "Documentation snapshots.",
	"svcr_knowledge_files_pending":            "Documentation files without embeddings.",
	"svcr_project_info":                       "Project id, organization id and slug path.",
	"svcr_mcp_gateway_servers":                "MCP gateway servers by state.",
	"svcr_mcp_gateway_server_info":            "MCP gateway server id and slug.",
	"svcr_deployments_recent":                 "Deployments that occurred within the window.",
	"svcr_projects_covered":                   "Projects having the aspect.",
	"svcr_job_queue":                          "Background job items by state.",
	"svcr_db_pool_connections":                "Database pool connections by state.",
	"svcr_db_pool_max_connections":            "Database pool size limit.",
	"svcr_db_pool_acquires_total":             "Connections acquired from the pool.",
	"svcr_db_pool_acquire_wait_seconds_total": "Time spent waiting for a pool connection.",
	"svcr_db_pool_empty_acquires_total":       "Acquires that had to wait for a connection.",
}

func helpOf(name string) string {
	if h, ok := helps[name]; ok {
		return h
	}
	return name
}
