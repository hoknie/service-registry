package middleware

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

func SameOrigin(c fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		if !isSameOrigin(c) && !(hasBearer(c) && c.Route().Path != loginPath) {
			return responses.NewAPIError(http.StatusForbidden, "auth.csrf_rejected",
				"cross-origin state-changing request rejected").Send(c)
		}
	}
	return c.Next()
}

const loginPath = "/api/v1/auth/login"

func hasBearer(c fiber.Ctx) bool {
	values := c.Request().Header.PeekAll(fiber.HeaderAuthorization)
	if len(values) == 0 {
		return false
	}
	scheme, _, ok := strings.Cut(string(values[0]), " ")
	return ok && strings.EqualFold(scheme, "Bearer")
}

func isSameOrigin(c fiber.Ctx) bool {
	h := &c.Request().Header
	if site := h.PeekAll("Sec-Fetch-Site"); len(site) > 0 {
		return string(site[0]) == "same-origin"
	}
	origins := h.PeekAll(fiber.HeaderOrigin)
	if len(origins) == 0 {
		return true
	}
	origin := string(origins[0])
	authority, ok := strings.CutPrefix(origin, "https://")
	if !ok {
		authority, ok = strings.CutPrefix(origin, "http://")
	}
	host := string(h.Host())
	return ok && host != "" && strings.EqualFold(authority, host)
}
