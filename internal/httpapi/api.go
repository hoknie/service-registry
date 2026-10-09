package httpapi

import (
	"sync"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/service"
)

type API struct {
	State *service.State

	mcpOnce    sync.Once
	mcpHandler fiber.Handler
}
