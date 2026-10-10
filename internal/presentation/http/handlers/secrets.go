package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListNodeSecrets(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Catalog.ListNodeSecrets(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SecretsOf(items, &id))
}

func (a *Handlers) CreateNodeSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Secret
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	created, err := a.Catalog.CreateNodeSecret(c.Context(), p, id, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.SecretOf(created, &id))
}

func (a *Handlers) UpdateNodeSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, sid, err := nodeAndSecret(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Secret
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	updated, err := a.Catalog.UpdateNodeSecret(c.Context(), p, id, sid, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SecretOf(updated, &id))
}

func (a *Handlers) DeleteNodeSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, sid, err := nodeAndSecret(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Catalog.DeleteNodeSecret(c.Context(), p, id, sid); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func nodeAndSecret(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	id, err := parseID(c, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	sid, err := parseID(c, "sid")
	return id, sid, err
}

func (a *Handlers) ListSecrets(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Catalog.ListGlobalSecrets(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SecretsOf(items, nil))
}

func (a *Handlers) CreateSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Secret
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	created, err := a.Catalog.CreateGlobalSecret(c.Context(), p, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.SecretOf(created, nil))
}

func (a *Handlers) GetSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	sid, err := parseID(c, "sid")
	if err != nil {
		return responses.Fail(c, err)
	}
	found, err := a.Catalog.GetGlobalSecret(c.Context(), p, sid)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SecretOf(found, nil))
}

func (a *Handlers) UpdateSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	sid, err := parseID(c, "sid")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Secret
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	updated, err := a.Catalog.UpdateGlobalSecret(c.Context(), p, sid, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SecretOf(updated, nil))
}

func (a *Handlers) DeleteSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	sid, err := parseID(c, "sid")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Catalog.DeleteGlobalSecret(c.Context(), p, sid); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}
