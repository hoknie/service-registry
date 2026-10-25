package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/postgres"
)

func (s *OwnerRepository) Badges(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]ownershiptypes.Badge, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, scanBadge, `
		WITH RECURSIVE up AS (
			SELECT n.id AS start, n.id, n.parent_id, 0 AS lvl
			FROM nodes n
			WHERE n.id = ANY($1)
			UNION ALL
			SELECT up.start, p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		),
		chain AS (
			SELECT DISTINCT ON (up.start, COALESCE(o.group_id, o.user_id)) up.start, up.lvl, o.position, o.group_id, o.user_id
			FROM up
			JOIN node_owners o ON o.node_id = up.id
			ORDER BY up.start, COALESCE(o.group_id, o.user_id), up.lvl DESC
		)
		SELECT c.start, c.group_id IS NOT NULL, COALESCE(c.group_id, c.user_id), COALESCE(g.name, u.display_name)
		FROM chain c
		LEFT JOIN groups g ON g.id = c.group_id
		LEFT JOIN users u ON u.id = c.user_id
		ORDER BY c.start, c.lvl DESC, c.position`, ids)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]ownershiptypes.Badge{}
	for _, r := range rows {
		out[r.nodeID] = append(out[r.nodeID], r.badge)
	}
	return out, nil
}

func (s *OwnerRepository) OwnedProjects(ctx context.Context, caller uuid.UUID, f catalogtypes.OwnerFilter) ([]uuid.UUID, error) {
	var group, user, me *uuid.UUID
	switch {
	case f.Me:
		me = &caller
	case f.Kind == catalogtypes.SubjectGroup:
		group = &f.ID
	default:
		user = &f.ID
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		WITH RECURSIVE owned AS (
			SELECT o.node_id AS id
			FROM node_owners o
			WHERE o.group_id = $1
				OR o.user_id = $2
				OR o.user_id = $3
				OR o.group_id IN (SELECT m.group_id FROM group_members m WHERE m.user_id = $3)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN owned w ON c.parent_id = w.id
		)
		SELECT n.id
		FROM owned w
		JOIN nodes n ON n.id = w.id
		WHERE n.kind = 'project'
		ORDER BY n.id`, group, user, me)
}

func (s *OwnerRepository) Suggest(ctx context.Context, v catalogtypes.Viewer, all bool, prefix string) ([]ownershiptypes.Suggestion, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanSuggestion, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $4::uuid[], $5::boolean)
		),
		readable AS (
			SELECT n.id
			FROM nodes n
			WHERE $2
				OR n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN readable r ON c.parent_id = r.id
		),
		up AS (
			SELECT n.id AS project, n.id, n.parent_id
			FROM nodes n
			JOIN readable r ON r.id = n.id
			WHERE n.kind = 'project'
			UNION ALL
			SELECT up.project, p.id, p.parent_id
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT o.group_id IS NOT NULL, COALESCE(o.group_id, o.user_id), COALESCE(g.name, u.display_name) AS name, count(DISTINCT up.project)
		FROM up
		JOIN node_owners o ON o.node_id = up.id
		LEFT JOIN groups g ON g.id = o.group_id
		LEFT JOIN users u ON u.id = o.user_id
		WHERE COALESCE(g.name, u.display_name) ILIKE $3 ESCAPE '\' OR u.email ILIKE $3 ESCAPE '\'
		GROUP BY 1, 2, 3
		ORDER BY count(DISTINCT up.project) DESC, lower(COALESCE(g.name, u.display_name)), 2
		LIMIT 20`, v.UserID, all, prefix, v.Nodes, v.Admin)
}

func (s *OwnerRepository) Candidates(ctx context.Context, nodeID uuid.UUID, prefix string) ([]ownershiptypes.Candidate, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanCandidate, `
		SELECT c.is_group, c.id, c.name, c.email, c.members, c.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = c.organization_id) AS organization_name
		FROM (
			SELECT true AS is_group, g.id, g.name, NULL::text AS email,
				(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS members, g.organization_id
			FROM groups g
			WHERE g.name ILIKE $1 ESCAPE '\'
				AND (g.organization_id IS NULL OR g.organization_id = node_root($2))
			UNION ALL
			SELECT false, u.id, u.display_name, u.email, NULL::bigint, NULL::uuid
			FROM users u
			WHERE u.status = 'active'
				AND NOT u.is_service
				AND (u.email ILIKE $1 ESCAPE '\' OR u.display_name ILIKE $1 ESCAPE '\')
		) c
		ORDER BY c.is_group DESC, c.organization_id IS NULL, lower(c.name), c.id
		LIMIT 20`, prefix, nodeID)
}

func (s *OwnerRepository) GroupOwned(ctx context.Context, groupIDs []uuid.UUID) ([][2]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) ([2]uuid.UUID, error) {
		var pair [2]uuid.UUID
		err := r.Scan(&pair[0], &pair[1])
		return pair, err
	}, `
		SELECT group_id, node_id
		FROM node_owners
		WHERE group_id IS NOT NULL
			AND ($1::uuid[] IS NULL OR group_id = ANY($1))
		ORDER BY group_id, node_id`, groupIDs)
}

func (s *OwnerRepository) OwnedAmong(ctx context.Context, userID uuid.UUID, nodeIDs []uuid.UUID) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		WITH RECURSIVE up AS (
			SELECT n.id AS target, n.id, n.parent_id
			FROM nodes n
			WHERE n.id = ANY($2)
			UNION ALL
			SELECT up.target, p.id, p.parent_id
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT DISTINCT up.target
		FROM up
		WHERE EXISTS (
			SELECT 1
			FROM node_owners o
			WHERE o.node_id = up.id
				AND (o.user_id = $1
					OR o.group_id IN (SELECT m.group_id FROM group_members m WHERE m.user_id = $1))
		)
		ORDER BY up.target`, userID, nodeIDs)
}

func (s *OwnerRepository) MetricCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE reach AS (
			SELECT DISTINCT node_id AS id
			FROM node_owners
			UNION
			SELECT n.id
			FROM nodes n
			JOIN reach r ON n.parent_id = r.id
		)
		SELECT 'covered:owners', count(*)
		FROM nodes
		WHERE kind = 'project'
			AND id IN (SELECT id FROM reach)`)
}
