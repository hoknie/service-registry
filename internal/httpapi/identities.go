package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) OwnIdentities(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.OwnIdentities(c.Context(), a.State, p)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.IdentitiesOf(items))
}

func (a *API) UnlinkIdentity(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.UnlinkIdentity(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *API) UserIdentities(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.UserIdentities(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.IdentitiesOf(items))
}
