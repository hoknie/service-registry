package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) SystemMetrics(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	v, err := a.Access.Auth.MetricsView(p)
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(http.StatusOK).JSON(responses.MetricsViewOf(v))
}

func (a *Handlers) SystemMetricsFamily(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	f, err := a.Access.Auth.MetricsFamily(p, c.Query("name"))
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(http.StatusOK).JSON(responses.MetricFamilyOf(*f))
}
