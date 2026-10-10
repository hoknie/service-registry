package handlers

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	accessservice "svc-registry/internal/feature/access/service"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
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

func (a *Handlers) Providers(c fiber.Ctx) error {
	providers, mode := a.Access.Providers()
	return c.Status(http.StatusOK).JSON(responses.ProvidersOf(providers, mode))
}

func (a *Handlers) OAuthStart(c fiber.Ctx) error {
	var link *uuid.UUID
	if c.Query("link") == "1" {
		token := c.Cookies(SessionCookie)
		if token == "" {
			return responses.Fail(c, apperr.New(apperr.Unauthenticated))
		}
		p, _, err := a.Access.Authenticate(c.Context(), token)
		if err != nil {
			return responses.Fail(c, err)
		}
		link = &p.UserID
	}
	start, err := a.Access.StartLogin(c.Context(), c.Params("provider"), c.Query("next"), link)
	if err != nil {
		return responses.Fail(c, err)
	}
	setOAuthCookie(c, start.BrowserID, accessservice.LoginStateTTL, a.Config.Session.CookieSecure)
	return c.Redirect().Status(http.StatusFound).To(start.URL)
}

func (a *Handlers) OAuthCallback(c fiber.Ctx) error {
	var previous *string
	if token := c.Cookies(SessionCookie); token != "" {
		previous = &token
	}
	cb := accessservice.LoginCallback{Provider: c.Params("provider"), BrowserID: c.Cookies(OAuthCookie), State: c.Query("state"),
		Code: c.Query("code"), Error: c.Query("error")}
	done, err := a.Access.CompleteLogin(c.Context(), cb, previous)
	secure := a.Config.Session.CookieSecure
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
		return responses.Fail(c, err)
	}
	if done.Token != "" {
		session := a.Config.Session
		setSessionCookie(c, done.Token, session.AbsoluteTimeoutSecs, secure)
	}
	return c.Redirect().Status(http.StatusFound).To(done.Next)
}
