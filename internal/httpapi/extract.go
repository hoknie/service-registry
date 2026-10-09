package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/service"
)

func (a *API) currentUser(c fiber.Ctx) (service.Principal, access.User, error) {
	var p service.Principal
	var user access.User
	var err error
	if header, ok := authorization(c); ok {
		secret, isBearer := bearer(header)
		if !isBearer {
			return service.Principal{}, access.User{}, apperr.New(apperr.InvalidToken)
		}
		p, user, err = service.AuthenticateToken(c.Context(), a.State, secret)
	} else {
		token := c.Cookies(SessionCookie)
		if token == "" {
			return service.Principal{}, access.User{}, apperr.New(apperr.Unauthenticated)
		}
		p, user, err = service.Authenticate(c.Context(), a.State, token)
	}
	if err != nil {
		return service.Principal{}, access.User{}, err
	}
	if err := service.RequireScope(p, routeNeed(c.Method(), c.Route().Path)); err != nil {
		return service.Principal{}, access.User{}, err
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

func routeNeed(method, route string) service.Need {
	switch {
	case route == "/api/v1/auth/me":
		return service.NeedAny
	case method == fiber.MethodPost && (route == "/api/v1/catalog/nodes/:id/links/:link_key/check" ||
		route == "/api/v1/catalog/nodes/:id/link-templates/preview"):
		return service.NeedRead
	case route == "/api/mcp":
		return service.NeedAny
	case route == "/api/v1/knowledge/search", route == "/api/v1/knowledge/search/modes":
		return service.NeedRead
	case strings.HasPrefix(route, "/api/v1/catalog/"):
		if method == fiber.MethodGet || method == fiber.MethodHead {
			return service.NeedRead
		}
		return service.NeedWrite
	case route == "/api/v1/users/:id/tokens":
		return service.NeedSession
	case route == "/api/v1/users", strings.HasPrefix(route, "/api/v1/users/"),
		route == "/api/v1/groups", strings.HasPrefix(route, "/api/v1/groups/"),
		route == "/api/v1/clusters", strings.HasPrefix(route, "/api/v1/clusters/"):
		return service.NeedAdmin
	case route == "/api/v1/link-kinds", strings.HasPrefix(route, "/api/v1/link-kinds/"),
		route == "/api/v1/environments", strings.HasPrefix(route, "/api/v1/environments/"):
		if method == fiber.MethodGet || method == fiber.MethodHead {
			return service.NeedRead
		}
		return service.NeedAdmin
	}
	return service.NeedSession
}

func parseID(c fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.UUID{}, apperr.New(apperr.NotFound)
	}
	return id, nil
}
