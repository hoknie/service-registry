package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListEvents(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Events
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	var filter ingest.EventFilter
	if q.Type != "" {
		filter.Type = &q.Type
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Ingest.ListEvents(c.Context(), p, id, filter, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.EventOf))
}

func (a *Handlers) ListDeployments(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Deployments
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	filter := ingest.NewDeploymentFilter(q.Service, q.Environment, q.Branch)
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Ingest.ListDeployments(c.Context(), p, id, filter, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.DeploymentOf))
}

func (a *Handlers) ListEnvironments(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Deploy.ListEnvironments(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.EnvironmentOf))
}
