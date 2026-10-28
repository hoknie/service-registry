package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type VersionsService struct {
	parts
	cards *cardBuilder
}

type PolicyView struct {
	Own       skillstypes.Policy
	Effective skillstypes.Effective
	CanWrite  bool
}

func (s *VersionsService) ListVersions(ctx context.Context, p accesstypes.Principal, id uuid.UUID, preview *skillstypes.Policy) ([]skillstypes.Version, error) {
	sk, err := readSkill(ctx, s.parts, p, id)
	if err != nil {
		return nil, err
	}
	sk, err = s.cards.one(ctx, sk)
	if err != nil {
		return nil, err
	}
	if preview != nil && sk.CanWrite {
		pol, err := skillstypes.ValidatePolicy(*preview)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		org, err := s.orgPolicy(ctx, sk.OrganizationID)
		if err != nil {
			return nil, err
		}
		eff := skillstypes.Merge(&pol, org)
		sk.Versions = eff.Mark(sk.Versions, sk.Status == skillstypes.StatusPublished)
	}
	if sk.CanWrite {
		return sk.Versions, nil
	}
	out := []skillstypes.Version{}
	for _, v := range sk.Versions {
		if v.Available {
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *VersionsService) orgPolicy(ctx context.Context, org *uuid.UUID) (*skillstypes.Policy, error) {
	if org == nil {
		return nil, nil
	}
	found, err := s.versions.Policies(ctx, nil, []uuid.UUID{*org})
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if p, ok := found[*org]; ok {
		return &p, nil
	}
	return nil, nil
}

func (s *VersionsService) YankVersion(ctx context.Context, p accesstypes.Principal, id uuid.UUID, version string, yanked bool) (skillstypes.Version, error) {
	if _, err := writeSkill(ctx, s.parts, p, id, accesstypes.ActionUpdate); err != nil {
		return skillstypes.Version{}, err
	}
	canonical, err := skillstypes.ParseVersion(version)
	if err != nil {
		return skillstypes.Version{}, apperr.New(apperr.NotFound)
	}
	if err := s.versions.Yank(ctx, id, canonical, yanked); err != nil {
		return skillstypes.Version{}, apperr.Wrap(err)
	}
	versions, err := s.ListVersions(ctx, p, id, nil)
	if err != nil {
		return skillstypes.Version{}, err
	}
	for _, v := range versions {
		if v.Version == canonical {
			return v, nil
		}
	}
	return skillstypes.Version{}, apperr.New(apperr.NotFound)
}

func (s *VersionsService) SkillPolicy(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (PolicyView, error) {
	sk, err := readSkill(ctx, s.parts, p, id)
	if err != nil {
		return PolicyView{}, err
	}
	found, err := s.cards.policies(ctx, []skillstypes.Skill{sk})
	if err != nil {
		return PolicyView{}, apperr.Wrap(err)
	}
	return PolicyView{Own: found[sk.ID], Effective: found.effective(sk), CanWrite: sk.CanWrite}, nil
}

func (s *VersionsService) PutSkillPolicy(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in skillstypes.Policy) (PolicyView, error) {
	if _, err := writeSkill(ctx, s.parts, p, id, accesstypes.ActionUpdate); err != nil {
		return PolicyView{}, err
	}
	if err := s.put(ctx, &id, nil, in); err != nil {
		return PolicyView{}, err
	}
	return s.SkillPolicy(ctx, p, id)
}

func (s *VersionsService) put(ctx context.Context, skill, org *uuid.UUID, in skillstypes.Policy) error {
	pol, err := skillstypes.ValidatePolicy(in)
	if err != nil {
		return apperr.Wrap(err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.versions.PutPolicy(ctx, id, skill, org, pol))
}

func (s *VersionsService) OrganizationPolicy(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (PolicyView, error) {
	sc, err := s.access.catalog.Access.Scope(ctx, p, org)
	if err != nil {
		return PolicyView{}, err
	}
	if len(sc.Path) > 0 {
		return PolicyView{}, apperr.New(apperr.NotFound)
	}
	own, err := s.orgPolicy(ctx, &org)
	if err != nil {
		return PolicyView{}, err
	}
	out := PolicyView{Effective: skillstypes.Merge(nil, own), CanWrite: p.IsSuperadmin || sc.Allows(catalogtypes.PermWrite)}
	if own != nil {
		out.Own = *own
	}
	return out, nil
}

func (s *VersionsService) PutOrganizationPolicy(ctx context.Context, p accesstypes.Principal, org uuid.UUID, in skillstypes.Policy) (PolicyView, error) {
	sc, err := s.access.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, org)
	if err != nil {
		return PolicyView{}, err
	}
	if len(sc.Path) > 0 {
		return PolicyView{}, apperr.New(apperr.NotFound)
	}
	if err := s.put(ctx, nil, &org, in); err != nil {
		return PolicyView{}, err
	}
	return s.OrganizationPolicy(ctx, p, org)
}
