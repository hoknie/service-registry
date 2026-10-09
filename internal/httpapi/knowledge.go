package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) knowledgeCall(c fiber.Ctx, fn func(service.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return Fail(c, err)
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

func knowledgeRef(c fiber.Ctx) service.KnowledgeRef {
	return service.KnowledgeRef{Branch: optionalQuery(c, "branch"), Commit: optionalQuery(c, "commit")}
}

func (a *API) GetKnowledge(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		o, err := service.GetKnowledge(c.Context(), a.State, p, id)
		return response.KnowledgeOf(o), err
	})
}

func (a *API) GetKnowledgeSettings(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		s, err := service.GetKnowledgeSettings(c.Context(), a.State, p, id)
		return response.NodeSettingsOf(s), err
	})
}

func (a *API) PutKnowledgeSettings(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.KnowledgeSettings
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	own, err := in.Settings()
	if err != nil {
		return Fail(c, apperr.Wrap(err))
	}
	s, err := service.PutKnowledgeSettings(c.Context(), a.State, p, id, own)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.NodeSettingsOf(s))
}

func (a *API) CollectKnowledge(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.CollectKnowledge(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}

func (a *API) KnowledgeFiles(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		s, files, err := service.KnowledgeFiles(c.Context(), a.State, p, id, knowledgeRef(c))
		return response.KnowledgeFilesOf(s, files), err
	})
}

func (a *API) KnowledgeFile(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		s, f, err := service.KnowledgeFile(c.Context(), a.State, p, id, knowledgeRef(c), c.Query("path"))
		return response.KnowledgeFileContentOf(s, f), err
	})
}

func (a *API) KnowledgeSpecs(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		o, err := service.KnowledgeOpenSpec(c.Context(), a.State, p, id, optionalQuery(c, "branch"))
		return response.SpecsOf(o), err
	})
}

func (a *API) KnowledgeChanges(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		o, err := service.KnowledgeOpenSpec(c.Context(), a.State, p, id, optionalQuery(c, "branch"))
		return response.ChangesOf(o), err
	})
}

func (a *API) KnowledgeADRs(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p service.Principal, id uuid.UUID) (any, error) {
		o, err := service.KnowledgeOpenSpec(c.Context(), a.State, p, id, optionalQuery(c, "branch"))
		return response.ADRsOf(o), err
	})
}
