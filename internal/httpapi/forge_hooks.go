package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/service"
)

const HookBodyLimit = 1 << 20

func (a *API) ForgeHook(c fiber.Ctx) error {
	body := c.Body()
	if len(body) > HookBodyLimit {
		return NewAPIError(http.StatusRequestEntityTooLarge, "validation.invalid_body", "the delivery is larger than 1 MiB").Send(c)
	}
	header := func(name string) string { return c.Get(name) }
	if err := service.AcceptDelivery(c.Context(), a.State, c.Params("connection_id"), header, body); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}
