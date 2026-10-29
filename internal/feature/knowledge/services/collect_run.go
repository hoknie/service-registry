package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/feature/knowledge/usecases"
	"svc-registry/internal/platform/apperr"
)

type collectRun struct {
	s           *CollectService
	id          uuid.UUID
	project     knowledgetypes.Project
	scan        *knowledgetypes.Scan
	next, retry uint32
	timeout     time.Duration
	defBranch   string
	failed      bool
}

func (r *collectRun) release(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	event := r.project.Settings.Refresh == knowledgetypes.RefreshEvent
	return apperr.Wrap(r.s.collectionRepository.Release(context.WithoutCancel(ctx), r.id, knowledgetypes.Release{
		AfterSecs: r.next, OnEvent: event && !r.failed, Retry: event && r.failed, Seen: r.project.TriggeredAt}))
}

func (r *collectRun) delay(reset *time.Time) {
	r.failed = true
	if reset != nil {
		if secs := time.Until(*reset).Seconds(); secs > 0 {
			r.next = uint32(secs) + 1
		}
		return
	}
	r.next = max(r.next, r.retry)
}

func (r *collectRun) fail(ctx context.Context, branches []string, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	code, reset := failureOf(err)
	r.scan.Fail(code, err)
	r.delay(reset)
	for _, b := range branches {
		r.scan.Branches = append(r.scan.Branches, knowledgetypes.ScanBranch{Name: b, Kind: knowledgetypes.RefBranch, Result: knowledgetypes.BranchFailed, Error: code})
		if err := r.s.recordFailure(ctx, r.id, knowledgetypes.RefBranch, b, code); err != nil {
			return err
		}
	}
	return r.release(ctx)
}

func (r *collectRun) sourceHeads(ctx context.Context, src *knowledgetypes.StoredSource) (knowledgeports.ReaderPort, []head, error) {
	s, p, id := r.s, r.project, r.id
	r.scan.Source = string(src.Kind)
	def := p.DefaultBranch
	known := knownBranches(src, def)
	token, err := s.sourceToken(ctx, id, src)
	if err != nil {
		return nil, nil, r.fail(ctx, known, err)
	}
	source := src.Source
	source.Timeout = r.timeout
	reader, err := s.readers.Open(ctx, source, p.Settings, token)
	if err != nil {
		return nil, nil, r.fail(ctx, known, err)
	}
	shown, srcDef, err := reader.Heads(ctx)
	if err != nil {
		return nil, nil, r.fail(ctx, known, err)
	}
	for name := range shown {
		if !catalogtypes.BranchFits(name) {
			delete(shown, name)
		}
	}
	if !catalogtypes.BranchFits(srcDef) {
		srcDef = ""
	}
	if src.Kind == knowledgetypes.SourceLocalDir {
		def = knowledgetypes.LocalBranch
	} else if def == "" {
		def = srcDef
	}
	r.defBranch = def
	var heads []head
	kept := map[string]string{}
	for name, sha := range shown {
		if p.Settings.CollectsBranch(name, def) {
			kept[name] = sha
			heads = append(heads, head{knowledgetypes.RefBranch, name, sha})
		}
	}
	var branches map[string]string
	if src.Kind != knowledgetypes.SourceLocalDir {
		branches = make(map[string]string, len(shown))
		for name, sha := range shown {
			branches[name] = knowledgetypes.CommitOf(sha)
		}
	}
	if err := s.setHeads(ctx, id, kept, srcDef, branches); err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	if src.Kind != knowledgetypes.SourceLocalDir {
		if err := s.fillDefaultBranch(ctx, id, srcDef); err != nil {
			return nil, nil, apperr.Wrap(err)
		}
	}
	if err := s.recordRepository(ctx, id, reader); err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	if len(heads) == 0 {
		r.scan.Warn(knowledgetypes.WarnNoBranches)
	}
	return reader, heads, nil
}

