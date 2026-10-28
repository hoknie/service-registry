package repositories

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

func scanConnection(row pgx.CollectableRow) (reposynctypes.Connection, error) {
	var c reposynctypes.Connection
	var kind string
	var webhook *string
	var secret *uuid.UUID
	var run nullRun
	err := row.Scan(&c.ID, &c.NodeID, &kind, &c.APIURL, &c.OwnerPath, &c.MirrorSubgroups, &c.IncludeArchived,
		&c.IncludeForks, &c.NameInclude, &c.NameExclude, &c.BranchInclude, &c.IntervalSecs, &secret, &webhook,
		&c.NextRunAt, &c.CreatedAt, &c.UpdatedAt,
		&run.id, &run.connID, &run.trigger, &run.status, &run.started, &run.finished, &run.created, &run.updated,
		&run.orphaned, &run.skipped, &run.code, &run.message, &run.problems)
	if err != nil {
		return reposynctypes.Connection{}, err
	}
	var ok bool
	if c.Kind, ok = reposynctypes.ParseKind(kind); !ok {
		return reposynctypes.Connection{}, apperr.UnknownValue("hosting kind", kind)
	}
	c.Credentials = credentialsOf(secret)
	if webhook != nil {
		m, ok := reposynctypes.ParseWebhookMode(*webhook)
		if !ok {
			return reposynctypes.Connection{}, apperr.UnknownValue("webhook mode", *webhook)
		}
		c.WebhookMode = &m
	}
	if run.id != nil {
		r, err := run.run()
		if err != nil {
			return reposynctypes.Connection{}, err
		}
		c.LastRun = &r
	}
	return c, nil
}

type nullRun struct {
	id, connID                          *uuid.UUID
	trigger, status, started            *string
	finished, code, message             *string
	created, updated, orphaned, skipped *int
	problems                            []byte
}

func (n nullRun) run() (reposynctypes.Run, error) {
	r := reposynctypes.Run{ID: *n.id, ConnectionID: *n.connID, StartedAt: *n.started, FinishedAt: n.finished,
		ErrorCode: n.code, ErrorMessage: n.message,
		Counts: reposynctypes.Counts{Created: *n.created, Updated: *n.updated, Orphaned: *n.orphaned, Skipped: *n.skipped}}
	return finishRun(r, *n.trigger, *n.status, n.problems)
}

func finishRun(r reposynctypes.Run, trigger, status string, problems []byte) (reposynctypes.Run, error) {
	r.Trigger = reposynctypes.Trigger(trigger)
	r.Status = reposynctypes.RunStatus(status)
	r.Problems = []reposynctypes.Problem{}
	if len(problems) > 0 {
		if err := json.Unmarshal(problems, &r.Problems); err != nil {
			return reposynctypes.Run{}, apperr.InternalDetail("run problems: " + err.Error())
		}
	}
	return r, nil
}

func scanRun(row pgx.CollectableRow) (reposynctypes.Run, error) {
	var r reposynctypes.Run
	var trigger, status string
	var problems []byte
	err := row.Scan(&r.ID, &r.ConnectionID, &trigger, &status, &r.StartedAt, &r.FinishedAt,
		&r.Counts.Created, &r.Counts.Updated, &r.Counts.Orphaned, &r.Counts.Skipped, &r.ErrorCode, &r.ErrorMessage, &problems)
	if err != nil {
		return reposynctypes.Run{}, err
	}
	return finishRun(r, trigger, status, problems)
}

func credentialsOf(secret *uuid.UUID) catalogtypes.Credentials {
	if secret == nil {
		return catalogtypes.Credentials{Kind: catalogtypes.CredentialsNone}
	}
	return catalogtypes.Credentials{Kind: catalogtypes.CredentialsSecret, SecretID: secret}
}

func scanProjectLink(row pgx.CollectableRow) (reposynctypes.ProjectLink, error) {
	var l reposynctypes.ProjectLink
	var kind string
	var secret *uuid.UUID
	if err := row.Scan(&l.ConnectionID, &l.NodeID, &l.Owner, &kind, &l.WebURL, &l.APIURL, &secret, &l.SyncedAt); err != nil {
		return l, err
	}
	var ok bool
	if l.Kind, ok = reposynctypes.ParseKind(kind); !ok {
		return l, apperr.UnknownValue("hosting kind", kind)
	}
	l.Credentials = credentialsOf(secret)
	return l, nil
}
