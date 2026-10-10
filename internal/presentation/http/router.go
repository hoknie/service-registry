package http

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/handlers"
	"svc-registry/internal/presentation/http/middleware"
	"svc-registry/internal/presentation/http/webui"
)

func NewRouter(api *handlers.Handlers, dist webui.Dist) *fiber.App {
	app := NewApp(api)
	app.Use(middleware.RequestLog())
	r := routes{app: app}

	r.add("/api/health", false, get(api.Health))
	r.add("/api/ready", false, get(api.Ready))

	r.add("/api/v1/auth/login", true, post(api.Login))
	r.add("/api/v1/auth/logout", true, post(api.Logout))
	r.add("/api/v1/auth/me", true, get(api.Me))
	r.add("/api/v1/auth/password", true, post(api.ChangePassword))
	r.add("/api/v1/providers", false, get(api.Providers))
	r.add("/api/v1/auth/oauth/:provider/start", false, get(api.OAuthStart))
	r.add("/api/v1/auth/oauth/:provider/callback", false, get(api.OAuthCallback))
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
	r.add("/api/v1/link-kinds", true, get(api.ListLinkKinds), post(api.CreateLinkKind))
	r.add("/api/v1/link-kinds/:key", true, patch(api.UpdateLinkKind), del(api.DeleteLinkKind))
	r.add("/api/v1/clusters", true, get(api.ListClusters), post(api.CreateCluster))
	r.add("/api/v1/clusters/:id", true, get(api.GetCluster), patch(api.UpdateCluster), del(api.DeleteCluster))
	r.add("/api/v1/clusters/:id/test", true, post(api.TestCluster))
	r.add("/api/v1/clusters/:id/poll", true, post(api.PollCluster))
	r.add("/api/v1/clusters/:id/unmatched", true, get(api.UnmatchedWorkloads))
	r.add("/api/v1/environments", true, get(api.ListEnvironmentsDirectory), post(api.CreateEnvironment))
	r.add("/api/v1/environments/:key", true, patch(api.UpdateEnvironment), del(api.DeleteEnvironment))
	r.add("/api/v1/groups", true, get(api.ListGroups), post(api.CreateGroup))
	r.add("/api/v1/groups/:id", true, get(api.GetGroup), patch(api.RenameGroup), del(api.DeleteGroup))
	r.add("/api/v1/groups/:id/members/:user_id", true, put(api.AddMember), del(api.RemoveMember))
	r.add("/api/v1/catalog/tree", true, get(api.Tree))
	r.add("/api/v1/catalog/nodes", true, get(api.ListNodes), post(api.CreateNode))
	r.add("/api/v1/catalog/nodes/:id", true, get(api.GetNode), patch(api.UpdateNode), del(api.DeleteNode))
	r.add("/api/v1/catalog/nodes/:id/activity", true, get(api.NodeActivity))
	r.add("/api/v1/catalog/table", true, get(api.CatalogTable))
	r.add("/api/v1/catalog/nodes/:id/link-icons", true, post(api.UploadLinkIcon))
	r.add("/api/v1/link-icons/:id", true, get(api.LinkIcon))
	r.add("/api/v1/catalog/nodes/:id/move", true, post(api.MoveNode))
	r.add("/api/v1/catalog/nodes/:id/bindings", true, get(api.ListBindings), post(api.GrantRole))
	r.add("/api/v1/catalog/nodes/:id/bindings/:binding_id", true, del(api.RevokeBinding))
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
	r.add("/api/v1/catalog/nodes/:id/knowledge/collect", true, post(api.CollectKnowledge))
	r.add("/api/v1/catalog/nodes/:id/knowledge/files", true, get(api.KnowledgeFiles))
	r.add("/api/v1/catalog/nodes/:id/knowledge/file", true, get(api.KnowledgeFile))
	r.add("/api/v1/catalog/nodes/:id/knowledge/specs", true, get(api.KnowledgeSpecs))
	r.add("/api/v1/catalog/nodes/:id/knowledge/changes", true, get(api.KnowledgeChanges))
	r.add("/api/v1/catalog/nodes/:id/knowledge/adrs", true, get(api.KnowledgeADRs))
	r.add("/api/v1/catalog/nodes/:id/knowledge/source", true, get(api.GetKnowledgeSource), put(api.PutKnowledgeSource), del(api.DeleteKnowledgeSource))
	r.add("/api/v1/catalog/nodes/:id/knowledge/source/check", true, post(api.CheckKnowledgeSource))
	r.add("/api/v1/knowledge/search", true, get(api.SearchKnowledge))
	r.add("/api/v1/knowledge/search/modes", true, get(api.KnowledgeSearchModes))
	r.add("/api/v1/knowledge/scans", true, get(api.ListKnowledgeScans))
	r.add("/api/mcp", false, post(api.MCP))
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
	r.add("/api/v1/ingest/projects/:project_id/events", false, post(api.IngestEvent))
	r.add("/api/v1/forge/hooks/:connection_id", false, post(api.ForgeHook))

	app.All("/api", handlers.APINotFound)
	app.All("/api/*", handlers.APINotFound)
	app.Use(webui.Serve(dist))
	return app
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
