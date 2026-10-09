package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/access"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListKnowledgeScans(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Scans
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	page, err := service.ListScans(c.Context(), a.State, p, q.Filter(), q.Page.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.ScanOf))
}
