package services

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	forgeports "svc-registry/internal/feature/forge/ports"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *CollectService) sourceToken(ctx context.Context, id uuid.UUID, src *knowledgetypes.StoredSource) (string, error) {
	switch {
	case src.CredentialsSecretID != nil:
		t, err := s.catalog.SecretResolver.ResolveSecret(ctx, *src.CredentialsSecretID)
		if err != nil {
			return "", knowledgetypes.Failure("forge.credentials_unavailable")
		}
		return t, nil
	case src.CredentialsEnc != nil:
		t, err := s.secrets.Open(*src.CredentialsEnc, secretbox.AAD(knowledgetypes.SourceTable, id.String(), knowledgetypes.SourceColumnCredentials))
		if err != nil {
			return "", knowledgetypes.Failure("forge.credentials_unavailable")
		}
		return t, nil
	case src.CredentialsRef != nil:
		t, err := config.ResolveSecretRef(*src.CredentialsRef)
		if err != nil {
			return "", knowledgetypes.Failure("forge.credentials_unavailable")
		}
		return t, nil
	}
	return "", nil
}

type syncedReader struct {
	client forgeports.ClientPort
	repo   forgetypes.RemoteRepo
}

func (r *syncedReader) Heads(context.Context) (map[string]string, string, error) {
	return nil, r.repo.DefaultBranch, nil
}

func (r *syncedReader) Tree(ctx context.Context, _, sha string) ([]knowledgetypes.Entry, bool, error) {
	tree, truncated, err := r.client.ListTree(ctx, r.repo, sha)
	if err != nil {
		return nil, false, err
	}
	out := make([]knowledgetypes.Entry, 0, len(tree))
	for _, e := range tree {
		out = append(out, knowledgetypes.Entry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size})
	}
	return out, truncated, nil
}

func (r *syncedReader) Read(ctx context.Context, e knowledgetypes.Entry, limit int64) ([]byte, error) {
	return r.client.ReadFile(ctx, r.repo, forgetypes.TreeEntry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size}, limit)
}

func (s *CollectService) collectBranch(ctx context.Context, p knowledgetypes.Project, reader knowledgeports.ReaderPort, h head, scan *knowledgetypes.Scan) error {
	cfg := s.cfg.Knowledge
	entries, truncated, err := reader.Tree(ctx, h.name, h.sha)
	if err != nil {
		return err
	}
	var shas []string
	for _, e := range entries {
		if p.Settings.Collects(e.Path) {
			shas = append(shas, e.BlobSHA)
		}
	}
	known, err := s.snapshotRepository.Known(ctx, p.ID, shas)
	if err != nil {
		slog.Warn("known documentation content not read", "project", p.ID, "error", err)
		known = map[string]knowledgetypes.Known{}
	}
	limits := knowledgetypes.Limits{MaxFileBytes: cfg.MaxFileBytes, MaxFiles: int(cfg.MaxFiles), MaxSnapshotBytes: cfg.MaxSnapshotBytes}
	files, err := knowledgetypes.Collect(entries, p.Settings, limits, func(e knowledgetypes.Entry, limit int64) ([]byte, error) {
		if k, ok := known[e.BlobSHA]; ok {
			return k.Content, nil
		}
		return reader.Read(ctx, e, limit)
	})
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(files))
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	snap := knowledgetypes.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: p.ID, Branch: h.name, Commit: h.sha,
		Status: knowledgetypes.SnapshotStatus(files, truncated), Truncated: truncated, Files: files, FileIDs: ids}
	if err := s.snapshotRepository.Save(ctx, snap, cfg.Keep); err != nil {
		slog.Warn("documentation snapshot not saved", "project", p.ID, "branch", h.name, "error", err)
		return knowledgetypes.Failure("forge.upstream_error")
	}
	b := knowledgetypes.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledgetypes.BranchCollected, Skipped: map[string]int{}, Truncated: truncated,
		WorkingTree: strings.Contains(h.sha, "+worktree:")}
	for _, f := range files {
		if f.Skip == "" {
			b.Files++
		} else {
			b.Skipped[string(f.Skip)]++
		}
	}
	scan.Branches = append(scan.Branches, b)
	switch {
	case len(files) == 0 && len(entries) > 0:
		scan.Warn(knowledgetypes.WarnNoFilesMatched)
	case len(b.Skipped) > 0:
		scan.Warn(knowledgetypes.WarnFilesSkipped)
	}
	if truncated {
		scan.Warn(knowledgetypes.WarnTruncated)
	}
	return nil
}

func (s *CollectService) fillDefaultBranch(ctx context.Context, id uuid.UUID, name string) error {
	if name == "" {
		return nil
	}
	managed, err := s.forge.Ports.Managed(ctx, []uuid.UUID{id})
	if err != nil || managed[id] {
		return err
	}
	return s.catalog.BranchRecords.SetDefaultBranchIfEmpty(ctx, id, name)
}

func (s *CollectService) recordRepository(ctx context.Context, id uuid.UUID, reader knowledgeports.ReaderPort) error {
	meta, ok := reader.(knowledgeports.RepoMetadataPort)
	if !ok {
		return nil
	}
	known, err := s.sourceRepoRepository.ChangedAt(ctx, id)
	if err != nil {
		return err
	}
	repo, details, err := meta.Metadata(ctx, known)
	if err != nil {
		code, _ := failureOf(err)
		slog.Warn("repository metadata unavailable", "project", id, "code", code)
		if repo.FullPath == "" {
			return nil
		}
	}
	return s.sourceRepoRepository.Upsert(ctx, id, repo, details)
}

func (s *CollectService) setHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string,
	branches map[string]string) error {
	return repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.sourceRepository.SetHeads(ctx, projectID, heads, defaultBranch); err != nil {
			return err
		}
		if branches == nil {
			return nil
		}
		return s.catalog.BranchRecords.SyncRepositoryBranches(ctx, projectID, branches, defaultBranch)
	}))
}

func (s *CollectService) PruneKnowledgeBlobs(ctx context.Context) (int64, error) {
	n, err := s.snapshotRepository.PruneBlobs(ctx)
	return n, apperr.Wrap(err)
}
