package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) Health(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(response.Status{Status: "ok"})
}

func (a *API) Ready(c fiber.Ctx) error {
	if err := service.Ready(c.Context(), a.State); err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(response.Status{Status: "unavailable"})
	}
	return c.Status(http.StatusOK).JSON(response.Status{Status: "ok"})
}
