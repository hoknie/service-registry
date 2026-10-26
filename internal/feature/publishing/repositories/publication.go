package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	publishingtypes "svc-registry/internal/feature/publishing/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type PublicationRepository struct{ db *pgtx.DB }

func NewPublicationRepository(db *pgtx.DB) *PublicationRepository {
	return &PublicationRepository{db: db}
}

func (s *PublicationRepository) Chain(ctx context.Context, nodeID uuid.UUID) ([]publishingtypes.ChainNode, error) {
	chain, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(nodeRow.chain), `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id, n.parent_id, up.lvl + 1
			FROM nodes n
			JOIN up ON n.id = up.parent_id
		)
		SELECT n.id, n.parent_id, n.kind, n.slug, n.name, n.description, p.publish, p.versions, p.default_branch,
			p.current_release, p.name AS public_name, p.host
		FROM up
		JOIN nodes n ON n.id = up.id
		LEFT JOIN docs_publications p ON p.node_id = n.id
		ORDER BY up.lvl`, nodeID)
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return nil, apperr.ErrNotFound
	}
	return chain, nil
}

func (s *PublicationRepository) Subtree(ctx context.Context, rootID uuid.UUID) ([]publishingtypes.Node, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(nodeRow.node), `
		WITH RECURSIVE down AS (
			SELECT id, 0 AS depth
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id, down.depth + 1
			FROM nodes n
			JOIN down ON n.parent_id = down.id
		)
		SELECT n.id, n.parent_id, n.kind, n.slug, n.name, n.description, p.publish, p.versions, p.default_branch,
			p.current_release, p.name AS public_name, p.host
		FROM down
		JOIN nodes n ON n.id = down.id
		LEFT JOIN docs_publications p ON p.node_id = n.id
		ORDER BY down.depth, CASE n.kind WHEN 'organization' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END, lower(n.name), n.id`, rootID)
}

func (s *PublicationRepository) EntryNode(ctx context.Context, segment string) (*uuid.UUID, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM (
			SELECT node_id AS id, 0 AS rank
			FROM docs_publications
			WHERE name = lower($1)
			UNION ALL
			SELECT id, 1 AS rank
			FROM nodes
			WHERE parent_id IS NULL
				AND lower(slug) = lower($1)
		) e
		ORDER BY rank
		LIMIT 1`, segment)
}

func (s *PublicationRepository) SlugTaken(ctx context.Context, name string) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT true
		FROM nodes
		WHERE parent_id IS NULL
			AND lower(slug) = lower($1)`, name)
	return found != nil, err
}

func (s *PublicationRepository) Hosts(ctx context.Context) (map[string]uuid.UUID, error) {
	return postgres.Pairs[string, uuid.UUID](ctx, s.db.From(ctx), constraints, `
		SELECT host, node_id
		FROM docs_publications
		WHERE host IS NOT NULL`)
}

func (s *PublicationRepository) Delete(ctx context.Context, nodeID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM docs_publications
		WHERE node_id = $1`, nodeID)
	return err
}

func (s *PublicationRepository) Upsert(ctx context.Context, nodeID uuid.UUID, in publishingtypes.NodeSettings) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO docs_publications (node_id, publish, versions, default_branch, current_release, name, host)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (node_id)
		DO UPDATE SET publish = $2, versions = $3, default_branch = $4, current_release = $5, name = $6, host = $7,
			updated_at = now()`,
		nodeID, in.Publish, versionRows(in.Versions), in.DefaultBranch, in.CurrentRelease, in.Name, in.Host)
	return err
}
