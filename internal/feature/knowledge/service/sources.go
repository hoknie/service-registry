package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

func (s *Service) GetKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID) (*knowledge.Source, error) {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil || src == nil {
		return nil, apperr.Wrap(err)
	}
	return &src.Source, nil
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

func (s *Service) prepareSource(id uuid.UUID, in knowledge.SourceInput) (knowledge.NewSource, error) {
	v, err := s.validSource(in)
	if err != nil {
		return knowledge.NewSource{}, err
	}
	n := knowledge.NewSource{Source: v.Source, CredentialsRef: v.Reference, Keep: v.Keep}
	if v.Token != nil {
		if !s.secrets.CanEncrypt() {
			return knowledge.NewSource{}, apperr.Wrap(forge.ConflictSecretsKeyMissing)
		}
		enc, err := s.secrets.Seal(*v.Token, secretbox.AAD(knowledge.SourceTable, id.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return knowledge.NewSource{}, apperr.Internalf("encrypt: %v", err)
		}
		fp := secretbox.Fingerprint(*v.Token)
		n.CredentialsEnc, n.Fingerprint = &enc, &fp
	}
	return n, nil
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
	n, err := s.prepareSource(id, in)
	if err != nil {
		return knowledge.Source{}, err
	}
	saved, err := s.sources.Put(ctx, id, n)
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
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
	case v.Token != nil:
		token = *v.Token
	case v.Reference != nil:
		t, err := s.sourceToken(id, &knowledge.StoredSource{CredentialsRef: v.Reference})
		if err != nil {
			code, _ := failureOf(err)
			return SourceCheck{ErrorCode: code}, nil
		}
		token = t
	case v.Keep:
		if stored, err := s.sources.Get(ctx, id); err == nil && stored != nil && stored.Kind == v.Kind && stored.URL == v.URL {
			t, err := s.sourceToken(id, stored)
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
