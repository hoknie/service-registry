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
	out := map[uuid.UUID][]ownershiptypes.Badge{}
	if len(ids) == 0 {
		return out, nil
	}
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
		src AS (
			SELECT DISTINCT ON (up.start) up.start, up.id
			FROM up
			WHERE (EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = up.id) OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = up.id))
			ORDER BY up.start, up.lvl
		)
		SELECT src.start, o.group_id IS NOT NULL, COALESCE(o.group_id, o.user_id), COALESCE(g.name, u.display_name)
		FROM src
		JOIN node_owners o ON o.node_id = src.id
		LEFT JOIN groups g ON g.id = o.group_id
		LEFT JOIN users u ON u.id = o.user_id
		ORDER BY src.start, o.position`, ids)
	if err != nil {
		return nil, err
	}
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
		WITH RECURSIVE down AS (
			SELECT n.id, n.kind,
				CASE WHEN (EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = n.id) OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = n.id)) THEN n.id END AS src
			FROM nodes n
			WHERE n.parent_id IS NULL
			UNION ALL
			SELECT c.id, c.kind,
				CASE WHEN (EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = c.id) OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = c.id)) THEN c.id ELSE d.src END
			FROM nodes c
			JOIN down d ON c.parent_id = d.id
		)
		SELECT d.id
		FROM down d
		WHERE d.kind = 'project'
			AND EXISTS (
				SELECT 1
				FROM node_owners o
				WHERE o.node_id = d.src
					AND (o.group_id = $1
						OR o.user_id = $2
						OR o.user_id = $3
						OR o.group_id IN (SELECT m.group_id FROM group_members m WHERE m.user_id = $3))
			)`, group, user, me)
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
		down AS (
			SELECT n.id, n.kind,
				CASE WHEN (EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = n.id) OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = n.id)) THEN n.id END AS src
			FROM nodes n
			WHERE n.parent_id IS NULL
			UNION ALL
			SELECT c.id, c.kind,
				CASE WHEN (EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = c.id) OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = c.id)) THEN c.id ELSE d.src END
			FROM nodes c
			JOIN down d ON c.parent_id = d.id
		)
		SELECT o.group_id IS NOT NULL, COALESCE(o.group_id, o.user_id), COALESCE(g.name, u.display_name) AS name, count(*)
		FROM down d
		JOIN readable r ON r.id = d.id
		JOIN node_owners o ON o.node_id = d.src
		LEFT JOIN groups g ON g.id = o.group_id
		LEFT JOIN users u ON u.id = o.user_id
		WHERE d.kind = 'project'
			AND (COALESCE(g.name, u.display_name) ILIKE $3 ESCAPE '\' OR u.email ILIKE $3 ESCAPE '\')
		GROUP BY 1, 2, 3
		ORDER BY count(*) DESC, lower(COALESCE(g.name, u.display_name)), 2
		LIMIT 20`, v.UserID, all, prefix, v.Nodes, v.Admin)
}

func (s *OwnerRepository) Candidates(ctx context.Context, prefix string) ([]ownershiptypes.Candidate, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanCandidate, `
		SELECT c.is_group, c.id, c.name, c.email, c.members
		FROM (
			SELECT true AS is_group, g.id, g.name, NULL::text AS email,
				(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS members
			FROM groups g
			WHERE g.name ILIKE $1 ESCAPE '\'
			UNION ALL
			SELECT false, u.id, u.display_name, u.email, NULL::bigint
			FROM users u
			WHERE u.status = 'active'
				AND NOT u.is_service
				AND (u.email ILIKE $1 ESCAPE '\' OR u.display_name ILIKE $1 ESCAPE '\')
		) c
		ORDER BY c.is_group DESC, lower(c.name), c.id
		LIMIT 20`, prefix)
}
