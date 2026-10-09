package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

const SessionCookie = "registry_session"

func setSessionCookie(c fiber.Ctx, token string, maxAgeSecs uint64, secure bool) {
	cookie := http.Cookie{
		Name: SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(maxAgeSecs),
	}
	if maxAgeSecs == 0 {
		cookie.MaxAge = -1
	}
	c.Response().Header.Add(fiber.HeaderSetCookie, cookie.String())
}
