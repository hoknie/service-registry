package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type SkillsService struct {
	parts
	cards *cardBuilder
}

type Page struct {
	Items      []skillstypes.Skill
	NextCursor *string
}

type ListInput struct {
	FolderID  *uuid.UUID
	TopLevel  bool
	Recursive bool
	Scope     string
	Query     string
	Tags      []string
	Status    string
	Active    *bool
	Agent     bool
	Limit     int
	Cursor    string
	Name      string
}

func (in ListInput) query() (skillstypes.ListQuery, error) {
	q := skillstypes.ListQuery{FolderID: in.FolderID, TopLevel: in.TopLevel && in.FolderID == nil, Recursive: in.Recursive, Query: in.Query, Limit: in.Limit,
		Active: in.Active}
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit < 1 || q.Limit > 100 {
		return q, skillstypes.InvalidPagination
	}
	switch in.Scope {
	case "":
	case "global":
		q.OnlyGlobal = true
	default:
		id, err := uuid.Parse(in.Scope)
		if err != nil {
			return q, skillstypes.InvalidFilter
		}
		q.OnlyOrg = &id
	}
	for _, t := range in.Tags {
		if !skillstypes.ValidTag(t) {
			return q, skillstypes.InvalidFilter
		}
		q.Tags = append(q.Tags, t)
	}
	if in.Status != "" {
		st, ok := skillstypes.ParseStatus(in.Status)
		if !ok {
			return q, skillstypes.InvalidFilter
		}
		q.Status = &st
	}
	if in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		var c skillstypes.Cursor
		if err != nil || json.Unmarshal(raw, &c) != nil || c.UpdatedAt == "" {
			return q, skillstypes.InvalidPagination
		}
		q.After = &c
	}
	return q, nil
}

func (s *SkillsService) ListSkills(ctx context.Context, p accesstypes.Principal, in ListInput) (Page, error) {
	q, err := in.query()
	if err != nil {
		return Page{}, apperr.Wrap(err)
	}
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return Page{}, err
	}
	q.Organizations, q.Global, q.Writable, q.WriteGlobal = read.Organizations, read.Global, write.Organizations, write.Global
	if in.Agent {
		q.Status, q.Active = nil, new(true)
		if q.SetOrgs, err = s.access.setOrgs(ctx, p); err != nil {
			return Page{}, apperr.Wrap(err)
		}
	}
	f := repositories.Filter{ListQuery: q}
	if in.Name != "" {
		f.Name = &in.Name
	}
	f.Limit = q.Limit + 1
	rows, err := s.skills.List(ctx, f)
	if err != nil {
		return Page{}, apperr.Wrap(err)
	}
	out := Page{Items: []skillstypes.Skill{}}
	for i, r := range rows {
		if i == q.Limit {
			raw, _ := json.Marshal(rows[i-1].Cursor)
			out.NextCursor = new(base64.RawURLEncoding.EncodeToString(raw))
			break
		}
		out.Items = append(out.Items, r.Skill)
	}
	out.Items, err = s.cards.decorate(ctx, out.Items, write)
	return out, err
}

func (s *SkillsService) GetSkill(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Skill, error) {
	sk, err := readSkill(ctx, s.parts, p, id)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	return s.cards.one(ctx, sk)
}

func (s *SkillsService) SkillTags(ctx context.Context, p accesstypes.Principal, q string) ([]skillstypes.TagCount, error) {
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return nil, err
	}
	out, err := s.skills.Tags(ctx, read, write, q)
	return out, apperr.Wrap(err)
}

type SkillPatch struct {
	Status         *string
	Tags           *[]string
	Slug           *string
	About          *string
	Move           bool
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
}

func (s *SkillsService) PatchSkill(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in SkillPatch) (skillstypes.Skill, error) {
	sk, err := writeSkill(ctx, s.parts, p, id, accesstypes.ActionUpdate)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	if sk.Managed() && (in.Tags != nil || in.Slug != nil || in.Move) {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.ConflictManaged)
	}
	var status *skillstypes.Status
	if in.Status != nil {
		st, ok := skillstypes.ParseStatus(*in.Status)
		if !ok {
			return skillstypes.Skill{}, apperr.Wrap(skillstypes.InvalidSkill)
		}
		status = &st
	}
	var tags []string
	if in.Tags != nil {
		if tags, err = skillstypes.NormalizeTags(*in.Tags); err != nil {
			return skillstypes.Skill{}, apperr.Wrap(err)
		}
	}
	if in.About != nil && utf8.RuneCountInString(*in.About) > skillstypes.MaxAbout {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.InvalidSkill)
	}
	if in.Slug != nil && !skillstypes.ValidSlug(*in.Slug) {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.InvalidSkill)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.skills.Patch(ctx, id, status, tags, in.Slug, in.About); err != nil {
			return err
		}
		if !in.Move {
			return nil
		}
		target, err := s.target(ctx, p, in.FolderID, in.OrganizationID, accesstypes.ActionUpdate)
		if err != nil {
			return err
		}
		return s.skills.Move(ctx, id, in.FolderID, target.OrganizationID)
	})
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(repositories.Err(err))
	}
	return s.GetSkill(ctx, p, id)
}

func (s *SkillsService) target(ctx context.Context, p accesstypes.Principal, folder, org *uuid.UUID, action accesstypes.Action) (skillstypes.Scope, error) {
	return placeOf(ctx, s.parts, p, folder, org, action)
}

func (s *SkillsService) DeleteSkill(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	sk, err := writeSkill(ctx, s.parts, p, id, accesstypes.ActionDelete)
	if err != nil {
		return err
	}
	if sk.Managed() {
		return apperr.Wrap(skillstypes.ConflictManaged)
	}
	return apperr.Wrap(s.skills.Delete(ctx, id))
}

func (s *SkillsService) CountDownload(ctx context.Context, id uuid.UUID) error {
	return apperr.Wrap(s.skills.CountDownload(ctx, id))
}

func (s *SkillsService) ResolveSkill(ctx context.Context, p accesstypes.Principal, ref string) (skillstypes.Skill, []skillstypes.Skill, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	if id, err := uuid.Parse(ref); err == nil {
		sk, err := s.GetSkill(ctx, p, id)
		if err == nil {
			err = forAgent(ctx, s.parts, p, sk)
		}
		return sk, nil, err
	}
	name := ref[strings.LastIndex(ref, "/")+1:]
	page, err := s.ListSkills(ctx, p, ListInput{Name: name, Agent: true, Limit: 100})
	if err != nil {
		return skillstypes.Skill{}, nil, err
	}
	var found []skillstypes.Skill
	for _, sk := range page.Items {
		if !strings.Contains(ref, "/") || sk.Path == ref {
			found = append(found, sk)
		}
	}
	switch len(found) {
	case 0:
		return skillstypes.Skill{}, nil, apperr.New(apperr.NotFound)
	case 1:
		return found[0], nil, nil
	}
	return skillstypes.Skill{}, found, apperr.Wrap(skillstypes.Ambiguous)
}
