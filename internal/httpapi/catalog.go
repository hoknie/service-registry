package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListNodes(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Children
	if err := bindQuery(c, &q, access.InvalidPagination, map[string]error{"parent": apperr.New(apperr.NotFound)}); err != nil {
		return Fail(c, err)
	}
	page, err := service.Children(c.Context(), a.State, p, q.Parent, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	body := response.PageOf(page, response.WalkedNode)
	if body.Items, err = a.withManaged(c, body.Items); err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(body)
}

func (a *API) Tree(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Tree
	if err := bindQuery(c, &q, catalog.InvalidDepth, map[string]error{"root": apperr.New(apperr.NotFound)}); err != nil {
		return Fail(c, err)
	}
	nodes, truncated, err := service.Tree(c.Context(), a.State, p, q.Root, q.Depth)
	if err != nil {
		return Fail(c, err)
	}
	items, err := a.withManaged(c, response.ItemsOf(nodes, response.WalkedNode).Items)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.Tree{Nodes: items, Truncated: truncated})
}

func (a *API) CreateNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateNode
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	node, key, err := service.CreateNode(c.Context(), a.State, p, in.Node(), in.SourceInput())
	if err != nil {
		return Fail(c, err)
	}
	return a.sendNode(c, http.StatusCreated, response.CreatedNode(node, key))
}

func (a *API) GetNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	scoped, err := service.GetNode(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	body := response.ScopedNode(scoped)
	if body.Access == "read" && scoped.Node.Kind == catalog.KindProject {
		repo, err := service.ProjectRepository(c.Context(), a.State, id)
		if err != nil {
			return Fail(c, err)
		}
		r := response.RepositoryOf(repo)
		body.Repository = &r
	}
	return a.sendNode(c, http.StatusOK, body)
}

func (a *API) UpdateNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.UpdateNode
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	node, err := service.UpdateNode(c.Context(), a.State, p, id, in.Changes())
	if err != nil {
		return Fail(c, err)
	}
	return a.sendNode(c, http.StatusOK, response.ReadNode(node))
}

func (a *API) MoveNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.MoveNode
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	if !in.ParentID.Present {
		return Fail(c, InvalidBody("parent_id is required"))
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	var parent *uuid.UUID
	if in.ParentID.Value != nil {
		pid, err := uuid.Parse(*in.ParentID.Value)
		if err != nil {
			return Fail(c, apperr.New(apperr.NotFound))
		}
		parent = &pid
	}
	node, err := service.MoveNode(c.Context(), a.State, p, id, parent)
	if err != nil {
		return Fail(c, err)
	}
	return a.sendNode(c, http.StatusOK, response.ReadNode(node))
}

func (a *API) DeleteNode(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.DeleteNode(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *API) withManaged(c fiber.Ctx, nodes []response.Node) ([]response.Node, error) {
	ids := make([]uuid.UUID, 0, len(nodes))
	for _, n := range nodes {
		if n.Access == "read" {
			ids = append(ids, n.ID)
		}
	}
	managed, err := service.ManagedNodes(c.Context(), a.State, ids)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		nodes[i] = nodes[i].WithManaged(managed)
	}
	return nodes, nil
}

func (a *API) sendNode(c fiber.Ctx, status int, node response.Node) error {
	nodes, err := a.withManaged(c, []response.Node{node})
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(status).JSON(nodes[0])
}
