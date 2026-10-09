package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/catalog"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListBranches(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Branches
	if err := bindQuery(c, &q, catalog.InvalidBranchPage, nil); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	items, f, total, err := service.ListBranches(c.Context(), a.State, p, id, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.BranchPage(items, f, total))
}

func (a *API) GetBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p service.Principal, id uuid.UUID, name string) (any, error) {
		b, err := service.GetBranch(c.Context(), a.State, p, id, name)
		return response.BranchOf(b), err
	})
}

func (a *API) PinBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p service.Principal, id uuid.UUID, name string) (any, error) {
		b, err := service.PinBranch(c.Context(), a.State, p, id, name, true)
		return response.BranchOf(b), err
	})
}

func (a *API) UnpinBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p service.Principal, id uuid.UUID, name string) (any, error) {
		b, err := service.PinBranch(c.Context(), a.State, p, id, name, false)
		return response.BranchOf(b), err
	})
}

func (a *API) DeleteBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p service.Principal, id uuid.UUID, name string) (any, error) {
		return nil, service.DeleteBranch(c.Context(), a.State, p, id, name)
	})
}

func (a *API) branchCall(c fiber.Ctx, fn func(service.Principal, uuid.UUID, string) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	body, err := fn(p, id, c.Query("name"))
	if err != nil {
		return Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}
