package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListDependencies(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	list, err := a.Topology.Dependencies.ProjectDependencies(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.DependenciesOf(list))
}

func (a *Handlers) AddDependency(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Dependency
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	item, err := a.Topology.Dependencies.AddDependency(c.Context(), p, id, *in.Target, in.Note)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.DependencyItemOf(item))
}

func (a *Handlers) DeleteDependency(c fiber.Ctx) error {
	return a.nodeChild(c, "dependency_id", a.Topology.Dependencies.DeleteDependency)
}

func (a *Handlers) DependencyGraph(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.DependencyGraphQuery
	if err := bindQuery(c, &q, topologytypes.InvalidDependencyDepth, nil); err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Topology.Dependencies.DependencyGraph(c.Context(), p, id, q.Depth)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.DependencyGraphOf(g))
}
