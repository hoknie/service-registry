package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListLinkTemplates(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		items, err := service.ListLinkTemplates(c.Context(), a.State, p, id)
		return response.ItemsOf(items, response.LinkTemplateOf), err
	})
}

func (a *API) PutLinkTemplate(c fiber.Ctx) error {
	var in request.PutLinkTemplate
	return a.nodeCall(c, &in, func(p service.Principal, id uuid.UUID) (any, error) {
		t, err := service.PutLinkTemplate(c.Context(), a.State, p, id, c.Params("link_key"), in.Put())
		return response.LinkTemplateOf(t), err
	})
}

func (a *API) DeleteLinkTemplate(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		return nil, service.DeleteLinkTemplate(c.Context(), a.State, p, id, c.Params("link_key"))
	})
}

func (a *API) PreviewLinkTemplate(c fiber.Ctx) error {
	var in request.PreviewLinkTemplate
	return a.nodeCall(c, &in, func(p service.Principal, id uuid.UUID) (any, error) {
		items, err := service.PreviewLinkTemplate(c.Context(), a.State, p, id, in.Preview())
		return response.ItemsOf(items, response.LinkOf), err
	})
}

func (a *API) ListNodeVars(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		items, err := service.ListNodeVars(c.Context(), a.State, p, id)
		return response.ItemsOf(items, response.NodeVarOf), err
	})
}

func (a *API) PutNodeVars(c fiber.Ctx) error {
	var in request.PutNodeVars
	return a.nodeCall(c, &in, func(p service.Principal, id uuid.UUID) (any, error) {
		items, err := service.ReplaceNodeVars(c.Context(), a.State, p, id, *in.Vars)
		return response.ItemsOf(items, response.NodeVarOf), err
	})
}

func (a *API) ProjectLinks(c fiber.Ctx) error {
	return a.linksCall(c, func(p service.Principal, id uuid.UUID, q request.Links) (any, error) {
		items, err := service.ProjectLinks(c.Context(), a.State, p, id, q.Query())
		return response.ItemsOf(items, response.LinkOf), err
	})
}

func (a *API) CheckLink(c fiber.Ctx) error {
	return a.linksCall(c, func(p service.Principal, id uuid.UUID, q request.Links) (any, error) {
		items, err := service.CheckLink(c.Context(), a.State, p, id, c.Params("link_key"), q.Query())
		return response.ItemsOf(items, response.LinkOf), err
	})
}

func (a *API) LinkChecks(c fiber.Ctx) error {
	return a.linksCall(c, func(p service.Principal, id uuid.UUID, q request.Links) (any, error) {
		items, err := service.LinkChecks(c.Context(), a.State, p, id, c.Params("link_key"), q.Query())
		return response.ItemsOf(items, response.LinkCheckEntryOf), err
	})
}

func (a *API) nodeCall(c fiber.Ctx, in any, fn func(service.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	if in != nil {
		if err := bindJSON(c, in); err != nil {
			return Fail(c, err)
		}
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}

func (a *API) linksCall(c fiber.Ctx, fn func(service.Principal, uuid.UUID, request.Links) (any, error)) error {
	var q request.Links
	if err := c.Bind().Query(&q); err != nil {
		return Fail(c, InvalidBody(describe(err)))
	}
	return a.nodeCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) { return fn(p, id, q) })
}
