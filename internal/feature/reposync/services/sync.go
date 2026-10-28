package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	reposyncports "svc-registry/internal/feature/reposync/ports"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

const syncLeaseSecs = 120

func (s *SyncService) ClaimRepoSyncRuns(ctx context.Context, limit, leaseSecs int) ([]reposynctypes.Claimed, error) {
	return s.connectionRepository.Claim(ctx, limit, leaseSecs)
}

func (s *SyncService) ExtendRepoSyncRun(ctx context.Context, c reposynctypes.Claimed, leaseSecs int) error {
	return s.connectionRepository.Extend(ctx, c.ID, leaseSecs)
}

func (s *SyncService) RunClaimedSync(ctx context.Context, c reposynctypes.Claimed) error {
	run, err := s.runSync(ctx, c.ID, c.Trigger)
	if err != nil {
		slog.Error("repository sync failed", "connection", c.ID, "error", err)
		return err
	}
	if run.Status == reposynctypes.RunFailed {
		return reposynctypes.ErrRunFailed
	}
	return nil
}

func (s *SyncService) RunSyncNow(ctx context.Context, id uuid.UUID) (reposynctypes.Run, error) {
	conn, err := s.connectionRepository.Find(ctx, id)
	if err != nil {
		return reposynctypes.Run{}, apperr.Wrap(err)
	}
	if conn == nil {
		return reposynctypes.Run{}, apperr.New(apperr.NotFound)
	}
	ok, err := s.connectionRepository.ClaimOne(ctx, id, syncLeaseSecs)
	if err != nil {
		return reposynctypes.Run{}, apperr.Wrap(err)
	}
	if !ok {
		return reposynctypes.Run{}, ErrSyncBusy
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(syncLeaseSecs / 3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = s.connectionRepository.Extend(context.WithoutCancel(ctx), id, syncLeaseSecs)
			}
		}
	}()
	return s.runSync(ctx, id, reposynctypes.TriggerCLI)
}

var ErrSyncBusy = errors.New("sync already running")

func (s *SyncService) runSync(ctx context.Context, id uuid.UUID, trigger reposynctypes.Trigger) (reposynctypes.Run, error) {
	keep := context.WithoutCancel(ctx)
	run, err := s.runRepository.Start(keep, reposynctypes.NewRun{ID: uuid.Must(uuid.NewV7()), ConnectionID: id, Trigger: trigger})
	if err != nil {
		_ = s.connectionRepository.Release(keep, id, nil)
		return reposynctypes.Run{}, err
	}
	conn, err := s.connectionRepository.Find(keep, id)
	if err != nil || conn == nil {
		return run, err
	}
	res, next := s.synchronize(ctx, *conn, trigger)
	if err := s.finishRun(keep, *conn, run, res); err != nil {
		return run, err
	}
	if err := s.connectionRepository.Release(keep, id, next); err != nil {
		return run, err
	}
	run.Status, run.Counts, run.ErrorCode, run.ErrorMessage, run.Problems = res.Status, res.Counts, res.ErrorCode, res.ErrorMessage, res.Problems
	return run, nil
}

func failedWith(res reposynctypes.RunResult, err error) (reposynctypes.RunResult, *time.Time) {
	code := reposynctypes.FailureCode(err)
	msg := err.Error()
	if code == "" {
		code = "internal"
		msg = "internal error"
		slog.Error("repository sync internal error", "error", err)
	}
	res.ErrorCode, res.ErrorMessage = &code, &msg
	if errors.Is(err, reposynctypes.ErrInterrupted) {
		res.Status = reposynctypes.RunFailed
		return res, new(time.Now())
	}
	if limited, ok := errors.AsType[*reposynctypes.RateLimited](err); ok {
		res.Status = reposynctypes.RunRateLimited
		if !limited.Reset.IsZero() {
			return res, new(limited.Reset)
		}
		return res, nil
	}
	res.Status = reposynctypes.RunFailed
	return res, nil
}

