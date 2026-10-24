package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) SetGroupContacts(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GroupContacts
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.GroupContacts.SetContacts(c.Context(), p, id, in.GroupContacts())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.GroupOf(g))
}

func (a *Handlers) GrantContactEditor(c fiber.Ctx) error {
	return a.contactEditor(c, true)
}

func (a *Handlers) RevokeContactEditor(c fiber.Ctx) error {
	return a.contactEditor(c, false)
}

func (a *Handlers) contactEditor(c fiber.Ctx, on bool) error {
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
	if err := a.Access.GroupContacts.SetContactEditor(c.Context(), p, groupID, userID, on); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) OwnGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	groups, err := a.Access.GroupContacts.OwnGroups(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(groups, responses.OwnGroupOf))
}

func (a *Handlers) SetOwnGroupContacts(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.GroupContacts
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.GroupContacts.SetOwnContacts(c.Context(), p, id, in.GroupContacts())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.OwnGroupOf(g))
}

func (a *Handlers) ManagedGroups(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	groups, err := a.Access.Groups.ManagedGroups(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(groups, responses.ManagedGroupOf))
}

func (a *Handlers) ManagedGroup(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	g, err := a.Access.Groups.ManagedGroup(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ManagedGroupDetailsOf(g))
}

func (a *Handlers) MemberCandidates(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	at, err := groupAt(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	out, err := a.Access.Groups.MemberCandidates(c.Context(), p, at, c.Query("q"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(out, responses.MemberCandidateOf))
}
