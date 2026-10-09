package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListEnvironmentsDirectory(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.ListEnvironmentDirectory(c.Context(), a.State, p)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(items, response.DirectoryEnvironmentOf))
}

func (a *API) CreateEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateEnvironment
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	e, err := service.CreateEnvironment(c.Context(), a.State, p, in.Environment())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.DirectoryEnvironmentOf(e))
}

func (a *API) UpdateEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.UpdateEnvironment
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	e, err := service.UpdateEnvironment(c.Context(), a.State, p, c.Params("key"), in.Environment())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.DirectoryEnvironmentOf(e))
}

func (a *API) DeleteEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	if err := service.DeleteEnvironment(c.Context(), a.State, p, c.Params("key")); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
