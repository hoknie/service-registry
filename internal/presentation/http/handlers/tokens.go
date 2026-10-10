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

func (a *Handlers) ListOwnTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	tokens, err := a.Access.ListOwnTokens(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(tokens, responses.TokenOf))
}

func (a *Handlers) IssueToken(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateToken
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	issued, err := a.Access.IssueToken(c.Context(), p, in.Token())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.IssuedTokenOf(issued))
}

func (a *Handlers) RevokeOwnToken(c fiber.Ctx) error {
	return a.revokeToken(c, a.Access.RevokeOwnToken)
}

func (a *Handlers) IssueUserToken(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateToken
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	issued, err := a.Access.IssueUserToken(c.Context(), p, id, in.Token())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.IssuedTokenOf(issued))
}

func (a *Handlers) ListUserTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	tokens, err := a.Access.ListUserTokens(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(tokens, responses.OwnedTokenOf))
}

func (a *Handlers) ListTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Tokens
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Access.ListTokens(c.Context(), p, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.OwnedTokenOf))
}

func (a *Handlers) RevokeToken(c fiber.Ctx) error {
	return a.revokeToken(c, a.Access.RevokeToken)
}

func (a *Handlers) revokeToken(c fiber.Ctx, revoke func(context.Context, access.Principal, uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := revoke(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
