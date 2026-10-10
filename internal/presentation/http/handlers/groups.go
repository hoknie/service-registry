package handlers

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Page
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Access.ListGroups(c.Context(), p, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.GroupOf))
}

func (a *Handlers) CreateGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GroupName
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.CreateGroup(c.Context(), p, in.Group())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.GroupOf(g))
}

func (a *Handlers) UserGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	groups, err := a.Access.UserGroups(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(groups, responses.GroupOf))
}

func (a *Handlers) GetGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.GetGroup(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.GroupDetailsOf(g))
}

func (a *Handlers) RenameGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GroupName
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.RenameGroup(c.Context(), p, id, in.Group())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.GroupOf(g))
}

func (a *Handlers) DeleteGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.DeleteGroup(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) AddMember(c fiber.Ctx) error {
	return a.member(c, a.Access.AddMember)
}

func (a *Handlers) RemoveMember(c fiber.Ctx) error {
	return a.member(c, a.Access.RemoveMember)
}

func (a *Handlers) member(c fiber.Ctx, op func(ctx context.Context, p access.Principal, groupID, userID uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	groupID, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	userID, err := parseID(c, "user_id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := op(c.Context(), p, groupID, userID); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
