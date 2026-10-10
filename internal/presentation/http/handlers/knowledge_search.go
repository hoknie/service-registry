package handlers

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) SearchKnowledge(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	in := knowledge.SearchInput{Q: c.Query("q"), Branch: optionalQuery(c, "branch"), Path: c.Query("path"), Cursor: c.Query("cursor"),
		Mode: c.Query("mode")}
	if v := c.Query("project"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return responses.Fail(c, apperr.Wrap(knowledge.InvalidSearch))
		}
		in.Project = &id
	}
	if v := optionalQuery(c, "limit"); v != nil {
		n, err := strconv.Atoi(*v)
		if err != nil {
			return responses.Fail(c, apperr.Wrap(knowledge.InvalidSearch))
		}
		in.Limit = &n
	}
	res, err := a.Knowledge.SearchKnowledge(c.Context(), p, in)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SearchOf(res))
}

func (a *Handlers) KnowledgeSearchModes(c fiber.Ctx) error {
	if _, _, err := a.currentUser(c); err != nil {
		return responses.Fail(c, err)
	}
	m, err := a.Knowledge.KnowledgeSearchModes(c.Context())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SearchModesOf(m))
}
