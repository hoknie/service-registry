package handlers

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/platform/postgres"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) Health(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(responses.Status{Status: "ok"})
}

func (a *Handlers) Ready(c fiber.Ctx) error {
	timeout := time.Duration(a.Config.DB.AcquireTimeoutSecs) * time.Second
	if !postgres.Ping(c.Context(), a.DB.Pool(), timeout) {
		return c.Status(http.StatusServiceUnavailable).JSON(responses.Status{Status: "unavailable"})
	}
	return c.Status(http.StatusOK).JSON(responses.Status{Status: "ok"})
}
