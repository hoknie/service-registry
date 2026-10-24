package repositories

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OwnerRepository struct{ db *pgtx.DB }

func NewOwnerRepository(db *pgtx.DB) *OwnerRepository { return &OwnerRepository{db: db} }

func (s *OwnerRepository) Replace(ctx context.Context, nodeID uuid.UUID, owners []ownershiptypes.NewOwner) error {
	kinds := make([]string, len(owners))
	subjects := make([]string, len(owners))
	for i, o := range owners {
		kinds[i], subjects[i] = string(o.SubjectKind), o.Subject
	}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if err := postgres.ExecFound(ctx, tx, constraints, `
			SELECT 1
			FROM nodes
			WHERE id = $1 FOR NO KEY UPDATE`, nodeID); err != nil {
			return err
		}
		resolved, err := postgres.List(ctx, tx, constraints, scanResolved, `
			SELECT g.id, u.id
			FROM unnest($1::text[], $2::text[]) WITH ORDINALITY AS x(kind, subject, ord)
			LEFT JOIN LATERAL (
				SELECT g.id
				FROM groups g
				WHERE x.kind = 'group'
					AND lower(g.name) = lower(x.subject)
					AND (g.organization_id IS NULL OR g.organization_id = node_root($3))
				ORDER BY g.organization_id IS NULL
				LIMIT 1
			) g ON true
			LEFT JOIN users u ON x.kind = 'user' AND lower(u.email) = lower(x.subject)
			ORDER BY x.ord`, kinds, subjects, nodeID)
		if err != nil {
			return err
		}
		inherited, err := postgres.List(ctx, tx, constraints, pgx.RowTo[uuid.UUID], `
			WITH RECURSIVE up AS (
				SELECT n.parent_id AS id
				FROM nodes n
				WHERE n.id = $1
					AND n.parent_id IS NOT NULL
				UNION ALL
				SELECT p.parent_id
				FROM nodes p
				JOIN up ON p.id = up.id
				WHERE p.parent_id IS NOT NULL
			)
			SELECT COALESCE(o.group_id, o.user_id)
			FROM node_owners o
			WHERE o.node_id IN (SELECT id FROM up)`, nodeID)
		if err != nil {
			return err
		}
		own, err := ownSubjects(resolved, inherited)
		if err != nil {
			return err
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			DELETE FROM node_owners
			WHERE node_id = $1`, nodeID); err != nil {
			return err
		}
		for i, r := range own {
			if _, err := postgres.Exec(ctx, tx, constraints, `
				INSERT INTO node_owners (id, node_id, group_id, user_id, position)
				VALUES ($1, $2, $3, $4, $5)`, uuid.Must(uuid.NewV7()), nodeID, r.groupID, r.userID, int16(i)); err != nil {
				return err
			}
		}
		return nil
	})
	return postgres.Translate(err, constraints)
}

func ownSubjects(resolved []resolvedRow, inherited []uuid.UUID) ([]resolvedRow, error) {
	seen := map[uuid.UUID]bool{}
	own := make([]resolvedRow, 0, len(resolved))
	for _, r := range resolved {
		id := r.groupID
		if id == nil {
			id = r.userID
		}
		if id == nil {
			return nil, catalogtypes.InvalidUnknownSubject
		}
		if seen[*id] {
			return nil, ownershiptypes.InvalidOwners
		}
		seen[*id] = true
		if !slices.Contains(inherited, *id) {
			own = append(own, r)
		}
	}
	return own, nil
}

func (s *OwnerRepository) AddUser(ctx context.Context, nodeID, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO node_owners (id, node_id, user_id, position)
		VALUES ($1, $2, $3, 0)`, uuid.Must(uuid.NewV7()), nodeID, userID)
	return err
}

func (s *OwnerRepository) Effective(ctx context.Context, nodeID uuid.UUID) ([]ownershiptypes.Owner, error) {
	q := s.db.From(ctx)
	owners, err := postgres.List(ctx, q, constraints, scanOwner, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		),
		chain AS (
			SELECT DISTINCT ON (COALESCE(o.group_id, o.user_id)) o.node_id, o.group_id, o.user_id, o.position, up.lvl
			FROM up
			JOIN node_owners o ON o.node_id = up.id
			ORDER BY COALESCE(o.group_id, o.user_id), up.lvl DESC
		)
		SELECT c.node_id, c.node_id <> $1 AS inherited, c.group_id IS NOT NULL AS is_group,
			COALESCE(c.group_id, c.user_id) AS subject_id, COALESCE(g.name, u.display_name) AS name,
			u.email, u.status, g.on_call_url
		FROM chain c
		LEFT JOIN groups g ON g.id = c.group_id
		LEFT JOIN users u ON u.id = c.user_id
		ORDER BY c.lvl DESC, c.position`, nodeID)
	if err != nil {
		return nil, err
	}
	return withContacts(ctx, q, owners)
}

func withContacts(ctx context.Context, q pgtx.Querier, owners []ownershiptypes.Owner) ([]ownershiptypes.Owner, error) {
	var groups []uuid.UUID
	for _, o := range owners {
		if o.SubjectKind == catalogtypes.SubjectGroup {
			groups = append(groups, o.SubjectID)
		}
	}
	if len(groups) == 0 {
		return owners, nil
	}
	rows, err := postgres.List(ctx, q, constraints, scanContact, `
		SELECT group_id, kind, value, label
		FROM group_contacts
		WHERE group_id = ANY($1)
		ORDER BY group_id, position`, groups)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		for i := range owners {
			if owners[i].SubjectKind == catalogtypes.SubjectGroup && owners[i].SubjectID == r.groupID {
				owners[i].Contacts = append(owners[i].Contacts, r.contact)
			}
		}
	}
	return owners, nil
}
