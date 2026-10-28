package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	membershipservices "svc-registry/internal/feature/membership/services"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListOrgMembers(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Membership.Members.ListMembers(c.Context(), p, id, membershipservices.MemberQuery{Q: c.Query("q"),
			Limit: fiber.Query[int](c, "limit"), Cursor: c.Query("cursor")})
		return responses.OrgMembersOf(out), err
	})
}

func (a *Handlers) OrgMembersViaGroups(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Membership.Members.GroupsWithAccess(c.Context(), p, id)
		return responses.GroupsAccessOf(out), err
	})
}

func (a *Handlers) OrgMemberCandidates(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Membership.Members.Candidates(c.Context(), p, id, c.Query("q"))
		return responses.MemberCandidatesOf(out), err
	})
}

func (a *Handlers) AddOrgMember(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.MemberAdd
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	out, created, err := a.Membership.Members.AddMember(c.Context(), p, id, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.Status(status).JSON(responses.OrgMemberOf(out))
}

func (a *Handlers) SetOrgMemberRole(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		user, err := parseID(c, "user_id")
		if err != nil {
			return nil, err
		}
		var in requests.MemberRole
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Membership.Members.SetMemberRole(c.Context(), p, id, user, in.Role.Value)
		return responses.OrgMemberOf(out), err
	})
}

func (a *Handlers) RemoveOrgMember(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		user, err := parseID(c, "user_id")
		if err != nil {
			return nil, err
		}
		return nil, a.Membership.Members.RemoveMember(c.Context(), p, id, user)
	})
}
