package services

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"

	forgeports "svc-registry/internal/feature/forge/ports"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/feature/knowledge/usecases"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
)

func (s *CollectService) sourceToken(ctx context.Context, id uuid.UUID, src *knowledgetypes.StoredSource) (string, error) {
	switch {
	case src.CredentialsSecretID != nil:
		t, err := s.catalog.SecretResolver.ResolveSecret(ctx, *src.CredentialsSecretID)
		if err != nil {
			return "", knowledgetypes.Failure("forge.credentials_unavailable")
		}
		return t, nil
	}
	return "", nil
}

type syncedReader struct {
	client   forgeports.ClientPort
	repo     forgetypes.RemoteRepo
	settings knowledgetypes.Settings
	maxTags  int
}

func (r *syncedReader) Heads(context.Context) (map[string]string, string, error) {
	return nil, r.repo.DefaultBranch, nil
}

func (r *syncedReader) Tags(ctx context.Context) (map[string]string, error) {
	tags, _, err := r.client.ListTags(ctx, r.repo, r.settings.CollectsTag, r.maxTags)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Commit != "" {
			out[t.Name] = t.Commit
		}
	}
	return out, nil
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

func (s *CollectService) readManifest(ctx context.Context, id uuid.UUID, reader knowledgeports.ReaderPort, entries []knowledgetypes.Entry, scan *knowledgetypes.Scan,
	branch string) ([]topologytypes.Ref, bool) {
	var found *knowledgetypes.Entry
	for _, name := range topologytypes.ManifestNames {
		if i := slices.IndexFunc(entries, func(e knowledgetypes.Entry) bool { return e.Path == name }); i >= 0 {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		return nil, true
	}
	problem := ""
	var m topologytypes.Manifest
	data, err := reader.Read(ctx, *found, topologytypes.MaxManifestBytes+1)
	if err == nil {
		m, err = topologytypes.ParseManifest(data)
	}
	switch {
	case found.Size > topologytypes.MaxManifestBytes:
		problem = "the file is larger than 64 KiB"
	case err != nil:
		problem = err.Error()
	case m.Problem != "":
		problem = m.Problem
	}
	if problem != "" {
		scan.Note(knowledgetypes.ScanNote{Code: knowledgetypes.WarnManifestInvalid, Branch: branch, Path: found.Path, Detail: problem})
		slog.Info("project manifest not fully read", "project", id, "path", found.Path, "problem", problem)
	}
	return m.Refs, err == nil && found.Size <= topologytypes.MaxManifestBytes
}

func (s *CollectService) collectBranch(ctx context.Context, p knowledgetypes.Project, reader knowledgeports.ReaderPort, h head, scan *knowledgetypes.Scan, isDefault bool) error {
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
	snap := knowledgetypes.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: p.ID, Branch: h.name, RefKind: h.kind, CommitSHA: h.sha,
		Status: knowledgetypes.SnapshotStatus(files, truncated), Truncated: truncated, Files: files, FileIDs: ids}
	var refs []topologytypes.Ref
	manifest := false
	if isDefault {
		refs, manifest = s.readManifest(ctx, p.ID, reader, entries, scan, h.name)
	}
	if err := s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.saveSnapshot.Run(ctx, usecases.SaveSnapshotInput{Snapshot: snap, Keep: cfg.Keep}); err != nil {
			return err
		}
		if !manifest {
			return nil
		}
		return s.topology.Records.ReplaceRepository(ctx, p.ID, h.name, refs)
	}); err != nil {
		slog.Warn("documentation snapshot not saved", "project", p.ID, "branch", h.name, "error", err)
		return knowledgetypes.Failure("forge.upstream_error")
	}
	b := knowledgetypes.ScanBranch{Name: h.name, Kind: h.kind, Commit: h.sha, Result: knowledgetypes.BranchCollected, Skipped: map[string]int{}, Truncated: truncated,
		WorkingTree: strings.Contains(h.sha, "+worktree:")}
	for _, f := range files {
		if f.Skip == "" {
			b.Files++
		} else {
			b.Skipped[string(f.Skip)]++
			if p.Settings.APIs && p.Settings.IsAPIPath(f.Path) {
				b.APISkipped++
			}
		}
	}
	scan.Branches = append(scan.Branches, b)
	if len(files) == 0 && len(entries) > 0 {
		scan.Note(knowledgetypes.ScanNote{Code: knowledgetypes.WarnNoFilesMatched, Branch: h.name})
	}
	for _, f := range files {
		if f.Skip != "" {
			scan.Note(knowledgetypes.ScanNote{Code: knowledgetypes.WarnFilesSkipped, Branch: h.name, Path: f.Path, Reason: f.Skip})
		}
	}
	if truncated {
		scan.Note(knowledgetypes.ScanNote{Code: knowledgetypes.WarnTruncated, Branch: h.name})
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
	return s.sourceRepoRepository.Upsert(ctx, id, knowledgetypes.NewSourceRepoRecord(repo, details))
}

func (s *CollectService) setHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string,
	branches map[string]string) error {
	if heads == nil {
		heads = map[string]string{}
	}
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
