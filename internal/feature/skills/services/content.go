package services

import (
	"context"
	"encoding/hex"
	"errors"
	"io"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/feature/skills/usecases"
	"svc-registry/internal/platform/apperr"
)

type ContentService struct {
	parts
	cards  *cardBuilder
	limits skillstypes.Limits
}

type Upload struct {
	Archive []byte
	Files   []skillstypes.Entry
	Version string
}

func (s *ContentService) unpack(in Upload) (skillstypes.Content, error) {
	if s.store == nil {
		return skillstypes.Content{}, apperr.Wrap(skillstypes.ConflictStorageOff)
	}
	if in.Archive != nil {
		c, err := skillstypes.UnpackZip(in.Archive, s.limits)
		return c, apperr.Wrap(err)
	}
	c, err := skillstypes.Build(in.Files, s.limits)
	return c, apperr.Wrap(err)
}

type CreateInput struct {
	Upload
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
	Status         string
}

func (s *ContentService) CreateSkill(ctx context.Context, p accesstypes.Principal, in CreateInput) (skillstypes.Skill, error) {
	status := skillstypes.StatusDraft
	if in.Status != "" {
		st, ok := skillstypes.ParseStatus(in.Status)
		if !ok {
			return skillstypes.Skill{}, apperr.Wrap(skillstypes.InvalidSkill)
		}
		status = st
	}
	if in.FolderID != nil {
		in.OrganizationID = nil
	}
	scope, err := placeOf(ctx, s.parts, p, in.FolderID, in.OrganizationID, accesstypes.ActionCreate)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	content, err := s.unpack(in.Upload)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	author, err := s.authorOf(ctx, p)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	_, err = s.save.Run(ctx, usecases.SaveInput{Content: content, FormVersion: in.Version, Origin: skillstypes.OriginUpload,
		CreatedBy: &p.UserID, Author: author, New: &repositories.NewSkill{ID: id, FolderID: in.FolderID,
			OrganizationID: scope.OrganizationID, Slug: content.Manifest.Name, Status: status, Tags: content.Manifest.Tags,
			CreatedBy: &p.UserID}})
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	return s.view(ctx, p, id)
}

func (s *ContentService) authorOf(ctx context.Context, p accesstypes.Principal) (string, error) {
	name, err := s.skills.UserName(ctx, p.UserID)
	return name, apperr.Wrap(err)
}

func (s *ContentService) view(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Skill, error) {
	sk, err := readSkill(ctx, s.parts, p, id)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	return s.cards.one(ctx, sk)
}

func (s *ContentService) UploadVersion(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in Upload) (skillstypes.Skill, error) {
	sk, err := writeSkill(ctx, s.parts, p, id, accesstypes.ActionUpdate)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	if sk.Managed() {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.ConflictManaged)
	}
	content, err := s.unpack(in)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	author, err := s.authorOf(ctx, p)
	if err != nil {
		return skillstypes.Skill{}, err
	}
	_, err = s.save.Run(ctx, usecases.SaveInput{SkillID: id, Content: content, FormVersion: in.Version,
		Origin: skillstypes.OriginUpload, CreatedBy: &p.UserID, Author: author, Policy: s.policyOf(ctx, sk)})
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	return s.view(ctx, p, id)
}

func (s *ContentService) policyOf(ctx context.Context, sk skillstypes.Skill) func(uuid.UUID) skillstypes.Effective {
	return func(uuid.UUID) skillstypes.Effective {
		pol, err := s.cards.policies(ctx, []skillstypes.Skill{sk})
		if err != nil {
			return skillstypes.Merge(nil, nil)
		}
		return pol.effective(sk)
	}
}

type Selected struct {
	Skill   skillstypes.Skill
	Version skillstypes.Version
	Files   []skillstypes.File
}

func (s *ContentService) selectVersion(ctx context.Context, p accesstypes.Principal, id uuid.UUID, selector string) (Selected, error) {
	return s.selectFor(ctx, p, id, selector, false)
}

func (s *ContentService) selectFor(ctx context.Context, p accesstypes.Principal, id uuid.UUID, selector string, agent bool) (Selected, error) {
	sk, err := s.view(ctx, p, id)
	if err != nil {
		return Selected{}, err
	}
	if agent {
		if sk.Status != skillstypes.StatusPublished {
			return Selected{}, apperr.New(apperr.NotFound)
		}
		sk.CanWrite = false
	}
	v, err := s.cards.pick(sk, selector)
	if err != nil {
		return Selected{}, err
	}
	files, err := s.versions.Files(ctx, v.ID)
	if err != nil {
		return Selected{}, apperr.Wrap(err)
	}
	return Selected{Skill: sk, Version: v, Files: files}, nil
}

