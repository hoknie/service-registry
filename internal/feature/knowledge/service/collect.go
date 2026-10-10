package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/feature/knowledge/repository"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *Service) ClaimKnowledge(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.settings.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *Service) ExtendKnowledge(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.settings.Extend(ctx, id, leaseSecs))
}

type head struct{ name, sha string }

func (s *Service) recordScan(ctx context.Context, scan *knowledge.Scan) {
	if ctx.Err() != nil || s.scans == nil {
		return
	}
	scan.FinishedAt = time.Now()
	scan.Settle()
	interval := s.cfg.Knowledge.IntervalSecs
	if scan.Kind == knowledge.IndexScan {
		interval = s.cfg.Search.IndexIntervalSecs
	}
	maxGap := 2 * time.Duration(interval) * time.Second
	if err := s.scans.Record(context.WithoutCancel(ctx), *scan, int(s.cfg.Knowledge.ScanHistory), maxGap); err != nil {
		slog.Warn("documentation scan not recorded", "project", scan.ProjectID, "kind", scan.Kind, "error", err)
	}
}

func (s *Service) RunKnowledgeCollect(ctx context.Context, id uuid.UUID) error {
	cfg := s.cfg.Knowledge
	next := cfg.IntervalSecs
	release := func() error {
		if ctx.Err() != nil {
			return nil
		}
		return apperr.Wrap(s.settings.Release(context.WithoutCancel(ctx), id, next, true))
	}
	scan := &knowledge.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledge.CollectScan,
		Trigger: knowledge.TriggerSchedule, StartedAt: time.Now()}
	p, err := s.settings.Project(ctx, id)
	if errors.Is(err, knowledge.ErrNotFound) {
		return nil
	}
	if err != nil {
		return apperr.Wrap(err)
	}
	if p.Force {
		scan.Trigger = knowledge.TriggerManual
	}
	defer s.recordScan(ctx, scan)
	if p.Settings, err = s.effectiveSettings(ctx, id); err != nil {
		return err
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	fail := func(branches []string, err error) error {
		if ctx.Err() != nil {
			return nil
		}
		code, reset := failureOf(err)
		scan.Fail(code, err)
		if reset != nil {
			if secs := time.Until(*reset).Seconds(); secs > 0 {
				next = uint32(secs) + 1
			}
		} else {
			next = max(next, cfg.RetrySecs)
		}
		for _, b := range branches {
			scan.Branches = append(scan.Branches, knowledge.ScanBranch{Name: b, Result: knowledge.BranchFailed, Error: code})
			if err := s.recordFailure(ctx, id, b, code); err != nil {
				return err
			}
		}
		return release()
	}

	var reader knowledge.Reader
	var heads []head
	def := p.DefaultBranch
	switch {
	case src != nil:
		scan.Source = string(src.Kind)
		known := knownBranches(src, def)
		token, err := s.sourceToken(ctx, id, src)
		if err != nil {
			return fail(known, err)
		}
		if reader, err = s.readers.Open(ctx, src.Source, p.Settings, token); err != nil {
			return fail(known, err)
		}
		shown, srcDef, err := reader.Heads(ctx)
		if err != nil {
			return fail(known, err)
		}
		if src.Kind == knowledge.SourceLocalDir {
			def = knowledge.LocalBranch
		} else if def == "" {
			def = srcDef
		}
		kept := map[string]string{}
		for name, sha := range shown {
			if p.Settings.CollectsBranch(name, def) {
				kept[name] = sha
				heads = append(heads, head{name, sha})
			}
		}
		var branches map[string]string
		if src.Kind != knowledge.SourceLocalDir {
			branches = make(map[string]string, len(shown))
			for name, sha := range shown {
				branches[name] = knowledge.CommitOf(sha)
			}
		}
		if err := s.setHeads(ctx, id, kept, srcDef, branches); err != nil {
			return apperr.Wrap(err)
		}
		if len(heads) == 0 {
			scan.Warn(knowledge.WarnNoBranches)
		}
	case p.Synced:
		scan.Source = "forge"
		repo, client, code := s.forge.ProjectClient(ctx, id)
		branches, err := s.snapshots.Branches(ctx, id)
		if err != nil {
			return apperr.Wrap(err)
		}
		for _, b := range branches {
			if !b.Gone && b.HeadSHA != "" && p.Settings.CollectsBranch(b.Name, def) {
				heads = append(heads, head{b.Name, b.HeadSHA})
			}
		}
		if code != "" {
			names := make([]string, 0, len(heads))
			for _, h := range heads {
				names = append(names, h.name)
			}
			return fail(names, knowledge.Failure(code))
		}
		reader = &syncedReader{client: client, repo: repo}
		if len(heads) == 0 {
			scan.Warn(knowledge.WarnNoBranches)
		}
	default:
		scan.Warn(knowledge.WarnNoSource)
		return release()
	}
	slices.SortFunc(heads, func(a, b head) int { return compareNames(a.name, b.name) })

	for _, h := range heads {
		if ctx.Err() != nil {
			return nil
		}
		if !p.Force {
			last, err := s.snapshots.LastAttempt(ctx, id, h.name)
			if err != nil {
				return apperr.Wrap(err)
			}
			if last != nil && last.Commit == h.sha && last.Status != knowledge.StatusFailed {
				scan.Branches = append(scan.Branches, knowledge.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledge.BranchUnchanged})
				continue
			}
		}
		if err := s.collectBranch(ctx, p, reader, h, scan); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			code, reset := failureOf(err)
			scan.Fail(code, err)
			scan.Branches = append(scan.Branches, knowledge.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledge.BranchFailed, Error: code})
			slog.Info("documentation collection failed", "project", id, "branch", h.name, "code", code)
			if err := s.recordFailure(ctx, id, h.name, code, h.sha); err != nil {
				return err
			}
			if reset != nil {
				if secs := time.Until(*reset).Seconds(); secs > 0 {
					next = uint32(secs) + 1
				}
			} else {
				next = max(next, cfg.RetrySecs)
			}
		}
	}
	return release()
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

