package services

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeservices "svc-registry/internal/feature/forge/services"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

func (s *CollectService) ClaimKnowledge(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.collectionRepository.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *CollectService) ExtendKnowledge(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.collectionRepository.Extend(ctx, id, leaseSecs))
}

type head struct{ name, sha string }

func (s *CollectService) recordScan(ctx context.Context, scan *knowledgetypes.Scan) {
	if ctx.Err() != nil || s.scanRepository == nil {
		return
	}
	scan.FinishedAt = time.Now()
	scan.Settle()
	interval := s.cfg.Knowledge.IntervalSecs
	if scan.Kind == knowledgetypes.IndexScan {
		interval = s.cfg.Search.IndexIntervalSecs
	}
	maxGap := 2 * time.Duration(interval) * time.Second
	if err := s.scanRepository.Record(context.WithoutCancel(ctx), *scan, int(s.cfg.Knowledge.ScanHistory), maxGap); err != nil {
		slog.Warn("documentation scan not recorded", "project", scan.ProjectID, "kind", scan.Kind, "error", err)
	}
}

func (s *CollectService) RunKnowledgeCollect(ctx context.Context, id uuid.UUID) error {
	cfg := s.cfg.Knowledge
	run := &collectRun{s: s, id: id, next: cfg.IntervalSecs, retry: cfg.RetrySecs,
		scan: &knowledgetypes.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledgetypes.CollectScan,
			Trigger: knowledgetypes.TriggerSchedule, StartedAt: time.Now()}}
	p, err := s.collectionRepository.Project(ctx, id)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil
	}
	if err != nil {
		return apperr.Wrap(err)
	}
	if p.Force {
		run.scan.Trigger = knowledgetypes.TriggerManual
	}
	defer s.recordScan(ctx, run.scan)
	if p.Settings, err = s.settingsService.effectiveSettings(ctx, id); err != nil {
		return err
	}
	run.project = p
	src, err := s.sourceRepository.Get(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	var reader knowledgeports.ReaderPort
	var heads []head
	switch {
	case src != nil:
		reader, heads, err = run.sourceHeads(ctx, src)
	case p.Synced:
		reader, heads, err = run.forgeHeads(ctx)
	default:
		run.scan.Warn(knowledgetypes.WarnNoSource)
		return run.release(ctx)
	}
	if err != nil || reader == nil {
		return err
	}
	slices.SortFunc(heads, func(a, b head) int { return compareNames(a.name, b.name) })
	return run.collect(ctx, reader, heads)
}

func compareNames(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func knownBranches(src *knowledgetypes.StoredSource, def string) []string {
	if src.Kind == knowledgetypes.SourceLocalDir {
		return []string{knowledgetypes.LocalBranch}
	}
	var out []string
	for name := range src.Heads {
		out = append(out, name)
	}
	if len(out) == 0 {
		switch {
		case def != "":
			out = []string{def}
		case src.DefaultBranch != "":
			out = []string{src.DefaultBranch}
		}
	}
	slices.Sort(out)
	return out
}

func (s *CollectService) recordFailure(ctx context.Context, id uuid.UUID, branch, code string, commit ...string) error {
	c := ""
	if len(commit) > 0 {
		c = commit[0]
	}
	snap := knowledgetypes.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Branch: branch, Commit: c,
		Status: knowledgetypes.StatusFailed, ErrorCode: code}
	return apperr.Wrap(s.snapshotRepository.Save(context.WithoutCancel(ctx), snap, s.cfg.Knowledge.Keep))
}

func failureOf(err error) (string, *time.Time) {
	var f knowledgetypes.Failure
	if errors.As(err, &f) {
		return string(f), nil
	}
	var limited *forgetypes.RateLimited
	if errors.As(err, &limited) {
		if limited.Reset.IsZero() {
			return "forge.rate_limited", nil
		}
		reset := limited.Reset
		return "forge.rate_limited", &reset
	}
	if code := forgetypes.FailureCode(err); code != "" && code != "forge.interrupted" {
		return code, nil
	}
	return "forge.upstream_error", nil
}

type CollectService struct {
	readers              knowledgeports.ReaderFactoryPort
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	db                   *postgres.DB
	forge                *forgeservices.ForgeServices
	scanRepository       *repositories.ScanRepository
	secrets              *secretbox.Box
	collectionRepository *repositories.CollectionRepository
	snapshotRepository   *repositories.SnapshotRepository
	sourceRepoRepository *repositories.SourceRepoRepository
	sourceRepository     *repositories.SourceRepository
	documents            *DocumentsService
	settingsService      *SettingsService
}

func (s *CollectService) CollectKnowledge(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	kp, err := s.documents.knowledgeProject(ctx, p, catalogtypes.PermWrite, id)
	if err != nil {
		return err
	}
	d, err := s.documents.documentationOf(ctx, kp)
	if err != nil {
		return err
	}
	if d.kind() == "" {
		return apperr.Wrap(knowledgetypes.ConflictNotSynced)
	}
	return apperr.Wrap(s.collectionRepository.RequestCollect(ctx, id))
}
