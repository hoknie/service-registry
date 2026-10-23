package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SecretRepository struct{ db *pgtx.DB }

func NewSecretRepository(db *pgtx.DB) *SecretRepository { return &SecretRepository{db: db} }

type secretRow struct {
	ID          uuid.UUID  `db:"id"`
	NodeID      *uuid.UUID `db:"node_id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	ValueRef    *string    `db:"value_ref"`
	Fingerprint *string    `db:"fingerprint"`
	FromName    *string    `db:"from_name"`
	CreatedAt   string     `db:"created_at"`
	UpdatedAt   string     `db:"updated_at"`
}

func (r secretRow) secret() catalogtypes.Secret {
	s := catalogtypes.Secret{ID: r.ID, NodeID: r.NodeID, Name: r.Name, Description: r.Description, Fingerprint: r.Fingerprint,
		Ref: r.ValueRef, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Storage: catalogtypes.SecretStored}
	if r.ValueRef != nil {
		s.Storage, s.Fingerprint = catalogtypes.SecretReference, nil
	}
	if r.NodeID != nil && r.FromName != nil {
		s.From = &catalogtypes.NodeRef{ID: *r.NodeID, Name: *r.FromName}
	}
	return s
}

func (s *SecretRepository) Insert(ctx context.Context, n catalogtypes.NewSecret) (catalogtypes.Secret, error) {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO secrets (id, node_id, name, description, value_enc, value_ref, fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.NodeID, n.Name, n.Description, n.Stored.Enc, n.Stored.Ref, n.Stored.Fingerprint)
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	return s.must(ctx, n.ID)
}

func (s *SecretRepository) must(ctx context.Context, id uuid.UUID) (catalogtypes.Secret, error) {
	found, err := s.Get(ctx, id)
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	if found == nil {
		return catalogtypes.Secret{}, apperr.ErrNotFound
	}
	return *found, nil
}

func (s *SecretRepository) Get(ctx context.Context, id uuid.UUID) (*catalogtypes.Secret, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(secretRow.secret), `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, n.name AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		LEFT JOIN nodes n ON n.id = s.node_id
		WHERE s.id = $1`, id)
}

func (s *SecretRepository) Update(ctx context.Context, id uuid.UUID, scope *uuid.UUID, c catalogtypes.SecretChanges) (catalogtypes.Secret, error) {
	var enc, ref, fingerprint *string
	if c.Stored != nil {
		enc, ref, fingerprint = c.Stored.Enc, c.Stored.Ref, c.Stored.Fingerprint
	}
	err := postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE secrets
		SET name = COALESCE($3, name), description = COALESCE($4, description),
			value_enc = CASE WHEN $5 THEN $6 ELSE value_enc END,
			value_ref = CASE WHEN $5 THEN $7 ELSE value_ref END,
			fingerprint = CASE WHEN $5 THEN $8 ELSE fingerprint END,
			updated_at = now()
		WHERE id = $1
			AND node_id IS NOT DISTINCT FROM $2`,
		id, scope, c.Name, c.Description, c.Stored != nil, enc, ref, fingerprint)
	if err != nil {
		return catalogtypes.Secret{}, err
	}
	return s.must(ctx, id)
}

func (s *SecretRepository) Delete(ctx context.Context, id uuid.UUID, scope *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM secrets
		WHERE id = $1
			AND node_id IS NOT DISTINCT FROM $2`, id, scope)
}

func (s *SecretRepository) Global(ctx context.Context) ([]catalogtypes.Secret, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(secretRow.secret), `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, NULL::text AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		WHERE s.node_id IS NULL
		ORDER BY lower(s.name), s.id`)
}

func (s *SecretRepository) Available(ctx context.Context, nodeID uuid.UUID) ([]catalogtypes.Secret, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(secretRow.secret), `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS depth
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.depth + 1
			FROM up
			JOIN nodes p ON p.id = up.parent_id
			WHERE up.depth < 64
		)
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, n.name AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		LEFT JOIN nodes n ON n.id = s.node_id
		WHERE s.node_id IS NULL
			OR s.node_id IN (SELECT id FROM up)
		ORDER BY lower(s.name), s.node_id NULLS LAST, s.id`, nodeID)
}

func (s *SecretRepository) Stored(ctx context.Context, id uuid.UUID) (*catalogtypes.StoredSecret, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (catalogtypes.StoredSecret, error) {
		var out catalogtypes.StoredSecret
		return out, r.Scan(&out.Enc, &out.Ref, &out.Fingerprint)
	}, `
		SELECT value_enc, value_ref, fingerprint
		FROM secrets
		WHERE id = $1`, id)
}

func (s *SecretRepository) Refs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalogtypes.SecretRef, error) {
	out := map[uuid.UUID]catalogtypes.SecretRef{}
	if len(ids) == 0 {
		return out, nil
	}
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(secretRow.secret), `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, n.name AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		LEFT JOIN nodes n ON n.id = s.node_id
		WHERE s.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	for _, f := range found {
		out[f.ID] = catalogtypes.SecretRef{ID: f.ID, Name: f.Name, From: f.From}
	}
	return out, nil
}

func (s *SecretRepository) InScope(ctx context.Context, scope *uuid.UUID) ([]catalogtypes.ScopedSecret, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (catalogtypes.ScopedSecret, error) {
		var row catalogtypes.ScopedSecret
		return row, r.Scan(&row.ID, &row.Name, &row.Stored.Enc, &row.Stored.Ref)
	}, `
		SELECT id, name, value_enc, value_ref
		FROM secrets
		WHERE node_id IS NOT DISTINCT FROM $1
		ORDER BY id`, scope)
}

func (s *SecretRepository) Sealed(ctx context.Context) ([]catalogtypes.StoredSecretRow, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (catalogtypes.StoredSecretRow, error) {
		var row catalogtypes.StoredSecretRow
		return row, r.Scan(&row.ID, &row.Enc)
	}, `
		SELECT id, value_enc
		FROM secrets
		WHERE value_enc IS NOT NULL
		ORDER BY id`)
}

func (s *SecretRepository) ReplaceValue(ctx context.Context, id uuid.UUID, enc string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE secrets
		SET value_enc = $2
		WHERE id = $1`, id, enc)
	return err
}
