package http

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/platform/config"
	"svc-registry/internal/presentation/http/handlers"
	"svc-registry/internal/presentation/http/middleware"
	"svc-registry/internal/presentation/http/webui"
)

func NewRouter(d Deps, dist webui.Dist) *fiber.App {
	api := d.handlers()
	app := newApp(api)
	if d.Publishing != nil {
		app.Use(middleware.PublicationHost(d.Publishing.Hosts.PublicationHost, handlers.PublicationHostKey, webui.ServePublicHost(dist), handlers.APINotFound))
	}
	if d.Metrics != nil {
		app.Use(middleware.Metrics(d.Metrics))
	}
	app.Use(middleware.RequestLog())
	r := routes{app: app}

	r.add("/api/health", false, get(api.Health))
	r.add("/api/ready", false, get(api.Ready))

	r.add("/api/v1/auth/login", true, post(api.CountLogin(api.Login)))
	r.add("/api/v1/auth/logout", true, post(api.Logout))
	r.add("/api/v1/auth/me", true, get(api.Me))
	r.add("/api/v1/auth/password", true, post(api.ChangePassword))
	r.add("/api/v1/providers", false, get(api.Providers))
	r.add("/api/v1/auth/oauth/:provider/start", false, get(api.OAuthStart))
	r.add("/api/v1/auth/oauth/:provider/callback", false, get(api.CountOAuthLogin(api.OAuthCallback)))
	r.add("/api/v1/users", true, get(api.ListUsers), post(api.CreateUser))
	r.add("/api/v1/users/:id", true, get(api.GetUser), patch(api.UpdateUser))
	r.add("/api/v1/users/:id/password", true, post(api.ResetPassword))
	r.add("/api/v1/users/:id/tokens", true, get(api.ListUserTokens), post(api.IssueUserToken))
	r.add("/api/v1/users/:id/identities", true, get(api.UserIdentities))
	r.add("/api/v1/users/:id/groups", true, get(api.UserGroups))
	r.add("/api/v1/account/identities", true, get(api.OwnIdentities))
	r.add("/api/v1/account/identities/:id", true, del(api.UnlinkIdentity))
	r.add("/api/v1/account/tokens", true, get(api.ListOwnTokens), post(api.IssueToken))
	r.add("/api/v1/account/tokens/:id", true, del(api.RevokeOwnToken))
	r.add("/api/v1/tokens", true, get(api.ListTokens))
	r.add("/api/v1/tokens/:id", true, del(api.RevokeToken))
	r.add("/api/v1/link-kinds", true, get(api.ListLinkKinds), post(api.CreateLinkKind), put(api.ReorderLinkKinds))
	r.add("/api/v1/link-kinds/:key", true, patch(api.UpdateLinkKind), del(api.DeleteLinkKind))
	r.add("/api/v1/clusters", true, get(api.ListClusters), post(api.CreateCluster))
	r.add("/api/v1/clusters/:id", true, get(api.GetCluster), patch(api.UpdateCluster), del(api.DeleteCluster))
	r.add("/api/v1/clusters/:id/test", true, post(api.TestCluster))
	r.add("/api/v1/clusters/:id/poll", true, post(api.PollCluster))
	r.add("/api/v1/clusters/:id/unmatched", true, get(api.UnmatchedWorkloads))
	r.add("/api/v1/clusters/:id/agent-token", true, post(api.RotateAgentToken), del(api.RevokeAgentTokens))
	r.add("/api/v1/environments", true, get(api.ListEnvironmentsDirectory), post(api.CreateEnvironment))
	r.add("/api/v1/environments/:key", true, patch(api.UpdateEnvironment), del(api.DeleteEnvironment))
	r.add("/api/v1/groups", true, get(api.ListGroups), post(api.CreateGroup))
	r.add("/api/v1/groups/:id", true, get(api.GetGroup), patch(api.RenameGroup), del(api.DeleteGroup))
	r.add("/api/v1/groups/:id/members/:user_id", true, put(api.AddMember), del(api.RemoveMember))
	r.add("/api/v1/groups/:id/contacts", true, put(api.SetGroupContacts))
	r.add("/api/v1/groups/:id/permissions", true, put(api.SetGroupPermissions))
	r.add("/api/v1/groups/:id/contact-editors/:user_id", true, put(api.GrantContactEditor), del(api.RevokeContactEditor))
	r.add("/api/v1/catalog/tree", true, get(api.Tree))
	r.add("/api/v1/catalog/nodes", true, get(api.ListNodes), post(api.CreateNode))
	r.add("/api/v1/catalog/nodes/:id", true, get(api.GetNode), patch(api.UpdateNode), del(api.DeleteNode))
	r.add("/api/v1/catalog/nodes/:id/activity", true, get(api.NodeActivity))
	r.add("/api/v1/catalog/labels", true, get(api.CatalogLabels))
	r.add("/api/v1/catalog/table", true, get(api.CatalogTable))
	r.add("/api/v1/catalog/owners", true, get(api.SuggestOwners))
	r.add("/api/v1/catalog/nodes/:id/owners", true, get(api.NodeOwners), put(api.PutNodeOwners))
	r.add("/api/v1/catalog/nodes/:id/owners/candidates", true, get(api.OwnerCandidates))
	r.add("/api/v1/catalog/nodes/:id/dependencies", true, get(api.ListDependencies), post(api.AddDependency))
	r.add("/api/v1/catalog/nodes/:id/dependencies/graph", true, get(api.DependencyGraph))
	r.add("/api/v1/catalog/nodes/:id/dependencies/:dependency_id", true, del(api.DeleteDependency))
	r.add("/api/v1/catalog/nodes/:id/notification-subscriptions", true, get(api.ListSubscriptions))
	r.add("/api/v1/catalog/nodes/:id/notification-subscriptions/:key", true, put(api.PutSubscription), del(api.DeleteSubscription))
	r.add("/api/v1/catalog/nodes/:id/notification-subscriptions/:key/test", true, post(api.TestSubscription))
	r.add("/api/v1/catalog/nodes/:id/notification-subscriptions/:key/signing-secret", true, post(api.RotateSigningSecret))
	r.add("/api/v1/catalog/nodes/:id/notification-deliveries", true, get(api.ListDeliveries))
	r.add("/api/v1/catalog/nodes/:id/link-icons", true, post(api.UploadLinkIcon))
	r.add("/api/v1/link-icons/:id", true, get(api.LinkIcon))
	r.add("/api/v1/catalog/nodes/:id/move", true, post(api.MoveNode))
	r.add("/api/v1/catalog/nodes/:id/visibility", true, put(api.SetNodeVisibility))
	r.add("/api/v1/catalog/nodes/:id/bindings", true, get(api.ListBindings), post(api.GrantRole))
	r.add("/api/v1/catalog/nodes/:id/bindings/:binding_id", true, del(api.RevokeBinding))
	r.add("/api/v1/catalog/nodes/:id/groups", true, get(api.ListGroups), post(api.CreateGroup))
	r.add("/api/v1/catalog/nodes/:id/groups/:group_id", true, get(api.GetGroup), patch(api.RenameGroup), del(api.DeleteGroup))
	r.add("/api/v1/catalog/nodes/:id/groups/:group_id/members/:user_id", true, put(api.AddMember), del(api.RemoveMember))
	r.add("/api/v1/catalog/nodes/:id/groups/:group_id/candidates", true, get(api.MemberCandidates))
	r.add("/api/v1/catalog/nodes/:id/secrets", true, get(api.ListNodeSecrets), post(api.CreateNodeSecret))
	r.add("/api/v1/catalog/nodes/:id/secrets/:sid", true, patch(api.UpdateNodeSecret), del(api.DeleteNodeSecret))
	r.add("/api/v1/secrets", true, get(api.ListSecrets), post(api.CreateSecret))
	r.add("/api/v1/secrets/:sid", true, get(api.GetSecret), patch(api.UpdateSecret), del(api.DeleteSecret))
	r.add("/api/v1/catalog/nodes/:id/keys", true, get(api.ListKeys), post(api.RotateKey))
	r.add("/api/v1/catalog/nodes/:id/keys/:key_id", true, del(api.RevokeKey))
	r.add("/api/v1/catalog/nodes/:id/events", true, get(api.ListEvents))
	r.add("/api/v1/catalog/nodes/:id/deployments", true, get(api.ListDeployments))
	r.add("/api/v1/catalog/nodes/:id/environments", true, get(api.ListEnvironments))
	r.add("/api/v1/catalog/nodes/:id/readme", true, get(api.Readme))
	r.add("/api/v1/catalog/nodes/:id/branches", true, get(api.ListBranches))
	r.add("/api/v1/catalog/nodes/:id/branches/item", true, get(api.GetBranch), del(api.DeleteBranch))
	r.add("/api/v1/catalog/nodes/:id/branches/pin", true, put(api.PinBranch), del(api.UnpinBranch))
	r.add("/api/v1/catalog/nodes/:id/knowledge", true, get(api.GetKnowledge))
	r.add("/api/v1/catalog/nodes/:id/knowledge/settings", true, get(api.GetKnowledgeSettings), put(api.PutKnowledgeSettings))
	r.add("/api/v1/catalog/nodes/:id/publication", true, get(api.GetPublication), put(api.PutPublication))
	r.add("/api/v1/catalog/nodes/:id/knowledge/collect", true, post(api.CollectKnowledge))
	r.add("/api/v1/catalog/nodes/:id/knowledge/files", true, get(api.KnowledgeFiles))
	r.add("/api/v1/catalog/nodes/:id/knowledge/tree", true, get(api.KnowledgeTree))
	r.add("/api/v1/catalog/nodes/:id/knowledge/file", true, get(api.KnowledgeFile))
	r.add("/api/v1/catalog/nodes/:id/knowledge/apis", true, get(api.KnowledgeAPIs))
	r.add("/api/v1/catalog/nodes/:id/knowledge/api", true, get(api.KnowledgeAPI))
	r.add("/api/v1/catalog/nodes/:id/knowledge/specs", true, get(api.KnowledgeSpecs))
	r.add("/api/v1/catalog/nodes/:id/knowledge/changes", true, get(api.KnowledgeChanges))
	r.add("/api/v1/catalog/nodes/:id/knowledge/adrs", true, get(api.KnowledgeADRs))
	r.add("/api/v1/catalog/nodes/:id/knowledge/source", true, get(api.GetKnowledgeSource), put(api.PutKnowledgeSource), del(api.DeleteKnowledgeSource))
	r.add("/api/v1/catalog/nodes/:id/knowledge/source/check", true, post(api.CheckKnowledgeSource))
	r.add("/api/v1/catalog/nodes/:id/knowledge/scans", true, get(api.ListProjectScans))
	r.add("/api/v1/docs/space", true, get(api.DocSpace))
	r.add("/api/v1/knowledge/search", true, get(api.SearchKnowledge))
	r.add("/api/v1/knowledge/search/modes", true, get(api.KnowledgeSearchModes))
	r.add("/api/v1/knowledge/scans", true, get(api.ListKnowledgeScans))
	r.add("/api/mcp", false, post(api.MCP))
	r.add("/.well-known/oauth-protected-resource", false, get(api.ProtectedResourceMetadata))
	r.add("/.well-known/oauth-protected-resource/api/mcp", false, get(api.ProtectedResourceMetadata))
	r.add("/.well-known/oauth-authorization-server", false, get(api.AuthorizationServerMetadata))
	if suffix := publicPath(d.Config); suffix != "" {
		r.add("/.well-known/oauth-authorization-server"+suffix, false, get(api.AuthorizationServerMetadata))
	}
	r.add("/api/oauth/register", false, post(api.RegisterOAuthClient))
	r.add("/api/oauth/authorize", false, get(api.OAuthAuthorize))
	r.add("/api/oauth/token", false, post(api.OAuthToken))
	r.add("/api/oauth/revoke", false, post(api.OAuthRevoke))
	r.add("/api/v1/oauth/requests/:id", true, get(api.OAuthRequest))
	r.add("/api/v1/oauth/requests/:id/approve", true, post(api.ApproveOAuthRequest))
	r.add("/api/v1/oauth/requests/:id/deny", true, post(api.DenyOAuthRequest))
	r.add("/api/v1/account/apps", true, get(api.ListOwnApps))
	r.add("/api/v1/account/groups", true, get(api.OwnGroups))
	r.add("/api/v1/account/groups/:id/contacts", true, put(api.SetOwnGroupContacts))
	r.add("/api/v1/account/managed-groups", true, get(api.ManagedGroups))
	r.add("/api/v1/account/managed-groups/:id", true, get(api.ManagedGroup))
	r.add("/api/v1/account/managed-groups/:id/members/:user_id", true, put(api.AddMember), del(api.RemoveMember))
	r.add("/api/v1/account/managed-groups/:id/candidates", true, get(api.MemberCandidates))
	r.add("/api/v1/account/apps/:id", true, del(api.RevokeOwnApp))
	r.add("/api/v1/users/:id/apps", true, get(api.ListUserApps))
	r.add("/api/v1/users/:id/apps/:gid", true, del(api.RevokeUserApp))
	r.add("/api/v1/catalog/nodes/:id/link-templates", true, get(api.ListLinkTemplates))
	r.add("/api/v1/catalog/nodes/:id/link-templates/preview", true, post(api.PreviewLinkTemplate))
	r.add("/api/v1/catalog/nodes/:id/link-templates/:link_key", true, put(api.PutLinkTemplate), del(api.DeleteLinkTemplate))
	r.add("/api/v1/catalog/nodes/:id/vars", true, get(api.ListNodeVars), put(api.PutNodeVars))
	r.add("/api/v1/catalog/nodes/:id/links", true, get(api.ProjectLinks))
	r.add("/api/v1/catalog/nodes/:id/links/:link_key/check", true, post(api.CheckLink))
	r.add("/api/v1/catalog/nodes/:id/links/:link_key/checks", true, get(api.LinkChecks))
	r.add("/api/v1/catalog/nodes/:id/forge/connections", true, get(api.ListConnections), post(api.CreateConnection))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid", true,
		get(api.GetConnection), patch(api.UpdateConnection), del(api.DeleteConnection))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/check", true, post(api.CheckConnection))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/preview", true, post(api.PreviewConnection))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/sync", true, post(api.StartSync))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/runs", true, get(api.ListRuns))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/webhook", true, post(api.SetupWebhook), del(api.DeleteWebhook))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/exclusions", true, get(api.ListExclusions))
	r.add("/api/v1/catalog/nodes/:id/forge/connections/:cid/exclusions/:external_id", true, del(api.DeleteExclusion))
	r.add("/api/v1/catalog/nodes/:id/forge/unlink", true, post(api.UnlinkProject))
	r.add("/api/v1/ingest/projects/:project_id/events", false, post(api.CountIngest(api.IngestEvent)))
	r.add("/api/v1/ingest/clusters/:id/config", false, get(api.AgentConfig))
	r.add("/api/v1/ingest/clusters/:id/reports", false, post(api.CountAgentReport(api.AgentReport)))
	r.add("/api/v1/forge/hooks/:connection_id", false, post(api.CountForgeHook(api.ForgeHook)))

	if d.Metrics != nil {
		r.add("/metrics", false, get(api.MetricsPage()))
		r.add("/api/v1/system/metrics", true, get(api.SystemMetrics))
		r.add("/api/v1/system/metrics/family", true, get(api.SystemMetricsFamily))
	}
	r.add("/api/public/docs/space", false, get(api.PublicSpace))
	r.add("/api/public/docs/tree", false, get(api.PublicTree))
	r.add("/api/public/docs/files", false, get(api.PublicFiles))
	r.add("/api/public/docs/file", false, get(api.PublicFile))
	r.skills(api)
	r.gateway(api)
	r.members(api)
	r.add("/api/public/docs/api", false, get(api.PublicAPI))
	r.add("/api/public/docs/search", false, get(api.PublicSearch))
	app.All("/api", handlers.APINotFound)
	app.All("/api/*", handlers.APINotFound)
	app.Use(webui.Serve(dist))
	return app
}

