package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

func (a *Handlers) currentUser(c fiber.Ctx) (accesstypes.Principal, accesstypes.User, error) {
	var p accesstypes.Principal
	var user accesstypes.User
	var err error
	if header, ok := authorization(c); ok {
		secret, isBearer := bearer(header)
		if !isBearer {
			return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
		}
		p, user, err = a.Access.Auth.AuthenticateToken(c.Context(), secret)
	} else {
		token := c.Cookies(SessionCookie)
		if token == "" {
			return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.Unauthenticated)
		}
		p, user, err = a.Access.Auth.Authenticate(c.Context(), token)
	}
	if err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, err
	}
	if p.App {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
	}
	if err := accesstypes.RequireScope(p, routeNeed(c.Method(), c.Route().Path)); err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, err
	}
	return p, user, nil
}

func authorization(c fiber.Ctx) (string, bool) {
	values := c.Request().Header.PeekAll(fiber.HeaderAuthorization)
	if len(values) == 0 {
		return "", false
	}
	return string(values[0]), true
}

func bearer(header string) (string, bool) {
	scheme, credentials, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(credentials), true
}

func routeNeed(method, route string) accesstypes.Need {
	switch {
	case route == "/api/v1/auth/me":
		return accesstypes.NeedAny
	case method == fiber.MethodPost && (route == "/api/v1/catalog/nodes/:id/links/:link_key/check" ||
		route == "/api/v1/catalog/nodes/:id/link-templates/preview"):
		return accesstypes.NeedRead
	case route == "/api/mcp":
		return accesstypes.NeedAny
	case route == "/api/v1/knowledge/search", route == "/api/v1/knowledge/search/modes", route == "/api/v1/link-icons/:id",
		route == "/api/v1/docs/space", route == "/api/v1/account/groups", route == "/api/v1/account/managed-groups",
		route == "/api/v1/account/managed-groups/:id", route == "/api/v1/account/managed-groups/:id/candidates":
		return accesstypes.NeedRead
	case route == "/api/v1/account/groups/:id/contacts", route == "/api/v1/account/managed-groups/:id/members/:user_id":
		return accesstypes.NeedWriteWhole
	case strings.HasPrefix(route, "/api/v1/catalog/"):
		if method == fiber.MethodGet || method == fiber.MethodHead {
			return accesstypes.NeedRead
		}
		return accesstypes.NeedWrite
	case route == "/api/v1/users/:id/tokens", route == "/api/v1/users/:id/apps", route == "/api/v1/users/:id/apps/:gid":
		return accesstypes.NeedSession
	case route == "/api/v1/users", strings.HasPrefix(route, "/api/v1/users/"):
		return globalNeed(method, accesstypes.NeedUsersRead, accesstypes.NeedUsersWrite)
	case route == "/api/v1/groups", strings.HasPrefix(route, "/api/v1/groups/"):
		return globalNeed(method, accesstypes.NeedGroupsRead, accesstypes.NeedGroupsWrite)
	case route == "/api/v1/clusters", strings.HasPrefix(route, "/api/v1/clusters/"):
		return globalNeed(method, accesstypes.NeedClustersRead, accesstypes.NeedClustersWrite)
	case route == "/api/v1/secrets", strings.HasPrefix(route, "/api/v1/secrets/"), route == "/api/v1/knowledge/scans":
		return globalNeed(method, accesstypes.NeedSettingsRead, accesstypes.NeedSettingsWrite)
	case route == "/api/v1/link-kinds", strings.HasPrefix(route, "/api/v1/link-kinds/"),
		route == "/api/v1/environments", strings.HasPrefix(route, "/api/v1/environments/"):
		return globalNeed(method, accesstypes.NeedRead, accesstypes.NeedSettingsWrite)
	}
	return accesstypes.NeedSession
}

func globalNeed(method string, read, write accesstypes.Need) accesstypes.Need {
	if method == fiber.MethodGet || method == fiber.MethodHead {
		return read
	}
	return write
}

func parseID(c fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.UUID{}, apperr.New(apperr.NotFound)
	}
	return id, nil
}
