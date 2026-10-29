package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) GetPolling(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Polling.Settings.GetPolling(c.Context(), p, id)
		return responses.PollingOf(out), err
	})
}

func (a *Handlers) PutPolling(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var raw map[string]any
	if err := bindJSON(c, &raw); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	own, err := requests.ParsePolling(c.Body())
	if err != nil {
		return responses.Fail(c, apperr.Wrap(err))
	}
	out, err := a.Polling.Settings.PutPolling(c.Context(), p, id, own)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PollingOf(out))
}
