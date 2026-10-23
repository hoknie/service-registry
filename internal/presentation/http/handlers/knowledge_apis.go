package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) KnowledgeAPIs(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		l, err := a.Apis.Specs.ListAPIs(c.Context(), p, id, knowledgeRef(c), c.Query("all") == "1")
		return responses.APIListOf(l), err
	})
}

func (a *Handlers) KnowledgeAPI(c fiber.Ctx) error {
	return a.knowledgeCall(c, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		s, d, err := a.Apis.Specs.GetAPI(c.Context(), p, id, knowledgeRef(c), c.Query("path"))
		return responses.APIDocumentOf(s, d), err
	})
}
