package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
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
	_, err := p.svc.sourcesService.saveSource(ctx, projectID, p.source)
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

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.legacySources(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		value := catalogtypes.SecretValue{Ref: row.Ref}
		if row.Ref == nil {
			v, err := s.secrets.Open(*row.Enc, secretbox.AAD(knowledgetypes.SourceTable, row.ProjectID.String(), knowledgetypes.SourceColumnCredentials))
			if err != nil {
				return 0, fmt.Errorf("documentation source %s: %w", row.ProjectID, err)
			}
			value.Value = &v
		}
		secret, err := adopt(ctx, catalogtypes.LegacyCredential{RecordKind: "source", RecordID: row.ProjectID, Scope: row.ParentID,
			Name: row.Slug, Value: value})
		if err != nil {
			return 0, fmt.Errorf("documentation source %s: %w", row.ProjectID, err)
		}
		if err := s.sourceRepository.AdoptSecret(ctx, row.ProjectID, secret); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *PortsService) LegacyCredentialCount(ctx context.Context) (int, error) {
	rows, err := s.legacySources(ctx)
	return len(rows), err
}

func (s *PortsService) legacySources(ctx context.Context) ([]knowledgetypes.LegacySource, error) {
	ok, err := s.sourceRepository.HasLegacyCredentials(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return s.sourceRepository.Legacy(ctx)
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

func (s *PortsService) TriggerCollection(ctx context.Context, projectID uuid.UUID, event string) error {
	settings, err := s.settingsService.effectiveSettings(ctx, projectID)
	if err != nil || settings.Refresh != knowledgetypes.RefreshEvent || !settings.Listens(knowledgetypes.CollectEvent(event)) {
		return err
	}
	return apperr.Wrap(s.collectionRepository.Trigger(ctx, projectID))
}

type PortsService struct {
	embedder             knowledgeports.EmbedderPort
	external             knowledgeports.ExternalIndexPort
	searchEngine         knowledgeports.EnginePort
	secrets              *secretbox.Box
	collectionRepository *repositories.CollectionRepository
	sourceRepository     *repositories.SourceRepository
	sourcesService       *SourcesService
	settingsService      *SettingsService
}
