package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/access"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/ingest"
	"svc-registry/internal/service"
)

func (a *API) ListEvents(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Events
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	var filter ingest.EventFilter
	if q.Type != "" {
		filter.Type = &q.Type
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	page, err := service.ListEvents(c.Context(), a.State, p, id, filter, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.EventOf))
}

func (a *API) ListDeployments(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Deployments
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	filter := ingest.NewDeploymentFilter(q.Service, q.Environment, q.Branch)
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	page, err := service.ListDeployments(c.Context(), a.State, p, id, filter, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.DeploymentOf))
}

func (a *API) ListEnvironments(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.ListEnvironments(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(items, response.EnvironmentOf))
}
