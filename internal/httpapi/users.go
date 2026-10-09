package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/access"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListUsers(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Page
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	page, err := service.ListUsers(c.Context(), a.State, p, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.UserOf))
}

func (a *API) CreateUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateUser
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	user, err := service.CreateUser(c.Context(), a.State, p, in.User())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.UserOf(user))
}

func (a *API) GetUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	user, err := service.GetUser(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.UserOf(user))
}

func (a *API) UpdateUser(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.UpdateUser
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	user, err := service.UpdateUser(c.Context(), a.State, p, id, in.Changes())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.UserOf(user))
}

func (a *API) ResetPassword(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.ResetPassword
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.ResetPassword(c.Context(), a.State, p, id, *in.Password); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