func knownBranches(src *knowledge.StoredSource, def string) []string {
	if src.Kind == knowledge.SourceLocalDir {
		return []string{knowledge.LocalBranch}
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

func (s *Service) recordFailure(ctx context.Context, id uuid.UUID, branch, code string, commit ...string) error {
	c := ""
	if len(commit) > 0 {
		c = commit[0]
	}
	snap := knowledge.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Branch: branch, Commit: c,
		Status: knowledge.StatusFailed, ErrorCode: code}
	return apperr.Wrap(s.snapshots.Save(context.WithoutCancel(ctx), snap, s.cfg.Knowledge.Keep))
}

func failureOf(err error) (string, *time.Time) {
	var f knowledge.Failure
	if errors.As(err, &f) {
		return string(f), nil
	}
	var limited *forge.RateLimited
	if errors.As(err, &limited) {
		if limited.Reset.IsZero() {
			return "forge.rate_limited", nil
		}
		reset := limited.Reset
		return "forge.rate_limited", &reset
	}
	if code := forge.FailureCode(err); code != "" && code != "forge.interrupted" {
		return code, nil
	}
	return "forge.upstream_error", nil
}

func (s *Service) sourceToken(ctx context.Context, id uuid.UUID, src *knowledge.StoredSource) (string, error) {
	switch {
	case src.CredentialsSecretID != nil:
		t, err := s.catalog.ResolveSecret(ctx, *src.CredentialsSecretID)
		if err != nil {
			return "", knowledge.Failure("forge.credentials_unavailable")
		}
		return t, nil
	case src.CredentialsEnc != nil:
		t, err := s.secrets.Open(*src.CredentialsEnc, secretbox.AAD(knowledge.SourceTable, id.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return "", knowledge.Failure("forge.credentials_unavailable")
		}
		return t, nil
	case src.CredentialsRef != nil:
		t, err := config.ResolveSecretRef(*src.CredentialsRef)
		if err != nil {
			return "", knowledge.Failure("forge.credentials_unavailable")
		}
		return t, nil
	}
	return "", nil
}

type syncedReader struct {
	client forge.Client
	repo   forge.RemoteRepo
}

func (r *syncedReader) Heads(context.Context) (map[string]string, string, error) {
	return nil, r.repo.DefaultBranch, nil
}

func (r *syncedReader) Tree(ctx context.Context, _, sha string) ([]knowledge.Entry, bool, error) {
	tree, truncated, err := r.client.ListTree(ctx, r.repo, sha)
	if err != nil {
		return nil, false, err
	}
	out := make([]knowledge.Entry, 0, len(tree))
	for _, e := range tree {
		out = append(out, knowledge.Entry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size})
	}
	return out, truncated, nil
}

func (r *syncedReader) Read(ctx context.Context, e knowledge.Entry, limit int64) ([]byte, error) {
	return r.client.ReadFile(ctx, r.repo, forge.TreeEntry{Path: e.Path, BlobSHA: e.BlobSHA, Size: e.Size}, limit)
}

func (s *Service) collectBranch(ctx context.Context, p knowledge.Project, reader knowledge.Reader, h head, scan *knowledge.Scan) error {
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
	known, err := s.snapshots.Known(ctx, p.ID, shas)
	if err != nil {
		slog.Warn("known documentation content not read", "project", p.ID, "error", err)
		known = map[string]knowledge.Known{}
	}
	limits := knowledge.Limits{MaxFileBytes: cfg.MaxFileBytes, MaxFiles: int(cfg.MaxFiles), MaxSnapshotBytes: cfg.MaxSnapshotBytes}
	files, err := knowledge.Collect(entries, p.Settings, limits, func(e knowledge.Entry, limit int64) ([]byte, error) {
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
	snap := knowledge.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: p.ID, Branch: h.name, Commit: h.sha,
		Status: knowledge.SnapshotStatus(files, truncated), Truncated: truncated, Files: files, FileIDs: ids}
	if err := s.snapshots.Save(ctx, snap, cfg.Keep); err != nil {
		slog.Warn("documentation snapshot not saved", "project", p.ID, "branch", h.name, "error", err)
		return knowledge.Failure("forge.upstream_error")
	}
	b := knowledge.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledge.BranchCollected, Skipped: map[string]int{}, Truncated: truncated,
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
		scan.Warn(knowledge.WarnNoFilesMatched)
	case len(b.Skipped) > 0:
		scan.Warn(knowledge.WarnFilesSkipped)
	}
	if truncated {
		scan.Warn(knowledge.WarnTruncated)
	}
	return nil
}

func (s *Service) PruneKnowledgeBlobs(ctx context.Context) (int64, error) {
	n, err := s.snapshots.PruneBlobs(ctx)
	return n, apperr.Wrap(err)
}

func (s *Service) setHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string,
	branches map[string]string) error {
	return repository.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.sources.SetHeads(ctx, projectID, heads, defaultBranch); err != nil {
			return err
		}
		if branches == nil {
			return nil
		}
		return s.catalog.SyncRepositoryBranches(ctx, projectID, branches, defaultBranch)
	}))
}
