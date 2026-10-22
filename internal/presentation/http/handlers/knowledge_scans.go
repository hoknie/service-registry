package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListKnowledgeScans(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Scans
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Knowledge.Scans.ListScans(c.Context(), p, q.Filter(), q.Page.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.ScanOf))
}

func (a *Handlers) ListProjectScans(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Scans
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Knowledge.Scans.ListProjectScans(c.Context(), p, id, q.Filter(), q.Page.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.ScanOf))
}
