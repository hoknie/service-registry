package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) CatalogTable(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.CatalogTable
	if err := bindQuery(c, &q, catalog.InvalidFilter, map[string]error{
		"parent": apperr.New(apperr.NotFound), "limit": access.InvalidPagination, "offset": access.InvalidPagination,
	}); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Catalog.CatalogTable(c.Context(), p, q.Parent, q.Query(), q.Filter())
	if err != nil {
		return responses.Fail(c, err)
	}
	nodes, err := a.withManaged(c, responses.TableNodes(page))
	if err != nil {
		return responses.Fail(c, err)
	}
	badges, err := a.Links.ProjectBadges(c.Context(), page.ReadableProjects())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.CatalogTableOf(page, nodes, badges))
}
