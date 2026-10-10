package handlers

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func connectionIDs(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	node, err := parseID(c, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	conn, err := parseID(c, "cid")
	return node, conn, err
}

func (a *Handlers) ListConnections(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	list, err := a.Forge.ListConnections(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(list, responses.ConnectionOf))
}

func (a *Handlers) CreateConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.CreateConnection
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	v, err := a.Forge.CreateConnection(c.Context(), p, id, in.Connection())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.ConnectionOf(v))
}

func (a *Handlers) GetConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	v, err := a.Forge.GetConnection(c.Context(), p, node, conn)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ConnectionOf(v))
}

func (a *Handlers) UpdateConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.UpdateConnection
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	v, err := a.Forge.UpdateConnection(c.Context(), p, node, conn, in.Changes())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ConnectionOf(v))
}

type connectionAction func(context.Context, access.Principal, uuid.UUID, uuid.UUID) error

func (a *Handlers) connectionCall(c fiber.Ctx, action connectionAction, status int) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := action(c.Context(), p, node, conn); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(status)
}

func (a *Handlers) DeleteConnection(c fiber.Ctx) error {
	return a.connectionCall(c, a.Forge.DeleteConnection, http.StatusNoContent)
}

func (a *Handlers) StartSync(c fiber.Ctx) error {
	return a.connectionCall(c, a.Forge.StartSync, http.StatusAccepted)
}

func (a *Handlers) DeleteWebhook(c fiber.Ctx) error {
	return a.connectionCall(c, a.Forge.DeleteWebhook, http.StatusNoContent)
}

func (a *Handlers) CheckConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Forge.CheckConnection(c.Context(), p, node, conn); err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.OK{OK: true})
}

func (a *Handlers) PreviewConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Forge.PreviewConnection(c.Context(), p, node, conn)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.PreviewItemOf))
}

func (a *Handlers) ListRuns(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	runs, err := a.Forge.ListRuns(c.Context(), p, node, conn)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(runs, responses.RunOf))
}

func (a *Handlers) SetupWebhook(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Webhook
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	setup, err := a.Forge.SetupWebhook(c.Context(), p, node, conn, *in.Mode)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.WebhookSetupOf(setup))
}

func (a *Handlers) Readme(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	r, err := a.Forge.ProjectReadme(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ReadmeOf(r))
}
