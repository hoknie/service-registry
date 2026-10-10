package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
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
	if err := bindQuery(c, &q, access.InvalidPagination, map[string]error{"parent": apperr.New(apperr.NotFound)}); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Catalog.Children(c.Context(), p, q.Parent, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	body := responses.PageOf(page, responses.WalkedNode)
	if body.Items, err = a.withManaged(c, body.Items); err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(body)
}

func (a *Handlers) Tree(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Tree
	if err := bindQuery(c, &q, catalog.InvalidDepth, map[string]error{"root": apperr.New(apperr.NotFound)}); err != nil {
		return responses.Fail(c, err)
	}
	nodes, truncated, err := a.Catalog.Tree(c.Context(), p, q.Root, q.Depth)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.withManaged(c, responses.ItemsOf(nodes, responses.WalkedNode).Items)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.Tree{Nodes: items, Truncated: truncated})
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
	var source catalog.SourceFactory
	if input := in.SourceInput(); input != nil {
		source = a.Knowledge.SourceFor(*input)
	}
	node, key, err := a.Catalog.CreateNode(c.Context(), p, in.Node(), source)
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
	scoped, err := a.Catalog.GetNode(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	body := responses.ScopedNode(scoped)
	if body.Access == "read" && scoped.Node.Kind == catalog.KindProject {
		repo, err := a.Forge.ProjectRepository(c.Context(), id)
		if err != nil {
			return responses.Fail(c, err)
		}
		r := responses.RepositoryOf(repo)
		body.Repository = &r
	}
	return a.sendNode(c, http.StatusOK, body)
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
	node, err := a.Catalog.UpdateNode(c.Context(), p, id, in.Changes())
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
	node, err := a.Catalog.MoveNode(c.Context(), p, id, parent)
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
	if err := a.Catalog.DeleteNode(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) withManaged(c fiber.Ctx, nodes []responses.Node) ([]responses.Node, error) {
	ids := make([]uuid.UUID, 0, len(nodes))
	for _, n := range nodes {
		if n.Access == "read" {
			ids = append(ids, n.ID)
		}
	}
	managed, err := a.Forge.Managed(c.Context(), ids)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		nodes[i] = nodes[i].WithManaged(managed)
	}
	return nodes, nil
}

func (a *Handlers) sendNode(c fiber.Ctx, status int, node responses.Node) error {
	nodes, err := a.withManaged(c, []responses.Node{node})
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(status).JSON(nodes[0])
}
