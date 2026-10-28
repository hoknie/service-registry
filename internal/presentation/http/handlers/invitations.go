package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListInvitations(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Membership.Invitations.ListInvitations(c.Context(), p, id)
		return responses.InvitationsOf(out), err
	})
}

func (a *Handlers) CreateInvitation(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusCreated, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.InvitationCreate
		if err := bindJSON(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Membership.Invitations.CreateInvitation(c.Context(), p, id, in.Input())
		return responses.IssuedOf(out), err
	})
}

func (a *Handlers) ResendInvitation(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusCreated, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		iid, err := parseID(c, "iid")
		if err != nil {
			return nil, err
		}
		var in requests.InvitationResend
		if len(c.Body()) > 0 {
			if err := bindJSON(c, &in); err != nil {
				return nil, err
			}
		}
		out, err := a.Membership.Invitations.ResendInvitation(c.Context(), p, id, iid, in.Locale)
		return responses.IssuedOf(out), err
	})
}

func (a *Handlers) RevokeInvitation(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		iid, err := parseID(c, "iid")
		if err != nil {
			return nil, err
		}
		return nil, a.Membership.Invitations.RevokeInvitation(c.Context(), p, id, iid)
	})
}

func (a *Handlers) ViewInvitation(c fiber.Ctx) error {
	out, err := a.Membership.Invitations.ViewInvitation(c.Context(), c.IP(), c.Params("token"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.InvitationViewOf(out))
}

func (a *Handlers) AcceptInvitation(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	out, err := a.Membership.Invitations.AcceptInvitation(c.Context(), p, c.Params("token"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.InvitationAcceptedOf(out))
}

func (a *Handlers) RegisterInvited(c fiber.Ctx) error {
	var in requests.InvitationRegister
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	var previous *string
	if token := c.Cookies(SessionCookie); token != "" {
		previous = &token
	}
	signedIn, err := a.Membership.Invitations.RegisterInvited(c.Context(), c.IP(), c.Params("token"), *in.Name, *in.Password, previous)
	if err != nil {
		return responses.Fail(c, err)
	}
	session := a.Config.Session
	setSessionCookie(c, signedIn.Token, session.AbsoluteTimeoutSecs, session.CookieSecure)
	return c.Status(http.StatusCreated).JSON(responses.UserOf(signedIn.User))
}
