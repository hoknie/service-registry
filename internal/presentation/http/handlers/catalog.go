package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListNodes(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Children
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, map[string]error{"parent": apperr.New(apperr.NotFound)}); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Catalog.Nodes.Children(c.Context(), p, q.Parent, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.WalkedNode))
}

func (a *Handlers) Tree(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Tree
	if err := bindQuery(c, &q, catalogtypes.InvalidDepth, map[string]error{"root": apperr.New(apperr.NotFound)}); err != nil {
		return responses.Fail(c, err)
	}
	nodes, truncated, err := a.Catalog.Nodes.Tree(c.Context(), p, q.Root, q.Depth)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.Tree{Nodes: responses.ItemsOf(nodes, responses.WalkedNode).Items, Truncated: truncated})
}

func (a *Handlers) CreateNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateNode
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	node, key, err := a.Compose.Projects.CreateNode(c.Context(), p, in.Node(), in.SourceInput())
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.sendNode(c, http.StatusCreated, responses.CreatedNode(node, key))
}

func (a *Handlers) GetNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	card, err := a.Compose.Projects.ProjectCard(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.sendNode(c, http.StatusOK, responses.ProjectCardOf(card))
}

func (a *Handlers) UpdateNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.UpdateNode
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	node, err := a.Catalog.Nodes.UpdateNode(c.Context(), p, id, in.Changes())
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.sendNode(c, http.StatusOK, responses.ReadNode(node))
}

func (a *Handlers) MoveNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.MoveNode
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	if !in.ParentID.Present {
		return responses.Fail(c, InvalidBody("parent_id is required"))
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var parent *uuid.UUID
	if in.ParentID.Value != nil {
		pid, err := uuid.Parse(*in.ParentID.Value)
		if err != nil {
			return responses.Fail(c, apperr.New(apperr.NotFound))
		}
		parent = &pid
	}
	node, err := a.Catalog.Nodes.MoveNode(c.Context(), p, id, parent)
	if err != nil {
		return responses.Fail(c, err)
	}
	return a.sendNode(c, http.StatusOK, responses.ReadNode(node))
}

func (a *Handlers) DeleteNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Catalog.Nodes.DeleteNode(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) sendNode(c fiber.Ctx, status int, node responses.Node) error {
	return c.Status(status).JSON(node)
}
