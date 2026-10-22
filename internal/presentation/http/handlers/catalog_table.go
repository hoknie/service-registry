package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
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
	if err := bindQuery(c, &q, catalogtypes.InvalidFilter, map[string]error{
		"parent": apperr.New(apperr.NotFound), "limit": accesstypes.InvalidPagination, "offset": accesstypes.InvalidPagination,
	}); err != nil {
		return responses.Fail(c, err)
	}
	table, err := a.Compose.Catalog.CatalogTable(c.Context(), p, q.Parent, q.Query(), q.Filter())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.CatalogTableOf(table))
}