func (r *collectRun) repoSyncHeads(ctx context.Context) (knowledgeports.ReaderPort, []head, error) {
	s, p, id := r.s, r.project, r.id
	r.scan.Source = "repo_sync"
	repo, client, code := s.repoSync.Projects.ProjectClient(ctx, id, r.timeout)
	branches, err := s.snapshotRepository.Branches(ctx, id)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	var heads []head
	for _, b := range branches {
		if !b.Gone && b.HeadSHA != "" && p.Settings.CollectsBranch(b.Name, p.DefaultBranch) {
			heads = append(heads, head{knowledgetypes.RefBranch, b.Name, b.HeadSHA})
		}
	}
	if code != "" {
		names := make([]string, 0, len(heads))
		for _, h := range heads {
			names = append(names, h.name)
		}
		return nil, nil, r.fail(ctx, names, knowledgetypes.Failure(code))
	}
	if len(heads) == 0 {
		r.scan.Warn(knowledgetypes.WarnNoBranches)
	}
	r.defBranch = p.DefaultBranch
	if r.defBranch == "" {
		r.defBranch = repo.DefaultBranch
	}
	return &syncedReader{client: client, repo: repo, settings: p.Settings, maxTags: int(s.cfg.Branches.SyncMaxPerRepo)}, heads, nil
}

func (r *collectRun) collect(ctx context.Context, reader knowledgeports.ReaderPort, heads []head) error {
	s, p, id := r.s, r.project, r.id
	for _, h := range heads {
		if ctx.Err() != nil {
			return nil
		}
		if !p.Force {
			last, err := s.snapshotRepository.LastAttempt(ctx, id, h.kind, h.name)
			if err != nil {
				return apperr.Wrap(err)
			}
			if last != nil && last.Commit == h.sha && last.Status != knowledgetypes.StatusFailed {
				r.scan.Branches = append(r.scan.Branches, knowledgetypes.ScanBranch{Name: h.name, Kind: h.kind, Commit: h.sha, Result: knowledgetypes.BranchUnchanged})
				continue
			}
		}
		if err := s.collectBranch(ctx, p, reader, h, r.scan, h.kind == knowledgetypes.RefBranch && h.name == r.defBranch); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			code, reset := failureOf(err)
			r.scan.Fail(code, err)
			r.scan.Branches = append(r.scan.Branches, knowledgetypes.ScanBranch{Name: h.name, Kind: h.kind, Commit: h.sha, Result: knowledgetypes.BranchFailed, Error: code})
			slog.Info("documentation collection failed", "project", id, "branch", h.name, "kind", h.kind, "code", code)
			if err := s.recordFailure(ctx, id, h.kind, h.name, code, h.sha); err != nil {
				return err
			}
			r.delay(reset)
		}
	}
	return r.release(ctx)
}

func (r *collectRun) tagHeads(ctx context.Context, reader knowledgeports.ReaderPort) ([]head, error) {
	s, p := r.s, r.project
	limit := int(s.cfg.Knowledge.MaxTags)
	if !p.Settings.WantsTags() || limit == 0 {
		return nil, apperr.Wrap(s.syncTags.Run(ctx, usecases.SyncTagsInput{Project: r.id}))
	}
	found, err := reader.Tags(ctx)
	if err != nil {
		code, reset := failureOf(err)
		r.scan.Fail(code, err)
		r.delay(reset)
		slog.Info("documentation tags not listed", "project", r.id, "code", code)
		return nil, nil
	}
	for name := range found {
		if !catalogtypes.BranchFits(name) {
			delete(found, name)
		}
	}
	known, err := s.tagRepository.Known(ctx, r.id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	picked, release := knowledgetypes.PickTags(found, known, p.Settings, limit, time.Now())
	if err := s.syncTags.Run(ctx, usecases.SyncTagsInput{Project: r.id, Picked: picked, Release: release}); err != nil {
		return nil, apperr.Wrap(err)
	}
	heads := make([]head, len(picked))
	for i, t := range picked {
		heads[i] = head{knowledgetypes.RefTag, t.Name, t.Commit}
	}
	return heads, nil
}
