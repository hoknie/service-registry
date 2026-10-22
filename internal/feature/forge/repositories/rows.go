package repositories

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
)

func scanConnection(row pgx.Row) (forgetypes.Connection, error) {
	var c forgetypes.Connection
	var kind string
	var ref, fingerprint, webhook *string
	var secret *uuid.UUID
	var hasEnc bool
	var run nullRun
	err := row.Scan(&c.ID, &c.NodeID, &kind, &c.APIURL, &c.OwnerPath, &c.MirrorSubgroups, &c.IncludeArchived,
		&c.IncludeForks, &c.NameInclude, &c.NameExclude, &c.BranchInclude, &c.IntervalSecs, &secret, &hasEnc, &ref, &fingerprint, &webhook,
		&c.NextRunAt, &c.CreatedAt, &c.UpdatedAt,
		&run.id, &run.connID, &run.trigger, &run.status, &run.started, &run.finished, &run.created, &run.updated,
		&run.orphaned, &run.skipped, &run.code, &run.message, &run.problems)
	if err != nil {
		return forgetypes.Connection{}, err
	}
	var ok bool
	if c.Kind, ok = forgetypes.ParseKind(kind); !ok {
		return forgetypes.Connection{}, apperr.UnknownValue("forge kind", kind)
	}
	c.Credentials = credentialsOf(secret, hasEnc, ref, fingerprint)
	if webhook != nil {
		m, ok := forgetypes.ParseWebhookMode(*webhook)
		if !ok {
			return forgetypes.Connection{}, apperr.UnknownValue("webhook mode", *webhook)
		}
		c.WebhookMode = &m
	}
	if run.id != nil {
		r, err := run.run()
		if err != nil {
			return forgetypes.Connection{}, err
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

func (n nullRun) run() (forgetypes.Run, error) {
	r := forgetypes.Run{ID: *n.id, ConnectionID: *n.connID, StartedAt: *n.started, FinishedAt: n.finished,
		ErrorCode: n.code, ErrorMessage: n.message,
		Counts: forgetypes.Counts{Created: *n.created, Updated: *n.updated, Orphaned: *n.orphaned, Skipped: *n.skipped}}
	return finishRun(r, *n.trigger, *n.status, n.problems)
}

func finishRun(r forgetypes.Run, trigger, status string, problems []byte) (forgetypes.Run, error) {
	r.Trigger = forgetypes.Trigger(trigger)
	r.Status = forgetypes.RunStatus(status)
	r.Problems = []forgetypes.Problem{}
	if len(problems) > 0 {
		if err := json.Unmarshal(problems, &r.Problems); err != nil {
			return forgetypes.Run{}, apperr.InternalDetail("run problems: " + err.Error())
		}
	}
	return r, nil
}

func scanRun(row pgx.Row) (forgetypes.Run, error) {
	var r forgetypes.Run
	var trigger, status string
	var problems []byte
	err := row.Scan(&r.ID, &r.ConnectionID, &trigger, &status, &r.StartedAt, &r.FinishedAt,
		&r.Counts.Created, &r.Counts.Updated, &r.Counts.Orphaned, &r.Counts.Skipped, &r.ErrorCode, &r.ErrorMessage, &problems)
	if err != nil {
		return forgetypes.Run{}, err
	}
	return finishRun(r, trigger, status, problems)
}

type signalRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	Kind      string    `db:"kind"`
	Running   bool      `db:"running"`
	Queued    bool      `db:"queued"`
	Failed    bool      `db:"failed"`
	Code      *string   `db:"code"`
	LastAt    *string   `db:"last_at"`
	Pending   *int64    `db:"pending"`
}

func credentialsOf(secret *uuid.UUID, hasEnc bool, ref, fingerprint *string) catalogtypes.Credentials {
	if secret != nil {
		return catalogtypes.Credentials{Kind: catalogtypes.CredentialsSecret, SecretID: secret}
	}
	var enc *string
	if hasEnc {
		enc = new(string)
	}
	return catalogtypes.LegacyCredentials(enc, ref, fingerprint)
}
