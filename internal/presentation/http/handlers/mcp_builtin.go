package handlers

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/mcp"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) builtinAnswer(c fiber.Ctx, disabled []string, canWrite bool) error {
	tools, err := mcp.Catalog(c.Context(), a.mcpDeps(), a.Version)
	if err != nil {
		return responses.Fail(c, err)
	}
	endpoint := "/api/mcp"
	if base := strings.TrimSuffix(a.Config.Web.PublicURL, "/"); base != "" {
		endpoint = base + endpoint
	}
	return c.Status(http.StatusOK).JSON(responses.MCPBuiltinOf(a.Version, mcp.Instructions(a.mcpDeps()), endpoint, tools, disabled, canWrite))
}

func (a *Handlers) MCPBuiltin(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	disabled, canWrite, err := a.Gateway.Servers.BuiltinTools(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.builtinAnswer(c, disabled, canWrite)
}

func (a *Handlers) DisableMCPBuiltinTools(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.MCPDisabledTools
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	disabled, canWrite, err := a.Gateway.Servers.DisableBuiltinTools(c.Context(), p, *in.Tools)
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.builtinAnswer(c, disabled, canWrite)
}
