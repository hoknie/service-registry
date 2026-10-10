package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/postgres"
)

type Secrets struct{ db *postgres.DB }

func NewSecrets(db *postgres.DB) *Secrets { return &Secrets{db: db} }

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

func (r secretRow) secret() catalog.Secret {
	s := catalog.Secret{ID: r.ID, NodeID: r.NodeID, Name: r.Name, Description: r.Description, Fingerprint: r.Fingerprint,
		Ref: r.ValueRef, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Storage: catalog.SecretStored}
	if r.ValueRef != nil {
		s.Storage, s.Fingerprint = catalog.SecretReference, nil
	}
	if r.NodeID != nil && r.FromName != nil {
		s.From = &catalog.NodeRef{ID: *r.NodeID, Name: *r.FromName}
	}
	return s
}

func collectSecrets(rows pgx.Rows, err error) ([]catalog.Secret, error) {
	if err != nil {
		return nil, dbErr(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[secretRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]catalog.Secret, 0, len(found))
	for _, r := range found {
		out = append(out, r.secret())
	}
	return out, nil
}

func (s *Secrets) Insert(ctx context.Context, n catalog.NewSecret) (catalog.Secret, error) {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO secrets (id, node_id, name, description, value_enc, value_ref, fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.NodeID, n.Name, n.Description, n.Stored.Enc, n.Stored.Ref, n.Stored.Fingerprint)
	if err != nil {
		return catalog.Secret{}, dbErr(err)
	}
	return s.must(ctx, n.ID)
}

func (s *Secrets) must(ctx context.Context, id uuid.UUID) (catalog.Secret, error) {
	found, err := s.Get(ctx, id)
	if err != nil {
		return catalog.Secret{}, err
	}
	if found == nil {
		return catalog.Secret{}, catalog.ErrNotFound
	}
	return *found, nil
}

func (s *Secrets) Get(ctx context.Context, id uuid.UUID) (*catalog.Secret, error) {
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

func (s *Secrets) Update(ctx context.Context, id uuid.UUID, scope *uuid.UUID, c catalog.SecretChanges) (catalog.Secret, error) {
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
		return catalog.Secret{}, dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return catalog.Secret{}, catalog.ErrNotFound
	}
	return s.must(ctx, id)
}

func (s *Secrets) Delete(ctx context.Context, id uuid.UUID, scope *uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM secrets
		WHERE id = $1
			AND node_id IS NOT DISTINCT FROM $2`, id, scope)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return nil
}

func (s *Secrets) Global(ctx context.Context) ([]catalog.Secret, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT s.id, s.node_id, s.name, s.description, s.value_ref, s.fingerprint, NULL::text AS from_name,
			rfc3339(s.created_at) AS created_at, rfc3339(s.updated_at) AS updated_at
		FROM secrets s
		WHERE s.node_id IS NULL
		ORDER BY lower(s.name), s.id`)
	return collectSecrets(rows, err)
}

func (s *Secrets) Available(ctx context.Context, nodeID uuid.UUID) ([]catalog.Secret, error) {
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

func (s *Secrets) Stored(ctx context.Context, id uuid.UUID) (*catalog.StoredSecret, error) {
	var out catalog.StoredSecret
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT value_enc, value_ref, fingerprint
		FROM secrets
		WHERE id = $1`, id).Scan(&out.Enc, &out.Ref, &out.Fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &out, nil
}

func (s *Secrets) Refs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalog.SecretRef, error) {
	out := map[uuid.UUID]catalog.SecretRef{}
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
		out[f.ID] = catalog.SecretRef{ID: f.ID, Name: f.Name, From: f.From}
	}
	return out, nil
}

func (s *Secrets) Sealed(ctx context.Context) ([]catalog.StoredSecretRow, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT id, value_enc
		FROM secrets
		WHERE value_enc IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalog.StoredSecretRow, error) {
		var row catalog.StoredSecretRow
		return row, r.Scan(&row.ID, &row.Enc)
	})
	return out, dbErr(err)
}

func (s *Secrets) ReplaceValue(ctx context.Context, id uuid.UUID, enc string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE secrets
		SET value_enc = $2
		WHERE id = $1`, id, enc)
	return dbErr(err)
}
