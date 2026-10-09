package httpapi

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func connectionIDs(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	node, err := parseID(c, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	conn, err := parseID(c, "cid")
	return node, conn, err
}

func (a *API) ListConnections(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	list, err := service.ListConnections(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(list, response.ConnectionOf))
}

func (a *API) CreateConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.CreateConnection
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	v, err := service.CreateConnection(c.Context(), a.State, p, id, in.Connection())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.ConnectionOf(v))
}

func (a *API) GetConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	v, err := service.GetConnection(c.Context(), a.State, p, node, conn)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ConnectionOf(v))
}

func (a *API) UpdateConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.UpdateConnection
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	v, err := service.UpdateConnection(c.Context(), a.State, p, node, conn, in.Changes())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ConnectionOf(v))
}

type connectionAction func(context.Context, *service.State, service.Principal, uuid.UUID, uuid.UUID) error

func (a *API) connectionCall(c fiber.Ctx, action connectionAction, status int) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	if err := action(c.Context(), a.State, p, node, conn); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(status)
}

func (a *API) DeleteConnection(c fiber.Ctx) error {
	return a.connectionCall(c, service.DeleteConnection, http.StatusNoContent)
}

func (a *API) StartSync(c fiber.Ctx) error {
	return a.connectionCall(c, service.StartSync, http.StatusAccepted)
}

func (a *API) DeleteWebhook(c fiber.Ctx) error {
	return a.connectionCall(c, service.DeleteWebhook, http.StatusNoContent)
}

func (a *API) CheckConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	if err := service.CheckConnection(c.Context(), a.State, p, node, conn); err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.OK{OK: true})
}

func (a *API) PreviewConnection(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.PreviewConnection(c.Context(), a.State, p, node, conn)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(items, response.PreviewItemOf))
}

func (a *API) ListRuns(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	runs, err := service.ListRuns(c.Context(), a.State, p, node, conn)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(runs, response.RunOf))
}

func (a *API) SetupWebhook(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.Webhook
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	node, conn, err := connectionIDs(c)
	if err != nil {
		return Fail(c, err)
	}
	setup, err := service.SetupWebhook(c.Context(), a.State, p, node, conn, *in.Mode)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.WebhookSetupOf(setup))
}

func (a *API) Readme(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	r, err := service.ProjectReadme(c.Context(), a.State, p, id)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ReadmeOf(r))
}
