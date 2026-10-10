package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) OwnIdentities(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Access.OwnIdentities(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.IdentitiesOf(items))
}

func (a *Handlers) UnlinkIdentity(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.UnlinkIdentity(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) UserIdentities(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Access.UserIdentities(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.IdentitiesOf(items))
}
