package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type accessRules struct {
	catalog *catalogservices.CatalogServices
}

func globalRight(p accesstypes.Principal, action accesstypes.Action) error {
	if p.ByToken() && !p.Scopes.Has(accesstypes.ScopeSkills) {
		return apperr.New(apperr.InsufficientScope)
	}
	return accesstypes.RequireRight(p, accesstypes.SectionSkills, action)
}

func (a *accessRules) canRead(ctx context.Context, p accesstypes.Principal, s skillstypes.Scope) error {
	if s.Global() {
		return nil
	}
	if _, err := a.catalog.Access.Scope(ctx, p, *s.OrganizationID); err != nil {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (a *accessRules) canWrite(ctx context.Context, p accesstypes.Principal, s skillstypes.Scope, action accesstypes.Action) error {
	if s.Global() {
		return globalRight(p, action)
	}
	_, err := a.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, *s.OrganizationID)
	return err
}

func (a *accessRules) writes(ctx context.Context, p accesstypes.Principal, s skillstypes.Scope) bool {
	return a.canWrite(ctx, p, s, accesstypes.ActionUpdate) == nil
}

func (a *accessRules) visible(ctx context.Context, p accesstypes.Principal) (skillstypes.Visible, skillstypes.Visible, error) {
	read, err := a.catalog.Access.VisibleOrganizations(ctx, p, catalogtypes.PermRead)
	if err != nil {
		return skillstypes.Visible{}, skillstypes.Visible{}, err
	}
	write, err := a.catalog.Access.VisibleOrganizations(ctx, p, catalogtypes.PermWrite)
	if err != nil {
		return skillstypes.Visible{}, skillstypes.Visible{}, err
	}
	return skillstypes.Visible{Organizations: read, Global: true},
		skillstypes.Visible{Organizations: write, Global: globalRight(p, accesstypes.ActionUpdate) == nil}, nil
}

func writable(w skillstypes.Visible, s skillstypes.Scope) bool {
	if s.Global() {
		return w.Global
	}
	for _, id := range w.Organizations {
		if id == *s.OrganizationID {
			return true
		}
	}
	return false
}

func readSkill(ctx context.Context, p parts, pr accesstypes.Principal, id uuid.UUID) (skillstypes.Skill, error) {
	s, err := p.skills.Get(ctx, id)
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	if s == nil {
		return skillstypes.Skill{}, apperr.New(apperr.NotFound)
	}
	if err := p.access.canRead(ctx, pr, s.Scope()); err != nil {
		return skillstypes.Skill{}, err
	}
	s.CanWrite = p.access.writes(ctx, pr, s.Scope())
	if s.Status == skillstypes.StatusDraft && !s.CanWrite {
		return skillstypes.Skill{}, apperr.New(apperr.NotFound)
	}
	return *s, nil
}

func writeSkill(ctx context.Context, p parts, pr accesstypes.Principal, id uuid.UUID, action accesstypes.Action) (skillstypes.Skill, error) {
	s, err := readSkill(ctx, p, pr, id)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	if err := p.access.canWrite(ctx, pr, s.Scope(), action); err != nil {
		return skillstypes.Skill{}, err
	}
	return s, nil
}

func (a *accessRules) setOrgs(ctx context.Context, p accesstypes.Principal) ([]uuid.UUID, error) {
	if len(p.Nodes) == 0 {
		return nil, nil
	}
	orgs, err := a.catalog.Access.NodeOrganizations(ctx, p.Nodes)
	if err != nil {
		return nil, err
	}
	if orgs == nil {
		orgs = []uuid.UUID{}
	}
	return orgs, nil
}

func inSet(ctx context.Context, p parts, pr accesstypes.Principal, id uuid.UUID) error {
	orgs, err := p.access.setOrgs(ctx, pr)
	if err != nil || orgs == nil {
		return err
	}
	rows, err := p.skills.List(ctx, repositories.Filter{All: true, ListQuery: skillstypes.ListQuery{IDs: []uuid.UUID{id}, SetOrgs: orgs, Limit: 1}})
	if err != nil {
		return apperr.Wrap(err)
	}
	if len(rows) == 0 {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func forAgent(ctx context.Context, p parts, pr accesstypes.Principal, sk skillstypes.Skill) error {
	if !sk.Active() {
		return apperr.New(apperr.NotFound)
	}
	return inSet(ctx, p, pr, sk.ID)
}
