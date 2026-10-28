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

func (a *Handlers) ListSkillSources(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Sources.ListSources(c.Context(), p, id)
		return responses.SkillSourcesOf(out), err
	})
}

func (a *Handlers) CreateSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusCreated, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillSource
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Sources.CreateSource(c.Context(), p, id, in.Input())
		return responses.SkillSourceOf(out), err
	})
}

func (a *Handlers) GetSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "sid", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Sources.GetSource(c.Context(), p, id)
		return responses.SkillSourceOf(out), err
	})
}

func (a *Handlers) UpdateSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "sid", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillSource
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Sources.UpdateSource(c.Context(), p, id, in.Input())
		return responses.SkillSourceOf(out), err
	})
}

func (a *Handlers) DeleteSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "sid", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Skills.Sources.DeleteSource(c.Context(), p, id)
	})
}

func (a *Handlers) SyncSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "sid", http.StatusAccepted, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Sources.QueueSource(c.Context(), p, id)
		return responses.SkillSourceOf(out), err
	})
}

func (a *Handlers) ListSkillVersions(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var preview *skillstypes.Policy
		args := c.Request().URI().QueryArgs()
		if args.Has("range") || args.Has("prerelease") || args.Has("exclude") {
			preview = &skillstypes.Policy{}
			if args.Has("range") {
				preview.Range = new(c.Query("range"))
			}
			if args.Has("prerelease") {
				preview.Prerelease = new(c.Query("prerelease") == "true")
			}
			if args.Has("exclude") {
				var exclude []string
				for _, v := range args.PeekMulti("exclude") {
					exclude = append(exclude, string(v))
				}
				preview.Exclude = &exclude
			}
		}
		out, err := a.Skills.Versions.ListVersions(c.Context(), p, id, preview)
		return map[string][]responses.SkillVersion{"items": responses.SkillVersionsOf(out)}, err
	})
}

func (a *Handlers) YankSkillVersion(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillYank
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Versions.YankVersion(c.Context(), p, id, c.Params("version"), *in.Yanked)
		return responses.SkillVersionOf(out), err
	})
}

func (a *Handlers) SkillPolicy(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Versions.SkillPolicy(c.Context(), p, id)
		return responses.SkillPolicyOf(out), err
	})
}

func (a *Handlers) PutSkillPolicy(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillPolicy
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Versions.PutSkillPolicy(c.Context(), p, id, in.Policy())
		return responses.SkillPolicyOf(out), err
	})
}

func (a *Handlers) OrganizationSkillPolicy(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Versions.OrganizationPolicy(c.Context(), p, id)
		return responses.SkillPolicyOf(out), err
	})
}

func (a *Handlers) PutOrganizationSkillPolicy(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillPolicy
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Versions.PutOrganizationPolicy(c.Context(), p, id, in.Policy())
		return responses.SkillPolicyOf(out), err
	})
}

func (a *Handlers) OrganizationSkillSpace(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Spaces.OrganizationSpace(c.Context(), p, id)
		return responses.SkillSpaceOf(out), err
	})
}

func (a *Handlers) PutOrganizationSkillSpace(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillSpace
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Spaces.PutOrganizationSpace(c.Context(), p, id, *in.Description)
		return responses.SkillSpaceOf(out), err
	})
}

func (a *Handlers) ImportSkillSource(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusCreated, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		var in requests.SkillImport
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		f, src, err := a.Skills.Sources.ImportSource(c.Context(), p, in.Input())
		if err != nil {
			return nil, err
		}
		return responses.SkillImported{Folder: responses.SkillFolderOf(f), Source: responses.SkillSourceOf(src)}, nil
	})
}

func (a *Handlers) OrganizationSkillSet(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Spaces.OrganizationSet(c.Context(), p, id)
		return responses.SkillSetOf(out), err
	})
}

func (a *Handlers) PutOrganizationSkillSet(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		var in requests.SkillSet
		if err := skillBody(c, &in); err != nil {
			return nil, err
		}
		out, err := a.Skills.Spaces.PutOrganizationSet(c.Context(), p, id, in.Mode, in.Folders, in.Skills)
		return responses.SkillSetOf(out), err
	})
}
