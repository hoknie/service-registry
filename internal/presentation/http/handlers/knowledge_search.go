package handlers

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) SearchKnowledge(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	in := knowledgetypes.SearchInput{Q: c.Query("q"), Branch: optionalQuery(c, "branch"), Path: c.Query("path"), Cursor: c.Query("cursor"),
		Mode: c.Query("mode")}
	if v := c.Query("project"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return responses.Fail(c, apperr.Wrap(knowledgetypes.InvalidSearch))
		}
		in.Project = &id
	}
	if v := optionalQuery(c, "limit"); v != nil {
		n, err := strconv.Atoi(*v)
		if err != nil {
			return responses.Fail(c, apperr.Wrap(knowledgetypes.InvalidSearch))
		}
		in.Limit = &n
	}
	res, err := a.Knowledge.Search.SearchKnowledge(c.Context(), p, in)
	if a.Metrics != nil {
		mode := in.Mode
		if mode == "" {
			mode = string(a.Knowledge.Search.DefaultSearchMode())
		}
		a.Metrics.Search(mode, "api", err != nil)
	}
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SearchOf(res))
}

func (a *Handlers) KnowledgeSearchModes(c fiber.Ctx) error {
	if _, _, err := a.currentUser(c); err != nil {
		return responses.Fail(c, err)
	}
	m, err := a.Knowledge.Search.KnowledgeSearchModes(c.Context())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SearchModesOf(m))
}
