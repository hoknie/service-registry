package handlers

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListBindings(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	found, err := a.Catalog.Bindings.ListBindings(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(found, func(b catalogtypes.Binding) responses.Binding {
		return responses.BindingOf(b, id)
	}))
}

func (a *Handlers) GrantRole(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GrantRole
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	b, err := a.Catalog.Bindings.GrantRole(c.Context(), p, id, in.Grant())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.BindingOf(b, id))
}

func (a *Handlers) RevokeBinding(c fiber.Ctx) error {
	return a.nodeChild(c, "binding_id", a.Catalog.Bindings.RevokeBinding)
}

func (a *Handlers) nodeChild(c fiber.Ctx, child string, op func(context.Context, accesstypes.Principal, uuid.UUID, uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	childID, err := parseID(c, child)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := op(c.Context(), p, id, childID); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
