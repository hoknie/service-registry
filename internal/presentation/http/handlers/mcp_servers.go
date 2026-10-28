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

func (a *Handlers) ListMCPFolders(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var parent *uuid.UUID
		if raw := c.Query("parent_id"); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return nil, gatewaytypes.InvalidFilter
			}
			parent = &id
		}
		out, err := a.Gateway.Folders.ListFolders(c.Context(), p, parent)
		return responses.MCPFoldersOf(out), err
	})
}

func (a *Handlers) CreateMCPFolder(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusCreated, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var in requests.MCPFolderCreate
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Folders.CreateFolder(c.Context(), p, in.Input())
		return responses.MCPFolderOf(out), err
	})
}

func (a *Handlers) GetMCPFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Folders.GetFolder(c.Context(), p, id)
		return responses.MCPFolderOf(out), err
	})
}

func (a *Handlers) UpdateMCPFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPFolderPatch
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Folders.UpdateFolder(c.Context(), p, id, in.Patch())
		return responses.MCPFolderOf(out), err
	})
}

func (a *Handlers) DeleteMCPFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Gateway.Folders.DeleteFolder(c.Context(), p, id)
	})
}

func (a *Handlers) ListMCPServers(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var q requests.MCPServersQuery
		if err := c.Bind().Query(&q); err != nil {
			return nil, gatewaytypes.InvalidFilter
		}
		in, err := q.Input()
		if err != nil {
			return nil, err
		}
		out, err := a.Gateway.Servers.ListServers(c.Context(), p, in)
		return responses.MCPServerPageOf(out), err
	})
}

func (a *Handlers) CreateMCPServer(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusCreated, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var in requests.MCPServerCreate
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Servers.CreateServer(c.Context(), p, in.Input())
		return responses.MCPServerOf(out), err
	})
}

func (a *Handlers) GetMCPServer(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Servers.GetServer(c.Context(), p, id)
		return responses.MCPServerOf(out), err
	})
}

func (a *Handlers) UpdateMCPServer(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPServerPatch
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Servers.UpdateServer(c.Context(), p, id, in.Patch())
		return responses.MCPServerOf(out), err
	})
}

func (a *Handlers) DeleteMCPServer(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Gateway.Servers.DeleteServer(c.Context(), p, id)
	})
}

func (a *Handlers) CheckMCPServer(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Checks.CheckNow(c.Context(), p, id)
		return responses.MCPServerOf(out), err
	})
}

func (a *Handlers) MCPServerTools(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Gateway.Servers.ServerTools(c.Context(), p, id)
		return responses.MCPToolsOf(out), err
	})
}

func (a *Handlers) DisableMCPServerTools(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.MCPDisabledTools
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Gateway.Servers.DisableTools(c.Context(), p, id, *in.Tools)
		return responses.MCPToolsOf(out), err
	})
}

func (a *Handlers) MCPServerCalls(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var q requests.MCPCallsQuery
		if err := c.Bind().Query(&q); err != nil {
			return nil, gatewaytypes.InvalidFilter
		}
		in, err := q.Input()
		if err != nil {
			return nil, err
		}
		out, err := a.Gateway.Gateway.CallLog(c.Context(), p, id, in)
		return responses.MCPCallsOf(out), err
	})
}

func (a *Handlers) MCPServerIcon(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	icon, err := a.Gateway.Servers.ServerIcon(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderContentType, icon.Type)
	c.Set(fiber.HeaderCacheControl, "private, max-age=300")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderContentSecurityPolicy, "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	return c.Status(http.StatusOK).Send(icon.Data)
}
