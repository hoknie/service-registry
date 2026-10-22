package handlers

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

type Readiness interface {
	Ready(ctx context.Context) bool
}

func (a *Handlers) Health(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(responses.Status{Status: "ok"})
}

func (a *Handlers) Ready(c fiber.Ctx) error {
	if !a.Readiness.Ready(c.Context()) {
		return c.Status(http.StatusServiceUnavailable).JSON(responses.Status{Status: "unavailable"})
	}
	return c.Status(http.StatusOK).JSON(responses.Status{Status: "ok"})
}
