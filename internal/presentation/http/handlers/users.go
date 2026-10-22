package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListUsers(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Page
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Access.Users.ListUsers(c.Context(), p, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.UserOf))
}

func (a *Handlers) CreateUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateUser
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	user, err := a.Access.Users.CreateUser(c.Context(), p, in.User())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.UserOf(user))
}

func (a *Handlers) GetUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	user, err := a.Access.Users.GetUser(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.UserOf(user))
}

func (a *Handlers) UpdateUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.UpdateUser
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	user, err := a.Access.Users.UpdateUser(c.Context(), p, id, in.Changes())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.UserOf(user))
}

func (a *Handlers) ResetPassword(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.ResetPassword
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.Users.ResetPassword(c.Context(), p, id, *in.Password); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
