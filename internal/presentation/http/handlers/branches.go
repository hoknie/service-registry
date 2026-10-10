package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListBranches(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Branches
	if err := bindQuery(c, &q, catalog.InvalidBranchPage, nil); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	items, f, total, err := a.Catalog.ListBranches(c.Context(), p, id, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.BranchPage(items, f, total))
}

func (a *Handlers) GetBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p access.Principal, id uuid.UUID, name string) (any, error) {
		b, err := a.Catalog.GetBranch(c.Context(), p, id, name)
		return responses.BranchOf(b), err
	})
}

func (a *Handlers) PinBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p access.Principal, id uuid.UUID, name string) (any, error) {
		b, err := a.Catalog.PinBranch(c.Context(), p, id, name, true)
		return responses.BranchOf(b), err
	})
}

func (a *Handlers) UnpinBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p access.Principal, id uuid.UUID, name string) (any, error) {
		b, err := a.Catalog.PinBranch(c.Context(), p, id, name, false)
		return responses.BranchOf(b), err
	})
}

func (a *Handlers) DeleteBranch(c fiber.Ctx) error {
	return a.branchCall(c, func(p access.Principal, id uuid.UUID, name string) (any, error) {
		return nil, a.Catalog.DeleteBranch(c.Context(), p, id, name)
	})
}

func (a *Handlers) branchCall(c fiber.Ctx, fn func(access.Principal, uuid.UUID, string) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	body, err := fn(p, id, c.Query("name"))
	if err != nil {
		return responses.Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}
