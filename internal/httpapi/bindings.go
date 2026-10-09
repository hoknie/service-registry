package httpapi

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/catalog"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListBindings(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	found, err := service.ListBindings(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(found, func(b catalog.Binding) response.Binding {
		return response.BindingOf(b, id)
	}))
}

func (a *API) GrantRole(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.GrantRole
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	b, err := service.GrantRole(c.Context(), a.State, p, id, in.Grant())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.BindingOf(b, id))
}

func (a *API) RevokeBinding(c fiber.Ctx) error {
	return a.nodeChild(c, "binding_id", service.RevokeBinding)
}

func (a *API) nodeChild(c fiber.Ctx, child string, op func(context.Context, *service.State, service.Principal, uuid.UUID, uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	childID, err := parseID(c, child)
	if err != nil {
		return Fail(c, err)
	}
	if err := op(c.Context(), a.State, p, id, childID); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
