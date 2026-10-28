package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ConnectionRepository struct{ db *pgtx.DB }

func NewConnectionRepository(db *pgtx.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

func (s *ConnectionRepository) Insert(ctx context.Context, n reposynctypes.NewConnection) error {
	st := n.Settings
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO repo_sync_connections (id, node_id, kind, api_url, owner_path, mirror_subgroups,
			include_archived, include_forks, name_include, name_exclude, interval_secs, credentials_secret_id,
			branch_include)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13::text[], '{}'))`,
		n.ID, n.NodeID, string(st.Kind), st.APIURL, st.OwnerPath, st.MirrorSubgroups,
		st.IncludeArchived, st.IncludeForks, st.NameInclude, st.NameExclude, st.IntervalSecs,
		n.SecretID, st.BranchInclude)
	return err
}

func (s *ConnectionRepository) List(ctx context.Context, nodeID uuid.UUID) ([]reposynctypes.Connection, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanConnection, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_secret_id, c.webhook_mode, rfc3339(c.next_run_at),
			rfc3339(c.created_at), rfc3339(c.updated_at), lr.id, lr.connection_id, lr.trigger, lr.status,
			rfc3339(lr.started_at), rfc3339(lr.finished_at), lr.created, lr.updated, lr.orphaned, lr.skipped,
			lr.error_code, lr.error_message, lr.problems
		FROM repo_sync_connections c
		LEFT JOIN LATERAL (
			SELECT *
			FROM repo_sync_runs r
			WHERE r.connection_id = c.id
			ORDER BY r.started_at DESC, r.id DESC
			LIMIT 1
		) lr ON true
		WHERE c.node_id = $1
		ORDER BY c.created_at, c.id`, nodeID)
}

func (s *ConnectionRepository) Find(ctx context.Context, id uuid.UUID) (*reposynctypes.Connection, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanConnection, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_secret_id, c.webhook_mode, rfc3339(c.next_run_at),
			rfc3339(c.created_at), rfc3339(c.updated_at), lr.id, lr.connection_id, lr.trigger, lr.status,
			rfc3339(lr.started_at), rfc3339(lr.finished_at), lr.created, lr.updated, lr.orphaned, lr.skipped,
			lr.error_code, lr.error_message, lr.problems
		FROM repo_sync_connections c
		LEFT JOIN LATERAL (
			SELECT *
			FROM repo_sync_runs r
			WHERE r.connection_id = c.id
			ORDER BY r.started_at DESC, r.id DESC
			LIMIT 1
		) lr ON true
		WHERE c.id = $1`, id)
}

func (s *ConnectionRepository) Secrets(ctx context.Context, id uuid.UUID) (*reposynctypes.Secrets, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (reposynctypes.Secrets, error) {
		var sec reposynctypes.Secrets
		err := r.Scan(&sec.CredentialsSecretID, &sec.WebhookSecretEnc, &sec.WebhookID)
		return sec, err
	}, `
		SELECT credentials_secret_id, webhook_secret_enc, webhook_id
		FROM repo_sync_connections
		WHERE id = $1`, id)
}

func (s *ConnectionRepository) Update(ctx context.Context, id uuid.UUID, st reposynctypes.Settings, secret *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE repo_sync_connections
		SET api_url = $2, owner_path = $3, mirror_subgroups = $4, include_archived = $5, include_forks = $6,
			name_include = $7, name_exclude = $8, interval_secs = $9, branch_include = COALESCE($10::text[], '{}'),
			credentials_secret_id = COALESCE($11::uuid, credentials_secret_id), updated_at = now()
		WHERE id = $1`, id, st.APIURL, st.OwnerPath, st.MirrorSubgroups, st.IncludeArchived,
		st.IncludeForks, st.NameInclude, st.NameExclude, st.IntervalSecs, st.BranchInclude, secret)
}

func (s *ConnectionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM repo_sync_connections
		WHERE id = $1`, id)
}

func (s *ConnectionRepository) SetWebhook(ctx context.Context, id uuid.UUID, h reposynctypes.Webhook) error {
	var mode *string
	if h.Mode != nil {
		mode = new(string(*h.Mode))
	}
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE repo_sync_connections
		SET webhook_mode = $2, webhook_secret_enc = $3, webhook_id = $4, updated_at = now()
		WHERE id = $1`, id, mode, h.SecretEnc, h.HookID)
}
