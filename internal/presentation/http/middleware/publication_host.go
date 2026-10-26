package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type HostLookup func(ctx context.Context, host string) (uuid.UUID, bool, error)

func PublicationHost(lookup HostLookup, key string, page, notFound fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, ok, err := lookup(c.Context(), string(c.Request().Host()))
		if err != nil || !ok {
			return c.Next()
		}
		c.Locals(key, id)
		path := string(c.Request().URI().Path())
		switch {
		case strings.HasPrefix(path, "/api/public/docs/"):
			return c.Next()
		case path == "/api" || strings.HasPrefix(path, "/api/") || path == "/metrics" || strings.HasPrefix(path, "/.well-known/"):
			return notFound(c)
		}
		return page(c)
	}
}
