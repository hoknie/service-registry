package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/knowledge"
	"svc-registry/internal/service"
)

func (a *API) SearchKnowledge(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	in := knowledge.SearchInput{Q: c.Query("q"), Branch: optionalQuery(c, "branch"), Path: c.Query("path"), Cursor: c.Query("cursor"),
		Mode: c.Query("mode")}
	if v := c.Query("project"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return Fail(c, apperr.Wrap(knowledge.InvalidSearch))
		}
		in.Project = &id
	}
	if v := optionalQuery(c, "limit"); v != nil {
		n, err := strconv.Atoi(*v)
		if err != nil {
			return Fail(c, apperr.Wrap(knowledge.InvalidSearch))
		}
		in.Limit = &n
	}
	res, err := service.SearchKnowledge(c.Context(), a.State, p, in)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.SearchOf(res))
}

func (a *API) KnowledgeSearchModes(c fiber.Ctx) error {
	if _, _, err := a.currentUser(c); err != nil {
		return Fail(c, err)
	}
	m, err := service.KnowledgeSearchModes(c.Context(), a.State)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.SearchModesOf(m))
}
