package httpapi

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Page
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	page, err := service.ListGroups(c.Context(), a.State, p, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.GroupOf))
}

func (a *API) CreateGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.GroupName
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	g, err := service.CreateGroup(c.Context(), a.State, p, in.Group())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.GroupOf(g))
}

func (a *API) UserGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	groups, err := service.UserGroups(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(groups, response.GroupOf))
}

func (a *API) GetGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	g, err := service.GetGroup(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.GroupDetailsOf(g))
}

func (a *API) RenameGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.GroupName
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	g, err := service.RenameGroup(c.Context(), a.State, p, id, in.Group())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.GroupOf(g))
}

func (a *API) DeleteGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.DeleteGroup(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *API) AddMember(c fiber.Ctx) error {
	return a.member(c, service.AddMember)
}

func (a *API) RemoveMember(c fiber.Ctx) error {
	return a.member(c, service.RemoveMember)
}

func (a *API) member(c fiber.Ctx, op func(ctx context.Context, state *service.State, p service.Principal, groupID, userID uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	groupID, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	userID, err := parseID(c, "user_id")
	if err != nil {
		return Fail(c, err)
	}
	if err := op(c.Context(), a.State, p, groupID, userID); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
