package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListLinkKinds(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.ListLinkKinds(c.Context(), a.State, p)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(items, response.LinkKindOf))
}

func (a *API) CreateLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateLinkKind
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	k, err := service.CreateLinkKind(c.Context(), a.State, p, in.Kind())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.LinkKindOf(k))
}

func (a *API) UpdateLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.UpdateLinkKind
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	k, err := service.UpdateLinkKind(c.Context(), a.State, p, c.Params("key"), in.Kind())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.LinkKindOf(k))
}

func (a *API) DeleteLinkKind(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	if err := service.DeleteLinkKind(c.Context(), a.State, p, c.Params("key")); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
