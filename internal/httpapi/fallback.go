package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

func APINotFound(c fiber.Ctx) error {
	return NewAPIError(http.StatusNotFound, "not_found",
		"no API route for "+c.Method()+" "+string(c.Request().URI().PathOriginal())).Send(c)
}

func APIMethodNotAllowed(c fiber.Ctx) error {
	return NewAPIError(http.StatusMethodNotAllowed, "method_not_allowed",
		c.Method()+" is not allowed on "+string(c.Request().URI().PathOriginal())).Send(c)
}

func APIMethodNotAllowedWith(allow string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderAllow, allow)
		return APIMethodNotAllowed(c)
	}
}
