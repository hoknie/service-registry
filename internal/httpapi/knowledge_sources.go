package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) GetKnowledgeSource(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		s, err := service.GetKnowledgeSource(c.Context(), a.State, p, id)
		return response.KnowledgeSourceOf(s), err
	})
}

func (a *API) sourceBody(c fiber.Ctx) (service.Principal, request.KnowledgeSource, uuid.UUID, error) {
	p, _, err := a.currentUser(c)
	if err != nil {
		return service.Principal{}, request.KnowledgeSource{}, uuid.UUID{}, err
	}
	var in request.KnowledgeSource
	if err := bindJSON(c, &in); err != nil {
		return service.Principal{}, request.KnowledgeSource{}, uuid.UUID{}, err
	}
	id, err := parseID(c, "id")
	return p, in, id, err
}

func (a *API) PutKnowledgeSource(c fiber.Ctx) error {
	p, in, id, err := a.sourceBody(c)
	if err != nil {
		return Fail(c, err)
	}
	s, err := service.PutKnowledgeSource(c.Context(), a.State, p, id, in.Input())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.KnowledgeSourceOf(&s))
}

func (a *API) DeleteKnowledgeSource(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.DeleteKnowledgeSource(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *API) CheckKnowledgeSource(c fiber.Ctx) error {
	p, in, id, err := a.sourceBody(c)
	if err != nil {
		return Fail(c, err)
	}
	res, err := service.CheckKnowledgeSource(c.Context(), a.State, p, id, in.Input())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.SourceCheckOf(res))
}
