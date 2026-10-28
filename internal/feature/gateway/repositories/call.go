package repositories

import (
	"context"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type CallRepository struct{ db *pgtx.DB }

func NewCallRepository(db *pgtx.DB) *CallRepository { return &CallRepository{db: db} }

func (s *CallRepository) Record(ctx context.Context, rec gatewaytypes.CallRecord) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH logged AS (
			INSERT INTO mcp_gateway_calls (id, server_id, tool, user_id, token_id, grant_id, duration_ms, outcome, code, request, response,
				payload_truncated)
			SELECT $1, $2, $3, $4, t.id, g.id, $7, $8, $9, $10, $11, $12
			FROM (SELECT 1) one
			LEFT JOIN personal_access_tokens t ON t.id = $5
			LEFT JOIN oauth_grants g ON g.id = $6
			WHERE EXISTS (SELECT 1 FROM mcp_servers WHERE id = $2)
			RETURNING server_id
		)
		UPDATE mcp_servers
		SET calls = calls + 1
		WHERE id IN (SELECT server_id FROM logged)
			AND $8 <> 'failed'`, rec.ID, rec.ServerID, rec.Tool, rec.Caller.UserID, rec.Caller.TokenID, rec.Caller.GrantID,
		rec.DurationMs, rec.Outcome, rec.Code, rec.Request, rec.Response, rec.Truncated)
	return err
}

type CallFilter struct {
	Tool     *string
	Outcome  *string
	UserID   *uuid.UUID
	AfterAt  *string
	AfterID  *uuid.UUID
	Limit    int
	ServerID uuid.UUID
}

func (s *CallRepository) List(ctx context.Context, f CallFilter) ([]gatewaytypes.CallEntry, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r callRow) gatewaytypes.CallEntry { return r.entry() }), `
		SELECT c.id, rfc3339(c.created_at) AS at, c.created_at::text AS cursor_at, c.tool, c.user_id, u.email AS user_email, u.display_name AS user_name,
			CASE WHEN t.id IS NOT NULL THEN 'token' WHEN g.id IS NOT NULL THEN 'app' END AS via_kind,
			COALESCE(t.id, g.id) AS via_id, COALESCE(t.name, cl.name) AS via_name, c.duration_ms, c.outcome, c.code, c.request, c.response,
			c.payload_truncated
		FROM mcp_gateway_calls c
		LEFT JOIN users u ON u.id = c.user_id
		LEFT JOIN personal_access_tokens t ON t.id = c.token_id
		LEFT JOIN oauth_grants g ON g.id = c.grant_id
		LEFT JOIN oauth_clients cl ON cl.id = g.client_id
		WHERE c.server_id = $1
			AND ($2::text IS NULL OR c.tool = $2)
			AND ($3::text IS NULL OR c.outcome = $3)
			AND ($4::uuid IS NULL OR c.user_id = $4)
			AND ($5::text IS NULL OR (c.created_at, c.id) < ($5::timestamptz, $6::uuid))
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $7`, f.ServerID, f.Tool, f.Outcome, f.UserID, f.AfterAt, f.AfterID, f.Limit)
}

type callRow struct {
	ID         uuid.UUID  `db:"id"`
	At         string     `db:"at"`
	CursorAt   string     `db:"cursor_at"`
	Tool       string     `db:"tool"`
	UserID     *uuid.UUID `db:"user_id"`
	UserEmail  *string    `db:"user_email"`
	UserName   *string    `db:"user_name"`
	ViaKind    *string    `db:"via_kind"`
	ViaID      *uuid.UUID `db:"via_id"`
	ViaName    *string    `db:"via_name"`
	DurationMs int        `db:"duration_ms"`
	Outcome    string     `db:"outcome"`
	Code       *string    `db:"code"`
	Request    *string    `db:"request"`
	Response   *string    `db:"response"`
	Truncated  bool       `db:"payload_truncated"`
}

func (r callRow) entry() gatewaytypes.CallEntry {
	return gatewaytypes.CallEntry{ID: r.ID, At: r.At, Cursor: r.CursorAt, Tool: r.Tool, UserID: r.UserID, UserEmail: r.UserEmail, UserName: r.UserName,
		ViaKind: r.ViaKind, ViaID: r.ViaID, ViaName: r.ViaName, DurationMs: r.DurationMs, Outcome: r.Outcome, Code: r.Code, Request: r.Request,
		Response: r.Response, Truncated: r.Truncated}
}

func (s *CallRepository) Prune(ctx context.Context, olderThanSecs int) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM mcp_gateway_calls
		WHERE created_at < now() - make_interval(secs => $1)`, float64(olderThanSecs))
}