func (s *SyncService) clientFor(ctx context.Context, conn reposynctypes.Connection) (reposyncports.ClientPort, error) {
	sec, err := s.connectionRepository.Secrets(ctx, conn.ID)
	if err != nil {
		return nil, err
	}
	if sec == nil {
		return nil, apperr.ErrNotFound
	}
	var token string
	switch {
	case sec.CredentialsSecretID != nil:
		token, err = s.catalog.SecretResolver.ResolveSecret(ctx, *sec.CredentialsSecretID)
	default:
		err = errors.New("no credentials")
	}
	if err != nil {
		slog.Warn("hosting credentials unavailable", "connection", conn.ID, "error", err)
		return nil, reposynctypes.ErrCredentialsUnavailable
	}
	return s.clients.New(reposynctypes.Endpoint{Kind: conn.Kind, APIURL: conn.APIURL, Owner: conn.OwnerPath, Token: token})
}

func settingsOf(c reposynctypes.Connection) reposynctypes.Settings {
	return reposynctypes.Settings{Kind: c.Kind, APIURL: c.APIURL, OwnerPath: c.OwnerPath, MirrorSubgroups: c.MirrorSubgroups,
		IncludeArchived: c.IncludeArchived, IncludeForks: c.IncludeForks, NameInclude: c.NameInclude,
		NameExclude: c.NameExclude, BranchInclude: c.BranchInclude, IntervalSecs: c.IntervalSecs}
}

func (s *SyncService) remoteState(ctx context.Context, conn reposynctypes.Connection, client reposyncports.ClientPort) ([]reposynctypes.Target, []reposynctypes.Link, error) {
	all, err := client.ListRepos(ctx)
	if err != nil {
		return nil, nil, err
	}
	settings := settingsOf(conn)
	var kept []reposynctypes.RemoteRepo
	for _, r := range all {
		if reposynctypes.Keep(settings, r) {
			kept = append(kept, r)
		}
	}
	excluded, err := s.exclusionRepository.IDs(ctx, conn.ID)
	if err != nil {
		return nil, nil, err
	}
	links, err := s.repoRepository.Links(ctx, conn.ID)
	if err != nil {
		return nil, nil, err
	}
	adoptions, err := s.adoptionRepository.Eligible(ctx, conn.ID)
	if err != nil {
		return nil, nil, err
	}
	targets, orphans := reposynctypes.Plan(reposynctypes.WithoutExcluded(kept, excluded), links)
	reposynctypes.Adopt(targets, adoptions)
	return targets, orphans, nil
}

func (s *SyncService) synchronize(ctx context.Context, conn reposynctypes.Connection, trigger reposynctypes.Trigger) (reposynctypes.RunResult, *time.Time) {
	res := reposynctypes.RunResult{Status: reposynctypes.RunSucceeded}
	failed := func(res reposynctypes.RunResult, err error) (reposynctypes.RunResult, *time.Time) {
		if ctx.Err() != nil {
			err = reposynctypes.ErrInterrupted
		}
		return failedWith(res, err)
	}
	client, err := s.clientFor(ctx, conn)
	if err != nil {
		return failed(res, err)
	}
	targets, orphans, err := s.remoteState(ctx, conn, client)
	if err != nil {
		return failed(res, err)
	}
	groups, err := s.repoRepository.Groups(ctx, conn.ID)
	if err != nil {
		return failed(res, err)
	}
	sc := &syncer{svc: s, conn: conn, trigger: trigger, client: client, groups: groups}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return failed(sc.res(res), reposynctypes.ErrInterrupted)
		}
		if err := sc.apply(ctx, t); err != nil {
			return failed(sc.res(res), err)
		}
	}
	ids := make([]uuid.UUID, 0, len(orphans))
	for _, o := range orphans {
		ids = append(ids, o.ProjectID)
	}
	if err := s.repoRepository.Orphan(context.WithoutCancel(ctx), ids); err != nil {
		return failed(sc.res(res), err)
	}
	sc.counts.Orphaned = len(ids)
	return sc.res(res), nil
}

func conflictCode(err error) string {
	if e := apperr.From(err); e != nil && e.Kind == apperr.Conflict {
		return e.Code
	}
	return ""
}

func description(d string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, d)
	if utf8.RuneCountInString(clean) > 2000 {
		clean = string([]rune(clean)[:2000])
	}
	return clean
}

func nodeName(name, slug string) string {
	clean := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if utf8.RuneCountInString(clean) > 100 {
		clean = string([]rune(clean)[:100])
	}
	if clean == "" {
		return slug
	}
	return clean
}
