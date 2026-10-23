package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) NodeOwners(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	set, err := a.Ownership.Owners.NodeOwners(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.OwnerSetOf(set))
}

func (a *Handlers) PutNodeOwners(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Owners
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	set, err := a.Ownership.Owners.SetNodeOwners(c.Context(), p, id, in.Set(), in.None)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.OwnerSetOf(set))
}

func (a *Handlers) OwnerCandidates(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.OwnerQuery
	if err := bindQuery(c, &q, catalogtypes.InvalidFilter, nil); err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Ownership.Owners.OwnerCandidates(c.Context(), p, id, q.Q)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.OwnerCandidateOf))
}

func (a *Handlers) SuggestOwners(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.OwnerQuery
	if err := bindQuery(c, &q, catalogtypes.InvalidFilter, nil); err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Ownership.Owners.SuggestOwners(c.Context(), p, q.Q)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.OwnerSuggestionOf))
}
