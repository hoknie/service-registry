package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListLinkKinds(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Links.ListLinkKinds(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.LinkKindOf))
}

func (a *Handlers) CreateLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateLinkKind
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	k, err := a.Links.CreateLinkKind(c.Context(), p, in.Kind())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.LinkKindOf(k))
}

func (a *Handlers) UpdateLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.UpdateLinkKind
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	k, err := a.Links.UpdateLinkKind(c.Context(), p, c.Params("key"), in.Kind())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.LinkKindOf(k))
}

func (a *Handlers) DeleteLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Links.DeleteLinkKind(c.Context(), p, c.Params("key")); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
