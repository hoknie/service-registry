package handlers

import (
	"bytes"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListKeys(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	keys, err := a.Catalog.ListKeys(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(keys, responses.KeyOf))
}

func (a *Handlers) RotateKey(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.RotateKey
	if len(bytes.TrimSpace(c.Body())) > 0 {
		if err := c.Bind().JSON(&in); err != nil {
			return responses.Fail(c, InvalidBody(describe(err)))
		}
	}
	issued, err := a.Catalog.RotateKey(c.Context(), p, id, in.GraceSecs)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.IssuedKeyOf(issued))
}

func (a *Handlers) RevokeKey(c fiber.Ctx) error {
	return a.nodeChild(c, "key_id", a.Catalog.RevokeKey)
}
