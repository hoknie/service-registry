package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListLinkTemplates(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		items, err := a.Links.Templates.ListLinkTemplates(c.Context(), p, id)
		return responses.ItemsOf(items, responses.LinkTemplateOf), err
	})
}

func (a *Handlers) PutLinkTemplate(c fiber.Ctx) error {
	var in requests.PutLinkTemplate
	return a.nodeCall(c, &in, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		t, err := a.Links.Templates.PutLinkTemplate(c.Context(), p, id, c.Params("link_key"), in.Put())
		return responses.LinkTemplateOf(t), err
	})
}

func (a *Handlers) DeleteLinkTemplate(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Links.Templates.DeleteLinkTemplate(c.Context(), p, id, c.Params("link_key"))
	})
}

func (a *Handlers) PreviewLinkTemplate(c fiber.Ctx) error {
	var in requests.PreviewLinkTemplate
	return a.nodeCall(c, &in, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		items, err := a.Links.Templates.PreviewLinkTemplate(c.Context(), p, id, in.Preview())
		return responses.ItemsOf(items, responses.LinkOf), err
	})
}

func (a *Handlers) ListNodeVars(c fiber.Ctx) error {
	return a.nodeCall(c, nil, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		items, err := a.Links.Templates.ListNodeVars(c.Context(), p, id)
		return responses.ItemsOf(items, responses.NodeVarOf), err
	})
}

func (a *Handlers) PutNodeVars(c fiber.Ctx) error {
	var in requests.PutNodeVars
	return a.nodeCall(c, &in, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		items, err := a.Links.Templates.ReplaceNodeVars(c.Context(), p, id, *in.Vars)
		return responses.ItemsOf(items, responses.NodeVarOf), err
	})
}

func (a *Handlers) ProjectLinks(c fiber.Ctx) error {
	return a.linksCall(c, func(p accesstypes.Principal, id uuid.UUID, q requests.Links) (any, error) {
		items, err := a.Links.Projects.ProjectLinks(c.Context(), p, id, q.Query())
		return responses.ItemsOf(items, responses.LinkOf), err
	})
}

func (a *Handlers) CheckLink(c fiber.Ctx) error {
	return a.linksCall(c, func(p accesstypes.Principal, id uuid.UUID, q requests.Links) (any, error) {
		items, err := a.Links.Projects.CheckLink(c.Context(), p, id, c.Params("link_key"), q.Query())
		return responses.ItemsOf(items, responses.LinkOf), err
	})
}

func (a *Handlers) LinkChecks(c fiber.Ctx) error {
	return a.linksCall(c, func(p accesstypes.Principal, id uuid.UUID, q requests.Links) (any, error) {
		items, err := a.Links.Projects.LinkChecks(c.Context(), p, id, c.Params("link_key"), q.Query())
		return responses.ItemsOf(items, responses.LinkCheckEntryOf), err
	})
}

func (a *Handlers) nodeCall(c fiber.Ctx, in any, fn func(accesstypes.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if in != nil {
		if err := bindJSON(c, in); err != nil {
			return responses.Fail(c, err)
		}
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}

func (a *Handlers) linksCall(c fiber.Ctx, fn func(accesstypes.Principal, uuid.UUID, requests.Links) (any, error)) error {
	var q requests.Links
	if err := c.Bind().Query(&q); err != nil {
		return responses.Fail(c, InvalidBody(describe(err)))
	}
	return a.nodeCall(c, nil, func(p accesstypes.Principal, id uuid.UUID) (any, error) { return fn(p, id, q) })
}
