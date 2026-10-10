package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) GetKnowledgeSource(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		s, err := a.Knowledge.GetKnowledgeSource(c.Context(), p, id)
		return responses.KnowledgeSourceOf(s), err
	})
}

func (a *Handlers) sourceBody(c fiber.Ctx) (access.Principal, requests.KnowledgeSource, uuid.UUID, error) {
	p, _, err := a.currentUser(c)
	if err != nil {
		return access.Principal{}, requests.KnowledgeSource{}, uuid.UUID{}, err
	}
	var in requests.KnowledgeSource
	if err := bindJSON(c, &in); err != nil {
		return access.Principal{}, requests.KnowledgeSource{}, uuid.UUID{}, err
	}
	id, err := parseID(c, "id")
	return p, in, id, err
}

func (a *Handlers) PutKnowledgeSource(c fiber.Ctx) error {
	p, in, id, err := a.sourceBody(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	s, err := a.Knowledge.PutKnowledgeSource(c.Context(), p, id, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.KnowledgeSourceOf(&s))
}

func (a *Handlers) DeleteKnowledgeSource(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Knowledge.DeleteKnowledgeSource(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) CheckKnowledgeSource(c fiber.Ctx) error {
	p, in, id, err := a.sourceBody(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	res, err := a.Knowledge.CheckKnowledgeSource(c.Context(), p, id, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SourceCheckOf(res))
}
