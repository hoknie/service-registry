package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type SecretRepository struct{ db *postgres.DB }

func NewSecretRepository(db *postgres.DB) *SecretRepository { return &SecretRepository{db: db} }

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

func collectSecrets(rows pgx.Rows, err error) ([]catalogtypes.Secret, error) {
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[secretRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := make([]catalogtypes.Secret, 0, len(found))
	for _, r := range found {
		out = append(out, r.secret())
	}
	return out, nil
}

func (s *SecretRepository) Insert(ctx context.Context, n catalogtypes.NewSecret) (catalogtypes.Secret, error) {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO secrets (id, node_id, name, description, value_enc, value_ref, fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.NodeID, n.Name, n.Description, n.Stored.Enc, n.Stored.Ref, n.Stored.Fingerprint)
	if err != nil {
		return catalogtypes.Secret{}, postgres.Translate(err, constraints)
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
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, n.name AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		LEFT JOIN nodes n ON n.id = s.node_id
		WHERE s.id = $1`, id)
	found, err := collectSecrets(rows, err)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0], nil
}

func (s *SecretRepository) Update(ctx context.Context, id uuid.UUID, scope *uuid.UUID, c catalogtypes.SecretChanges) (catalogtypes.Secret, error) {
	var enc, ref, fingerprint *string
	if c.Stored != nil {
		enc, ref, fingerprint = c.Stored.Enc, c.Stored.Ref, c.Stored.Fingerprint
	}
	tag, err := s.db.From(ctx).Exec(ctx, `
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
		return catalogtypes.Secret{}, postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return catalogtypes.Secret{}, apperr.ErrNotFound
	}
	return s.must(ctx, id)
}

func (s *SecretRepository) Delete(ctx context.Context, id uuid.UUID, scope *uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM secrets
		WHERE id = $1
			AND node_id IS NOT DISTINCT FROM $2`, id, scope)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *SecretRepository) Global(ctx context.Context) ([]catalogtypes.Secret, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, NULL::text AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		WHERE s.node_id IS NULL
		ORDER BY lower(s.name), s.id`)
	return collectSecrets(rows, err)
}

func (s *SecretRepository) Available(ctx context.Context, nodeID uuid.UUID) ([]catalogtypes.Secret, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
	return collectSecrets(rows, err)
}

func (s *SecretRepository) Stored(ctx context.Context, id uuid.UUID) (*catalogtypes.StoredSecret, error) {
	var out catalogtypes.StoredSecret
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT value_enc, value_ref, fingerprint
		FROM secrets
		WHERE id = $1`, id).Scan(&out.Enc, &out.Ref, &out.Fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	return &out, nil
}

func (s *SecretRepository) Refs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalogtypes.SecretRef, error) {
	out := map[uuid.UUID]catalogtypes.SecretRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, n.name AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		LEFT JOIN nodes n ON n.id = s.node_id
		WHERE s.id = ANY($1)`, ids)
	found, err := collectSecrets(rows, err)
	if err != nil {
		return nil, err
	}
	for _, f := range found {
		out[f.ID] = catalogtypes.SecretRef{ID: f.ID, Name: f.Name, From: f.From}
	}
	return out, nil
}

func (s *SecretRepository) Sealed(ctx context.Context) ([]catalogtypes.StoredSecretRow, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT id, value_enc
		FROM secrets
		WHERE value_enc IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalogtypes.StoredSecretRow, error) {
		var row catalogtypes.StoredSecretRow
		return row, r.Scan(&row.ID, &row.Enc)
	})
	return out, postgres.Translate(err, constraints)
}

func (s *SecretRepository) ReplaceValue(ctx context.Context, id uuid.UUID, enc string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE secrets
		SET value_enc = $2
		WHERE id = $1`, id, enc)
	return postgres.Translate(err, constraints)
}
