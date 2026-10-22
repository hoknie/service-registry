package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListEnvironmentsDirectory(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Deploy.Environments.ListEnvironmentDirectory(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.DirectoryEnvironmentOf))
}

func (a *Handlers) CreateEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateEnvironment
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	e, err := a.Deploy.Environments.CreateEnvironment(c.Context(), p, in.Environment())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.DirectoryEnvironmentOf(e))
}

func (a *Handlers) UpdateEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.UpdateEnvironment
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	e, err := a.Deploy.Environments.UpdateEnvironment(c.Context(), p, c.Params("key"), in.Environment())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.DirectoryEnvironmentOf(e))
}

func (a *Handlers) DeleteEnvironment(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Deploy.Environments.DeleteEnvironment(c.Context(), p, c.Params("key")); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