func publicPath(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	u, err := url.Parse(cfg.Web.PublicURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.EscapedPath(), "/")
}

type route struct {
	methods []string
	handler fiber.Handler
}

func get(h fiber.Handler) route   { return route{[]string{fiber.MethodGet, fiber.MethodHead}, h} }
func post(h fiber.Handler) route  { return route{[]string{fiber.MethodPost}, h} }
func put(h fiber.Handler) route   { return route{[]string{fiber.MethodPut}, h} }
func patch(h fiber.Handler) route { return route{[]string{fiber.MethodPatch}, h} }
func del(h fiber.Handler) route   { return route{[]string{fiber.MethodDelete}, h} }

type routes struct{ app *fiber.App }

func (r routes) add(path string, csrf bool, methods ...route) {
	var allow []string
	for _, m := range methods {
		allow = append(allow, m.methods...)
		if csrf {
			r.app.Add(m.methods, path, middleware.SameOrigin, m.handler)
		} else {
			r.app.Add(m.methods, path, m.handler)
		}
	}
	notAllowed := handlers.APIMethodNotAllowedWith(strings.Join(allow, ","))
	if csrf {
		r.app.All(path, middleware.SameOrigin, notAllowed)
	} else {
		r.app.All(path, notAllowed)
	}
}
