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

func (a *API) ListOwnTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	tokens, err := service.ListOwnTokens(c.Context(), a.State, p)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(tokens, response.TokenOf))
}

func (a *API) IssueToken(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateToken
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	issued, err := service.IssueToken(c.Context(), a.State, p, in.Token())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.IssuedTokenOf(issued))
}

func (a *API) RevokeOwnToken(c fiber.Ctx) error {
	return a.revokeToken(c, service.RevokeOwnToken)
}

func (a *API) ListUserTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	tokens, err := service.ListUserTokens(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(tokens, response.OwnedTokenOf))
}

func (a *API) ListTokens(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Tokens
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	page, err := service.ListTokens(c.Context(), a.State, p, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.PageOf(page, response.OwnedTokenOf))
}

func (a *API) RevokeToken(c fiber.Ctx) error {
	return a.revokeToken(c, service.RevokeToken)
}

func (a *API) revokeToken(c fiber.Ctx, revoke func(context.Context, *service.State, service.Principal, uuid.UUID) error) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := revoke(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
