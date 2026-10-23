package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) OAuthRequest(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	req, err := a.Access.AuthorizationServer.Request(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ConsentRequestOf(req))
}

func (a *Handlers) ApproveOAuthRequest(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.ApproveOAuth
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	to, err := a.Access.AuthorizationServer.Approve(c.Context(), p, id, in.Nodes)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.RedirectTo{RedirectTo: to})
}

func (a *Handlers) DenyOAuthRequest(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	to, err := a.Access.AuthorizationServer.Deny(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.RedirectTo{RedirectTo: to})
}

func (a *Handlers) ListOwnApps(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	apps, err := a.Access.Apps.ListOwnApps(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(apps, responses.AppOf))
}

func (a *Handlers) RevokeOwnApp(c fiber.Ctx) error {
	return a.revokeToken(c, a.Access.Apps.RevokeOwnApp)
}

func (a *Handlers) ListUserApps(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	apps, err := a.Access.Apps.ListUserApps(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(apps, responses.AppOf))
}

func (a *Handlers) RevokeUserApp(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	userID, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "gid")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.Apps.RevokeUserApp(c.Context(), p, userID, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
