package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) GetKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID) (*knowledge.Source, error) {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil || src == nil {
		return nil, apperr.Wrap(err)
	}
	out, err := s.decorateSource(ctx, src.Source)
	return &out, err
}

func (s *Service) decorateSource(ctx context.Context, src knowledge.Source) (knowledge.Source, error) {
	if src.Credentials.SecretID == nil {
		return src, nil
	}
	refs, err := s.catalog.SecretRefs(ctx, []uuid.UUID{*src.Credentials.SecretID})
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if ref, ok := refs[*src.Credentials.SecretID]; ok {
		src.Credentials.Secret = &ref
	}
	return src, nil
}

func (s *Service) validSource(in knowledge.SourceInput) (knowledge.ValidSource, error) {
	v, err := knowledge.ValidateSource(in)
	if err != nil {
		return knowledge.ValidSource{}, apperr.Wrap(err)
	}
	if v.IsLocal() {
		if err := knowledge.LocalPathAllowed(s.cfg.Knowledge.LocalRoots, v.Path); err != nil {
			return knowledge.ValidSource{}, apperr.Wrap(err)
		}
	}
	return v, nil
}

func (s *Service) prepareSource(ctx context.Context, id uuid.UUID, in knowledge.SourceInput, checkSecret bool) (knowledge.NewSource, error) {
	v, err := s.validSource(in)
	if err != nil {
		return knowledge.NewSource{}, err
	}
	if checkSecret {
		if err := s.checkSecret(ctx, id, v.SecretID); err != nil {
			return knowledge.NewSource{}, err
		}
	}
	return knowledge.NewSource(v), nil
}

func (s *Service) checkSecret(ctx context.Context, projectID uuid.UUID, secret *uuid.UUID) error {
	if secret == nil {
		return nil
	}
	_, err := s.catalog.SecretFor(ctx, &projectID, *secret)
	return err
}

func sourceRepo(s knowledge.Source) (f *catalog.Forge, url *string, ok bool) {
	if s.Kind != knowledge.SourceRemote {
		return nil, nil, false
	}
	f, ferr := catalog.ValidateForge(s.Forge)
	url, uerr := catalog.ValidateRepoURL(s.URL)
	return f, url, ferr == nil && uerr == nil && f != nil && url != nil
}

func (s *Service) PutKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID, in knowledge.SourceInput) (knowledge.Source, error) {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return knowledge.Source{}, err
	}
	n, err := s.prepareSource(ctx, id, in, true)
	if err != nil {
		return knowledge.Source{}, err
	}
	saved, err := s.sources.Put(ctx, id, n)
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if saved, err = s.decorateSource(ctx, saved); err != nil {
		return knowledge.Source{}, err
	}
	f, url, ok := sourceRepo(saved)
	if !ok {
		return saved, nil
	}
	managed, err := s.forge.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if !managed[id] {
		changes := catalog.NodeChanges{Forge: catalog.Change[catalog.Forge]{Set: true, Value: f}, RepoURL: catalog.Change[string]{Set: true, Value: url}}
		if _, err := s.catalog.ChangeNode(ctx, id, changes); err != nil {
			return knowledge.Source{}, apperr.Wrap(err)
		}
	}
	return saved, nil
}

func (s *Service) DeleteKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return err
	}
	return apperr.Wrap(s.sources.Delete(ctx, id))
}

type SourceCheck struct {
	OK        bool
	Branches  int
	ErrorCode string
}

func (s *Service) CheckKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID, in knowledge.SourceInput) (SourceCheck, error) {
	kp, err := s.knowledgeProject(ctx, p, catalog.PermWrite, id)
	if err != nil {
		return SourceCheck{}, err
	}
	v, err := s.validSource(in)
	if err != nil {
		return SourceCheck{}, err
	}
	token := ""
	switch {
	case v.SecretID != nil:
		if err := s.checkSecret(ctx, id, v.SecretID); err != nil {
			return SourceCheck{}, err
		}
		t, err := s.sourceToken(ctx, id, &knowledge.StoredSource{CredentialsSecretID: v.SecretID})
		if err != nil {
			code, _ := failureOf(err)
			return SourceCheck{ErrorCode: code}, nil
		}
		token = t
	case v.Keep:
		if stored, err := s.sources.Get(ctx, id); err == nil && stored != nil && stored.Kind == v.Kind && stored.URL == v.URL {
			t, err := s.sourceToken(ctx, id, stored)
			if err != nil {
				code, _ := failureOf(err)
				return SourceCheck{ErrorCode: code}, nil
			}
			token = t
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Outbound.TimeoutSecs)*time.Second)
	defer cancel()
	reader, err := s.readers.Open(ctx, v.Source, kp.Settings, token)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	heads, _, err := reader.Heads(ctx)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	return SourceCheck{OK: true, Branches: len(heads)}, nil
}
