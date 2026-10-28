package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) MCPServerAccess(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Visibility.ServerAccess(c.Context(), p, id)
		return responses.MCPAccessOf(out), err
	})
}

func (a *Handlers) PutMCPServerAccess(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPAccess
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Visibility.PutServerAccess(c.Context(), p, id, *in.Everyone, in.Groups, in.Users)
		return responses.MCPAccessOf(out), err
	})
}

func (a *Handlers) MCPOrganizationSpace(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Spaces.OrganizationSpace(c.Context(), p, id)
		return responses.MCPSpaceOf(out), err
	})
}

func (a *Handlers) PutMCPOrganizationSpace(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPSpace
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Spaces.PutOrganizationSpace(c.Context(), p, id, *in.Description)
		return responses.MCPSpaceOf(out), err
	})
}

func (a *Handlers) MCPOrganizationSet(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Visibility.OrganizationSet(c.Context(), p, id)
		return responses.MCPOrganizationSetOf(out), err
	})
}

func (a *Handlers) PutMCPOrganizationSet(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPOrganizationSet
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Visibility.PutOrganizationSet(c.Context(), p, id, *in.Mode, in.Servers)
		return responses.MCPOrganizationSetOf(out), err
	})
}

func (a *Handlers) MCPVisibility(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var only *uuid.UUID
		if raw := c.Query("organization_id"); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return nil, gatewaytypes.InvalidFilter
			}
			only = &id
		}
		out, err := a.Gateway.Visibility.Summary(c.Context(), p, only)
		return responses.MCPVisibilityOf(out), err
	})
}

func (a *Handlers) mcpProxy(c fiber.Ctx, kind string) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Proxy.Proxy(c.Context(), p, kind, id)
		return responses.MCPProxySettingOf(out), err
	})
}

func (a *Handlers) putMCPProxy(c fiber.Ctx, kind string) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPProxy
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Proxy.PutProxy(c.Context(), p, kind, id, in.Proxy())
		return responses.MCPProxySettingOf(out), err
	})
}

func (a *Handlers) deleteMCPProxy(c fiber.Ctx, kind string) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Gateway.Proxy.DeleteProxy(c.Context(), p, kind, id)
	})
}

func (a *Handlers) MCPOrganizationProxy(c fiber.Ctx) error {
	return a.mcpProxy(c, gatewaytypes.OwnerOrganization)
}

func (a *Handlers) PutMCPOrganizationProxy(c fiber.Ctx) error {
	return a.putMCPProxy(c, gatewaytypes.OwnerOrganization)
}

func (a *Handlers) DeleteMCPOrganizationProxy(c fiber.Ctx) error {
	return a.deleteMCPProxy(c, gatewaytypes.OwnerOrganization)
}

func (a *Handlers) MCPFolderProxy(c fiber.Ctx) error { return a.mcpProxy(c, gatewaytypes.OwnerFolder) }

func (a *Handlers) PutMCPFolderProxy(c fiber.Ctx) error {
	return a.putMCPProxy(c, gatewaytypes.OwnerFolder)
}

func (a *Handlers) DeleteMCPFolderProxy(c fiber.Ctx) error {
	return a.deleteMCPProxy(c, gatewaytypes.OwnerFolder)
}

func (a *Handlers) MCPServerProxy(c fiber.Ctx) error { return a.mcpProxy(c, gatewaytypes.OwnerServer) }

func (a *Handlers) PutMCPServerProxy(c fiber.Ctx) error {
	return a.putMCPProxy(c, gatewaytypes.OwnerServer)
}

func (a *Handlers) DeleteMCPServerProxy(c fiber.Ctx) error {
	return a.deleteMCPProxy(c, gatewaytypes.OwnerServer)
}
