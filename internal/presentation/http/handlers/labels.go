package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) CatalogLabels(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Labels
	if err := bindQuery(c, &q, catalog.InvalidFilter, nil); err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Catalog.SuggestLabels(c.Context(), p, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.LabelSuggestionsOf(items, q.Key != nil))
}
