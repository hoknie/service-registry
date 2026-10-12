package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) DocSpace(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	space, err := a.Knowledge.DocSpace(c.Context(), p, c.Query("path"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.DocSpaceOf(space))
}
