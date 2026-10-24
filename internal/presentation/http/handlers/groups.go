package handlers

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Page
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	org, err := organizationOf(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	list, err := a.Access.Groups.ListGroups(c.Context(), p, accesstypes.GroupQuery{Page: q.Query(), Organization: org})
	if err != nil {
		return responses.Fail(c, err)
	}
	if org != nil {
		return c.Status(http.StatusOK).JSON(responses.OrganizationGroupsOf(list))
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(list.Page, responses.GroupOf))
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
	org, err := organizationOf(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.Groups.CreateGroup(c.Context(), p, in.Group(org))
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
	groups, err := a.Access.Groups.UserGroups(c.Context(), p, id)
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
	at, err := groupAt(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.Groups.GetGroup(c.Context(), p, at)
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
	at, err := groupAt(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.Groups.RenameGroup(c.Context(), p, at, in.Group(nil))
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
	at, err := groupAt(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.Groups.DeleteGroup(c.Context(), p, at); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) AddMember(c fiber.Ctx) error {
	return a.member(c, a.Access.Groups.AddMember)
}

func (a *Handlers) RemoveMember(c fiber.Ctx) error {
	return a.member(c, a.Access.Groups.RemoveMember)
}

func (a *Handlers) member(c fiber.Ctx, op func(ctx context.Context, p accesstypes.Principal, at accesstypes.GroupAt, userID uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	at, err := groupAt(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	userID, err := parseID(c, "user_id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := op(c.Context(), p, at, userID); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) SetGroupPermissions(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GroupPermissions
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	rights, err := a.Access.Groups.SetGroupPermissions(c.Context(), p, id, *in.Permissions)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.GroupPermissions{Permissions: responses.RightsOf(rights)})
}

func organizationOf(c fiber.Ctx) (*uuid.UUID, error) {
	if c.Params("id") == "" {
		return nil, nil
	}
	id, err := parseID(c, "id")
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func groupAt(c fiber.Ctx) (accesstypes.GroupAt, error) {
	if c.Params("group_id") == "" {
		id, err := parseID(c, "id")
		return accesstypes.GroupAt{ID: id}, err
	}
	org, err := parseID(c, "id")
	if err != nil {
		return accesstypes.GroupAt{}, err
	}
	id, err := parseID(c, "group_id")
	return accesstypes.GroupAt{ID: id, Organization: &org}, err
}
