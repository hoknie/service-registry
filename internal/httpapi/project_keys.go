package httpapi

import (
	"bytes"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListKeys(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	keys, err := service.ListKeys(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(keys, response.KeyOf))
}

func (a *API) RotateKey(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	var in request.RotateKey
	if len(bytes.TrimSpace(c.Body())) > 0 {
		if err := c.Bind().JSON(&in); err != nil {
			return Fail(c, InvalidBody(describe(err)))
		}
	}
	issued, err := service.RotateKey(c.Context(), a.State, p, id, in.GraceSecs)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.IssuedKeyOf(issued))
}

func (a *API) RevokeKey(c fiber.Ctx) error {
	return a.nodeChild(c, "key_id", service.RevokeKey)
}
