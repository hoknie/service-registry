package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) Login(c fiber.Ctx) error {
	var in request.Login
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	var previous *string
	if token := c.Cookies(SessionCookie); token != "" {
		previous = &token
	}
	signedIn, err := service.Login(c.Context(), a.State, c.IP(), in.Credentials(), previous)
	if err != nil {
		return Fail(c, err)
	}
	session := a.State.Config.Session
	setSessionCookie(c, signedIn.Token, session.AbsoluteTimeoutSecs, session.CookieSecure)
	return c.Status(http.StatusOK).JSON(response.UserOf(signedIn.User))
}

func (a *API) Logout(c fiber.Ctx) error {
	if _, ok := authorization(c); ok {
		if _, _, err := a.currentUser(c); err != nil {
			return Fail(c, err)
		}
	}
	var token *string
	if t := c.Cookies(SessionCookie); t != "" {
		token = &t
	}
	if err := service.Logout(c.Context(), a.State, token); err != nil {
		return Fail(c, err)
	}
	setSessionCookie(c, "", 0, a.State.Config.Session.CookieSecure)
	return c.SendStatus(http.StatusNoContent)
}

func (a *API) Me(c fiber.Ctx) error {
	p, user, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.Me{User: response.UserOf(user), LoginMethod: p.Method})
}

func (a *API) ChangePassword(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.ChangePassword
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	if err := service.ChangePassword(c.Context(), a.State, p, in.Change()); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
