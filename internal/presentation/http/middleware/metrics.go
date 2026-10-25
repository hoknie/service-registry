package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/platform/metrics"
)

func Metrics(m *metrics.Metrics) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		m.HTTPInFlight(1)
		defer m.HTTPInFlight(-1)
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			}
		}
		m.HTTPRequest(c.Method(), routeOf(c), status, time.Since(start))
		return err
	}
}

func routeOf(c fiber.Ctx) string {
	switch p := c.Route().Path; p {
	case "/":
		return "web"
	case "/api", "/api/*":
		return "unmatched"
	default:
		return p
	}
}
