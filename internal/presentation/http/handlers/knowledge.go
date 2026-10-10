package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) knowledgeCall(c fiber.Ctx, fn func(access.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(body)
}

func optionalQuery(c fiber.Ctx, name string) *string {
	if !c.Request().URI().QueryArgs().Has(name) {
		return nil
	}
	v := c.Query(name)
	return &v
}

func knowledgeRef(c fiber.Ctx) knowledgeservice.KnowledgeRef {
	return knowledgeservice.KnowledgeRef{Branch: optionalQuery(c, "branch"), Commit: optionalQuery(c, "commit")}
}

func (a *Handlers) GetKnowledge(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		o, err := a.Knowledge.GetKnowledge(c.Context(), p, id)
		return responses.KnowledgeOf(o), err
	})
}

func (a *Handlers) GetKnowledgeSettings(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		s, err := a.Knowledge.GetKnowledgeSettings(c.Context(), p, id)
		return responses.NodeSettingsOf(s), err
	})
}

func (a *Handlers) PutKnowledgeSettings(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.KnowledgeSettings
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	own, err := in.Settings()
	if err != nil {
		return responses.Fail(c, apperr.Wrap(err))
	}
	s, err := a.Knowledge.PutKnowledgeSettings(c.Context(), p, id, own)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.NodeSettingsOf(s))
}

func (a *Handlers) CollectKnowledge(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Knowledge.CollectKnowledge(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}

func (a *Handlers) KnowledgeFiles(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		s, files, err := a.Knowledge.KnowledgeFiles(c.Context(), p, id, knowledgeRef(c))
		return responses.KnowledgeFilesOf(s, files), err
	})
}

func (a *Handlers) KnowledgeFile(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		s, f, err := a.Knowledge.KnowledgeFile(c.Context(), p, id, knowledgeRef(c), c.Query("path"))
		return responses.KnowledgeFileContentOf(s, f), err
	})
}

func (a *Handlers) KnowledgeSpecs(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		o, err := a.Knowledge.KnowledgeOpenSpec(c.Context(), p, id, optionalQuery(c, "branch"))
		return responses.SpecsOf(o), err
	})
}

func (a *Handlers) KnowledgeChanges(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		o, err := a.Knowledge.KnowledgeOpenSpec(c.Context(), p, id, optionalQuery(c, "branch"))
		return responses.ChangesOf(o), err
	})
}

func (a *Handlers) KnowledgeADRs(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p access.Principal, id uuid.UUID) (any, error) {
		o, err := a.Knowledge.KnowledgeOpenSpec(c.Context(), p, id, optionalQuery(c, "branch"))
		return responses.ADRsOf(o), err
	})
}
