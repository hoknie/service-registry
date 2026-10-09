package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

const OAuthCookie = "registry_oauth"

const oauthCookiePath = "/api/v1/auth/oauth"

func setOAuthCookie(c fiber.Ctx, value string, maxAgeSecs int, secure bool) {
	cookie := http.Cookie{Name: OAuthCookie, Value: value, Path: oauthCookiePath, HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: maxAgeSecs}
	if maxAgeSecs == 0 {
		cookie.MaxAge = -1
	}
	c.Response().Header.Add(fiber.HeaderSetCookie, cookie.String())
}

func (a *API) Providers(c fiber.Ctx) error {
	providers, mode := service.Providers(a.State)
	return c.Status(http.StatusOK).JSON(response.ProvidersOf(providers, mode))
}

func (a *API) OAuthStart(c fiber.Ctx) error {
	var link *uuid.UUID
	if c.Query("link") == "1" {
		token := c.Cookies(SessionCookie)
		if token == "" {
			return Fail(c, apperr.New(apperr.Unauthenticated))
		}
		p, _, err := service.Authenticate(c.Context(), a.State, token)
		if err != nil {
			return Fail(c, err)
		}
		link = &p.UserID
	}
	start, err := service.StartLogin(c.Context(), a.State, c.Params("provider"), c.Query("next"), link)
	if err != nil {
		return Fail(c, err)
	}
	setOAuthCookie(c, start.BrowserID, service.LoginStateTTL, a.State.Config.Session.CookieSecure)
	return c.Redirect().Status(http.StatusFound).To(start.URL)
}

func (a *API) OAuthCallback(c fiber.Ctx) error {
	var previous *string
	if token := c.Cookies(SessionCookie); token != "" {
		previous = &token
	}
	cb := service.LoginCallback{Provider: c.Params("provider"), BrowserID: c.Cookies(OAuthCookie), State: c.Query("state"),
		Code: c.Query("code"), Error: c.Query("error")}
	done, err := service.CompleteLogin(c.Context(), a.State, cb, previous)
	secure := a.State.Config.Session.CookieSecure
	setOAuthCookie(c, "", 0, secure)
	var refusal access.Refusal
	switch {
	case errors.As(err, &refusal):
		target := "/login?error=" + url.QueryEscape(string(refusal))
		if done.Link {
			target = "/account?tab=login&error=" + url.QueryEscape(string(refusal))
		}
		return c.Redirect().Status(http.StatusFound).To(target)
	case err != nil:
		return Fail(c, err)
	}
	if done.Token != "" {
		session := a.State.Config.Session
		setSessionCookie(c, done.Token, session.AbsoluteTimeoutSecs, secure)
	}
	return c.Redirect().Status(http.StatusFound).To(done.Next)
}
