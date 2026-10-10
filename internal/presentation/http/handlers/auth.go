package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) Login(c fiber.Ctx) error {
	var in requests.Login
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	var previous *string
	if token := c.Cookies(SessionCookie); token != "" {
		previous = &token
	}
	signedIn, err := a.Access.Login(c.Context(), c.IP(), in.Credentials(), previous)
	if err != nil {
		return responses.Fail(c, err)
	}
	session := a.Config.Session
	setSessionCookie(c, signedIn.Token, session.AbsoluteTimeoutSecs, session.CookieSecure)
	return c.Status(http.StatusOK).JSON(responses.UserOf(signedIn.User))
}

func (a *Handlers) Logout(c fiber.Ctx) error {
	if _, ok := authorization(c); ok {
		if _, _, err := a.currentUser(c); err != nil {
			return responses.Fail(c, err)
		}
	}
	var token *string
	if t := c.Cookies(SessionCookie); t != "" {
		token = &t
	}
	if err := a.Access.Logout(c.Context(), token); err != nil {
		return responses.Fail(c, err)
	}
	setSessionCookie(c, "", 0, a.Config.Session.CookieSecure)
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) Me(c fiber.Ctx) error {
	p, user, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.Me{User: responses.UserOf(user), LoginMethod: p.Method})
}

func (a *Handlers) ChangePassword(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.ChangePassword
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Access.ChangePassword(c.Context(), p, in.Change()); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
