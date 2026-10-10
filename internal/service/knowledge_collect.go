package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/config"
	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
	"svc-registry/pkg/secretbox"
)

func ClaimKnowledge(ctx context.Context, state *State, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := state.Knowledge.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func ExtendKnowledge(ctx context.Context, state *State, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(state.Knowledge.Extend(ctx, id, leaseSecs))
}

type head struct{ name, sha string }

func recordScan(ctx context.Context, state *State, scan *knowledge.Scan) {
	if ctx.Err() != nil || state.Scans == nil {
		return
	}
	scan.FinishedAt = time.Now()
	scan.Settle()
	interval := state.Config.Knowledge.IntervalSecs
	if scan.Kind == knowledge.IndexScan {
		interval = state.Config.Search.IndexIntervalSecs
	}
	maxGap := 2 * time.Duration(interval) * time.Second
	if err := state.Scans.Record(context.WithoutCancel(ctx), *scan, int(state.Config.Knowledge.ScanHistory), maxGap); err != nil {
		slog.Warn("documentation scan not recorded", "project", scan.ProjectID, "kind", scan.Kind, "error", err)
	}
}

func RunKnowledgeCollect(ctx context.Context, state *State, id uuid.UUID) error {
	cfg := state.Config.Knowledge
	next := cfg.IntervalSecs
	release := func() error {
		if ctx.Err() != nil {
			return nil
		}
		return apperr.Wrap(state.Knowledge.Release(context.WithoutCancel(ctx), id, next, true))
	}
	scan := &knowledge.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledge.CollectScan,
		Trigger: knowledge.TriggerSchedule, StartedAt: time.Now()}
	p, err := state.Knowledge.Project(ctx, id)
	if errors.Is(err, knowledge.ErrNotFound) {
		return nil
	}
	if err != nil {
		return apperr.Wrap(err)
	}
	if p.Force {
		scan.Trigger = knowledge.TriggerManual
	}
	defer recordScan(ctx, state, scan)
	if p.Settings, err = effectiveSettings(ctx, state, id); err != nil {
		return err
	}
	src, err := state.Sources.Get(ctx, id)
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
			if err := recordFailure(ctx, state, id, b, code); err != nil {
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
		token, err := sourceToken(state, id, src)
		if err != nil {
			return fail(known, err)
		}
		if reader, err = state.DocReaders.Open(ctx, src.Source, p.Settings, token); err != nil {
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
		if err := state.Sources.SetHeads(ctx, id, kept, srcDef, branches); err != nil {
			return apperr.Wrap(err)
		}
		if len(heads) == 0 {
			scan.Warn(knowledge.WarnNoBranches)
		}
	case p.Synced:
		scan.Source = "forge"
		repo, client, code := projectClient(ctx, state, id)
		branches, err := state.Snapshots.Branches(ctx, id)
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
			last, err := state.Snapshots.LastAttempt(ctx, id, h.name)
			if err != nil {
				return apperr.Wrap(err)
			}
			if last != nil && last.Commit == h.sha && last.Status != knowledge.StatusFailed {
				scan.Branches = append(scan.Branches, knowledge.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledge.BranchUnchanged})
				continue
			}
		}
		if err := collectBranch(ctx, state, p, reader, h, scan); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			code, reset := failureOf(err)
			scan.Fail(code, err)
			scan.Branches = append(scan.Branches, knowledge.ScanBranch{Name: h.name, Commit: h.sha, Result: knowledge.BranchFailed, Error: code})
			slog.Info("documentation collection failed", "project", id, "branch", h.name, "code", code)
			if err := recordFailure(ctx, state, id, h.name, code, h.sha); err != nil {
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

func recordFailure(ctx context.Context, state *State, id uuid.UUID, branch, code string, commit ...string) error {
	c := ""
	if len(commit) > 0 {
		c = commit[0]
	}
	snap := knowledge.NewSnapshot{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Branch: branch, Commit: c,
		Status: knowledge.StatusFailed, ErrorCode: code}
	return apperr.Wrap(state.Snapshots.Save(context.WithoutCancel(ctx), snap, state.Config.Knowledge.Keep))
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

func sourceToken(state *State, id uuid.UUID, src *knowledge.StoredSource) (string, error) {
	switch {
	case src.CredentialsEnc != nil:
		t, err := state.Secrets.Open(*src.CredentialsEnc, secretbox.AAD(knowledge.SourceTable, id.String(), knowledge.SourceColumnCredentials))
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

func projectClient(ctx context.Context, state *State, id uuid.UUID) (forge.RemoteRepo, forge.Client, string) {
	r, err := state.Repositories.Find(ctx, id)
	if err != nil || r == nil {
		return forge.RemoteRepo{}, nil, "forge.not_found"
	}
	conn, err := state.Connections.Find(ctx, r.ConnectionID)
	if err != nil || conn == nil {
		return forge.RemoteRepo{}, nil, "forge.credentials_unavailable"
	}
	client, err := clientFor(ctx, state, *conn)
	if err != nil {
		if code := forge.FailureCode(err); code != "" {
			return forge.RemoteRepo{}, nil, code
		}
		return forge.RemoteRepo{}, nil, "forge.credentials_unavailable"
	}
	repo := forge.RemoteRepo{ExternalID: r.ExternalID, FullPath: r.FullPath}
	if r.DefaultBranch != nil {
		repo.DefaultBranch = *r.DefaultBranch
	}
	return repo, client, ""
}

func collectBranch(ctx context.Context, state *State, p knowledge.Project, reader knowledge.Reader, h head, scan *knowledge.Scan) error {
	cfg := state.Config.Knowledge
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
	known, err := state.Snapshots.Known(ctx, p.ID, shas)
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
	if err := state.Snapshots.Save(ctx, snap, cfg.Keep); err != nil {
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

func PruneKnowledgeBlobs(ctx context.Context, state *State) (int64, error) {
	n, err := state.Snapshots.PruneBlobs(ctx)
	return n, apperr.Wrap(err)
}
