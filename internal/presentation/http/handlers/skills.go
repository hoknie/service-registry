package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) skillCall(c fiber.Ctx, param string, status int, fn func(accesstypes.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var id uuid.UUID
	if param != "" {
		if id, err = parseID(c, param); err != nil {
			return responses.Fail(c, err)
		}
	}
	body, err := fn(p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(status).JSON(body)
}

func skillBody[T any](c fiber.Ctx, out *T) error { return bindJSON(c, out) }

func (a *Handlers) ListSkillFolders(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var parent *uuid.UUID
		if raw := c.Query("parent_id"); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return nil, skillstypes.InvalidFilter
			}
			parent = &id
		}
		out, err := a.Skills.Folders.ListFolders(c.Context(), p, parent)
		return responses.SkillFoldersOf(out), err
	})
}

func (a *Handlers) CreateSkillFolder(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusCreated, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var in requests.SkillFolderCreate
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Folders.CreateFolder(c.Context(), p, in.Input())
		return responses.SkillFolderOf(out), err
	})
}

func (a *Handlers) GetSkillFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Folders.GetFolder(c.Context(), p, id)
		return responses.SkillFolderOf(out), err
	})
}

func (a *Handlers) UpdateSkillFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillFolderPatch
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Folders.UpdateFolder(c.Context(), p, id, in.Patch())
		return responses.SkillFolderOf(out), err
	})
}

func (a *Handlers) DeleteSkillFolder(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Skills.Folders.DeleteFolder(c.Context(), p, id)
	})
}

func (a *Handlers) ListSkills(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var q requests.SkillsQuery
		if err := bindQuery(c, &q, skillstypes.InvalidFilter, map[string]error{"limit": skillstypes.InvalidPagination}); err != nil {
			return nil, err
		}
		in, err := q.Input()
		if err != nil {
			return nil, err
		}
		out, err := a.Skills.Skills.ListSkills(c.Context(), p, in)
		return responses.SkillPageOf(out), err
	})
}

func (a *Handlers) SkillTags(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusOK, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		out, err := a.Skills.Skills.SkillTags(c.Context(), p, c.Query("q"))
		return responses.SkillTagsOf(out), err
	})
}

func (a *Handlers) GetSkill(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Skills.GetSkill(c.Context(), p, id)
		return responses.SkillOf(out), err
	})
}

func (a *Handlers) PatchSkill(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillPatch
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Skills.PatchSkill(c.Context(), p, id, in.Patch())
		return responses.SkillOf(out), err
	})
}

func (a *Handlers) DeleteSkill(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Skills.Skills.DeleteSkill(c.Context(), p, id)
	})
}
