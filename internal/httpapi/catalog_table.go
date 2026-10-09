package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) CatalogTable(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.CatalogTable
	if err := bindQuery(c, &q, catalog.InvalidFilter, map[string]error{
		"parent": apperr.New(apperr.NotFound), "limit": access.InvalidPagination, "offset": access.InvalidPagination,
	}); err != nil {
		return Fail(c, err)
	}
	page, err := service.CatalogTable(c.Context(), a.State, p, q.Parent, q.Query(), q.Filter())
	if err != nil {
		return Fail(c, err)
	}
	nodes, err := a.withManaged(c, response.TableNodes(page))
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.CatalogTableOf(page, nodes))
}
