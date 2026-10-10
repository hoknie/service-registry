package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

const HookBodyLimit = 1 << 20

func (a *Handlers) ForgeHook(c fiber.Ctx) error {
	body := c.Body()
	if len(body) > HookBodyLimit {
		return responses.NewAPIError(http.StatusRequestEntityTooLarge, "validation.invalid_body", "the delivery is larger than 1 MiB").Send(c)
	}
	header := func(name string) string { return c.Get(name) }
	if err := a.Forge.AcceptDelivery(c.Context(), c.Params("connection_id"), header, body); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}
