package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/feature/knowledge/internal/repository"
	"svc-registry/pkg/secretbox"
)

func (s *Service) ActivitySignals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
	}
	return s.settings.Signals(ctx, projects, s.embedder != nil || s.external != nil, model, pending, busy)
}

type preparedSource struct {
	sources *repository.Sources
	source  knowledge.NewSource
}

func (p preparedSource) Repo() (*catalog.Forge, *string, bool) { return sourceRepo(p.source.Source) }

func (p preparedSource) Attach(ctx context.Context, projectID uuid.UUID) error {
	_, err := p.sources.Put(ctx, projectID, p.source)
	return err
}

func (s *Service) SourceFor(in knowledge.SourceInput) catalog.SourceFactory {
	return func(projectID uuid.UUID) (catalog.ProjectSource, error) {
		n, err := s.prepareSource(projectID, in)
		if err != nil {
			return nil, err
		}
		return preparedSource{sources: s.sources, source: n}, nil
	}
}

func (s *Service) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	stored, err := s.sources.Secrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(stored))
	for i, src := range stored {
		v, err := s.secrets.Open(src.CredentialsEnc, secretbox.AAD(knowledge.SourceTable, src.ProjectID.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, src := range stored {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(knowledge.SourceTable, src.ProjectID.String(), knowledge.SourceColumnCredentials))
			if err != nil {
				return err
			}
			if err := s.sources.ReplaceSecret(ctx, src.ProjectID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(stored), nil
}

type IndexInfo struct {
	Engine   knowledge.Engine
	Model    string
	Enabled  bool
	External bool
}

func (s *Service) Indexing() IndexInfo {
	info := IndexInfo{Engine: s.searchEngine, Enabled: s.embedder != nil || s.external != nil, External: s.external != nil}
	if s.embedder != nil {
		info.Model = s.embedder.Model()
	}
	return info
}
