package services

import (
	"context"

	"github.com/google/uuid"

	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
	}
	return s.collectionRepository.Signals(ctx, projects, s.embedder != nil || s.external != nil, model, pending, busy)
}

type preparedSource struct {
	svc    *PortsService
	source knowledgetypes.NewSource
}

func (p preparedSource) Repo() (*catalogtypes.Forge, *string, bool) {
	return sourceRepo(p.source.Source)
}

func (p preparedSource) Attach(ctx context.Context, projectID uuid.UUID) error {
	if err := p.svc.sourcesService.checkSecret(ctx, projectID, p.source.SecretID); err != nil {
		return err
	}
	_, err := p.svc.sourceRepository.Put(ctx, projectID, p.source)
	return err
}

func (s *PortsService) SourceFor(in knowledgetypes.SourceInput) catalogports.SourceFactoryPort {
	return func(projectID uuid.UUID) (catalogports.ProjectSourcePort, error) {
		n, err := s.sourcesService.prepareSource(context.Background(), projectID, in, false)
		if err != nil {
			return nil, err
		}
		return preparedSource{svc: s, source: n}, nil
	}
}

func (s *PortsService) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	stored, err := s.sourceRepository.Secrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(stored))
	for i, src := range stored {
		v, err := s.secrets.Open(src.CredentialsEnc, secretbox.AAD(knowledgetypes.SourceTable, src.ProjectID.String(), knowledgetypes.SourceColumnCredentials))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, src := range stored {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(knowledgetypes.SourceTable, src.ProjectID.String(), knowledgetypes.SourceColumnCredentials))
			if err != nil {
				return err
			}
			if err := s.sourceRepository.ReplaceSecret(ctx, src.ProjectID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(stored), nil
}

type IndexInfo struct {
	Engine   knowledgeports.EnginePort
	Model    string
	Enabled  bool
	External bool
}

func (s *PortsService) Indexing() IndexInfo {
	info := IndexInfo{Engine: s.searchEngine, Enabled: s.embedder != nil || s.external != nil, External: s.external != nil}
	if s.embedder != nil {
		info.Model = s.embedder.Model()
	}
	return info
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.sourceRepository.SecretUsage(ctx, secrets)
}

type PortsService struct {
	embedder             knowledgeports.EmbedderPort
	external             knowledgeports.ExternalIndexPort
	searchEngine         knowledgeports.EnginePort
	secrets              *secretbox.Box
	collectionRepository *repositories.CollectionRepository
	sourceRepository     *repositories.SourceRepository
	sourcesService       *SourcesService
}
