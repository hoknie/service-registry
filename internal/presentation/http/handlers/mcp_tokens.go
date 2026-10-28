package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) SetOwnTokenMCPServers(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPServerList
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Access.Tokens.SetOwnTokenServers(c.Context(), p, id, in.Servers)
		return responses.TokenOf(out), err
	})
}

func (a *Handlers) SetUserTokenMCPServers(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, userID uuid.UUID) (any, error) {
		tokenID, err := parseID(c, "tid")
		if err != nil {
			return nil, err
		}
		var in requests.MCPServerList
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Access.Tokens.SetUserTokenServers(c.Context(), p, userID, tokenID, in.Servers)
		return responses.OwnedTokenOf(out), err
	})
}

func (a *Handlers) SetOwnAppMCPServers(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPServerList
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Access.Apps.SetOwnAppServers(c.Context(), p, id, in.Servers)
		return responses.AppOf(out), err
	})
}
