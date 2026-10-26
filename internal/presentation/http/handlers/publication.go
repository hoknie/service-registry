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

func (a *Handlers) GetPublication(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Publishing.Settings.GetPublication(c.Context(), p, id)
		return responses.PublicationOf(out), err
	})
}

func (a *Handlers) PutPublication(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Publication
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	own, err := in.Settings()
	if err != nil {
		return responses.Fail(c, apperr.Wrap(err))
	}
	out, err := a.Publishing.Settings.PutPublication(c.Context(), p, id, own)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PublicationOf(out))
}
