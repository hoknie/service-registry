package services

import (
	"context"

	"github.com/google/uuid"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type cardBuilder struct{ parts }

type policies map[uuid.UUID]skillstypes.Policy

func (p policies) effective(s skillstypes.Skill) skillstypes.Effective {
	var own, org *skillstypes.Policy
	if v, ok := p[s.ID]; ok {
		own = &v
	}
	if s.OrganizationID != nil {
		if v, ok := p[*s.OrganizationID]; ok {
			org = &v
		}
	}
	return skillstypes.Merge(own, org)
}

func (c *cardBuilder) policies(ctx context.Context, skills []skillstypes.Skill) (policies, error) {
	ids, orgs := make([]uuid.UUID, 0, len(skills)), []uuid.UUID{}
	for _, s := range skills {
		ids = append(ids, s.ID)
		if s.OrganizationID != nil {
			orgs = append(orgs, *s.OrganizationID)
		}
	}
	found, err := c.versions.Policies(ctx, ids, orgs)
	return policies(found), err
}

func (c *cardBuilder) decorate(ctx context.Context, skills []skillstypes.Skill, w skillstypes.Visible) ([]skillstypes.Skill, error) {
	if len(skills) == 0 {
		return skills, nil
	}
	ids := make([]uuid.UUID, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	versions, err := c.versions.Of(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	pol, err := c.policies(ctx, skills)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for i := range skills {
		s := &skills[i]
		s.CanWrite = writable(w, s.Scope())
		eff := pol.effective(*s)
		s.Versions = eff.Mark(versions[s.ID], s.Status == skillstypes.StatusPublished)
		s.Version, s.VersionsAvailable = nil, 0
		for j := range s.Versions {
			if s.Versions[j].Available {
				s.VersionsAvailable++
				if s.Version == nil {
					s.Version = &s.Versions[j]
				}
			}
		}
		if s.Version == nil && s.CanWrite && len(s.Versions) > 0 {
			s.Version = &s.Versions[0]
		}
		s.HasImage = s.HasUploadedImage || (s.ImagePath != nil && s.Version != nil)
	}
	return skills, nil
}

func (c *cardBuilder) one(ctx context.Context, s skillstypes.Skill) (skillstypes.Skill, error) {
	w := skillstypes.Visible{}
	if s.CanWrite {
		if s.OrganizationID != nil {
			w.Organizations = []uuid.UUID{*s.OrganizationID}
		} else {
			w.Global = true
		}
	}
	out, err := c.decorate(ctx, []skillstypes.Skill{s}, w)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	return out[0], nil
}

func (c *cardBuilder) pick(s skillstypes.Skill, selector string) (skillstypes.Version, error) {
	v, err := skillstypes.Pick(s.Versions, selector, s.CanWrite)
	if err != nil {
		return skillstypes.Version{}, apperr.Wrap(err)
	}
	if v == nil {
		return skillstypes.Version{}, apperr.New(apperr.NotFound)
	}
	return *v, nil
}