func (s *ContentService) SkillFiles(ctx context.Context, p accesstypes.Principal, id uuid.UUID, version string) (Selected, error) {
	return s.selectVersion(ctx, p, id, version)
}

func findFile(files []skillstypes.File, path string) (skillstypes.File, error) {
	for _, f := range files {
		if f.Path == path {
			return f, nil
		}
	}
	return skillstypes.File{}, apperr.New(apperr.NotFound)
}

func (s *ContentService) open(ctx context.Context, sum []byte) (io.ReadCloser, error) {
	if s.store == nil {
		return nil, apperr.Wrap(skillstypes.StorageUnavailable)
	}
	rc, err := s.store.Open(ctx, hex.EncodeToString(sum))
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, apperr.Wrap(skillstypes.StorageUnavailable)
	}
	return rc, apperr.Wrap(err)
}

func (s *ContentService) read(ctx context.Context, f skillstypes.File) ([]byte, error) {
	rc, err := s.open(ctx, f.SHA256)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, f.Size+1))
	if err != nil {
		return nil, apperr.Wrap(skillstypes.StorageUnavailable)
	}
	return data, nil
}

func (s *ContentService) SkillFile(ctx context.Context, p accesstypes.Principal, id uuid.UUID, path, version string) (skillstypes.FileContent, error) {
	sel, err := s.selectVersion(ctx, p, id, version)
	if err != nil {
		return skillstypes.FileContent{}, err
	}
	f, err := findFile(sel.Files, path)
	if err != nil {
		return skillstypes.FileContent{}, err
	}
	out := skillstypes.FileContent{File: f, Version: sel.Version.Version}
	if f.Kind == skillstypes.KindMarkdown || f.Kind == skillstypes.KindText {
		data, err := s.read(ctx, f)
		if err != nil {
			return skillstypes.FileContent{}, err
		}
		out.Content = new(string(data))
	}
	return out, nil
}

func (s *ContentService) RawFile(ctx context.Context, p accesstypes.Principal, id uuid.UUID, path, version string) (skillstypes.File, []byte, error) {
	sel, err := s.selectVersion(ctx, p, id, version)
	if err != nil {
		return skillstypes.File{}, nil, err
	}
	f, err := findFile(sel.Files, path)
	if err != nil {
		return skillstypes.File{}, nil, err
	}
	data, err := s.read(ctx, f)
	return f, data, err
}

type ArchiveFile struct {
	Path string
	Data []byte
}

func (s *ContentService) Archive(ctx context.Context, p accesstypes.Principal, id uuid.UUID, version string, count bool) (Selected, []ArchiveFile, error) {
	sel, err := s.selectVersion(ctx, p, id, version)
	if err != nil {
		return Selected{}, nil, err
	}
	out := make([]ArchiveFile, 0, len(sel.Files))
	for _, f := range sel.Files {
		data, err := s.read(ctx, f)
		if err != nil {
			return Selected{}, nil, err
		}
		out = append(out, ArchiveFile{Path: f.Path, Data: data})
	}
	if count {
		if err := s.skills.CountDownload(ctx, id); err != nil {
			return Selected{}, nil, apperr.Wrap(err)
		}
	}
	return sel, out, nil
}

func (s *ContentService) AgentContent(ctx context.Context, p accesstypes.Principal, id uuid.UUID, selector string) (Selected, []ArchiveFile, error) {
	sel, err := s.selectFor(ctx, p, id, selector, true)
	if err != nil {
		return Selected{}, nil, err
	}
	out := make([]ArchiveFile, 0, len(sel.Files))
	for _, f := range sel.Files {
		if f.Kind != skillstypes.KindMarkdown && f.Kind != skillstypes.KindText {
			continue
		}
		data, err := s.read(ctx, f)
		if err != nil {
			return Selected{}, nil, err
		}
		out = append(out, ArchiveFile{Path: f.Path, Data: data})
	}
	return sel, out, apperr.Wrap(s.skills.CountDownload(ctx, id))
}

func (s *ContentService) AgentFile(ctx context.Context, p accesstypes.Principal, id uuid.UUID, path, selector string) (skillstypes.File, []byte, error) {
	sel, err := s.selectFor(ctx, p, id, selector, true)
	if err != nil {
		return skillstypes.File{}, nil, err
	}
	f, err := findFile(sel.Files, path)
	if err != nil {
		return skillstypes.File{}, nil, err
	}
	data, err := s.read(ctx, f)
	return f, data, err
}
