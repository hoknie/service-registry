package forge

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/forge"
)

func internal(what, value string) error {
	return &domain.InternalError{Detail: "unknown " + what + " " + value}
}

func scanConnection(row pgx.Row) (domain.Connection, error) {
	var c domain.Connection
	var kind string
	var ref, fingerprint, webhook *string
	var run nullRun
	err := row.Scan(&c.ID, &c.NodeID, &kind, &c.APIURL, &c.OwnerPath, &c.MirrorSubgroups, &c.IncludeArchived,
		&c.IncludeForks, &c.NameInclude, &c.NameExclude, &c.BranchInclude, &c.IntervalSecs, &ref, &fingerprint, &webhook,
		&c.NextRunAt, &c.CreatedAt, &c.UpdatedAt,
		&run.id, &run.connID, &run.trigger, &run.status, &run.started, &run.finished, &run.created, &run.updated,
		&run.orphaned, &run.skipped, &run.code, &run.message, &run.problems)
	if err != nil {
		return domain.Connection{}, err
	}
	var ok bool
	if c.Kind, ok = domain.ParseKind(kind); !ok {
		return domain.Connection{}, internal("forge kind", kind)
	}
	if ref != nil {
		c.Credentials = domain.Credentials{Kind: domain.CredentialsRef, Ref: *ref}
	} else {
		c.Credentials = domain.Credentials{Kind: domain.CredentialsToken}
		if fingerprint != nil {
			c.Credentials.Fingerprint = *fingerprint
		}
	}
	if webhook != nil {
		m, ok := domain.ParseWebhookMode(*webhook)
		if !ok {
			return domain.Connection{}, internal("webhook mode", *webhook)
		}
		c.WebhookMode = &m
	}
	if run.id != nil {
		r, err := run.run()
		if err != nil {
			return domain.Connection{}, err
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

func (n nullRun) run() (domain.Run, error) {
	r := domain.Run{ID: *n.id, ConnectionID: *n.connID, StartedAt: *n.started, FinishedAt: n.finished,
		ErrorCode: n.code, ErrorMessage: n.message,
		Counts: domain.Counts{Created: *n.created, Updated: *n.updated, Orphaned: *n.orphaned, Skipped: *n.skipped}}
	return finishRun(r, *n.trigger, *n.status, n.problems)
}

func finishRun(r domain.Run, trigger, status string, problems []byte) (domain.Run, error) {
	r.Trigger = domain.Trigger(trigger)
	r.Status = domain.RunStatus(status)
	r.Problems = []domain.Problem{}
	if len(problems) > 0 {
		if err := json.Unmarshal(problems, &r.Problems); err != nil {
			return domain.Run{}, &domain.InternalError{Detail: "run problems: " + err.Error()}
		}
	}
	return r, nil
}

func scanRun(row pgx.Row) (domain.Run, error) {
	var r domain.Run
	var trigger, status string
	var problems []byte
	err := row.Scan(&r.ID, &r.ConnectionID, &trigger, &status, &r.StartedAt, &r.FinishedAt,
		&r.Counts.Created, &r.Counts.Updated, &r.Counts.Orphaned, &r.Counts.Skipped, &r.ErrorCode, &r.ErrorMessage, &problems)
	if err != nil {
		return domain.Run{}, err
	}
	return finishRun(r, trigger, status, problems)
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &conflict) || errors.Is(err, domain.ErrNotFound) || errors.As(err, &internalErr) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.ConstraintName {
		case "forge_connections_owner_key":
			return domain.ConflictOwnerTaken
		case "forge_connections_node_id_fkey", "forge_repositories_project_id_fkey", "forge_groups_node_id_fkey":
			return domain.ErrNotFound
		}
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}
