package repositories

import (
	"context"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OwnerRepository struct{ db *pgtx.DB }

func NewOwnerRepository(db *pgtx.DB) *OwnerRepository { return &OwnerRepository{db: db} }

func (s *OwnerRepository) Replace(ctx context.Context, nodeID uuid.UUID, owners []ownershiptypes.NewOwner, none bool) error {
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
			LEFT JOIN groups g ON x.kind = 'group' AND lower(g.name) = lower(x.subject)
			LEFT JOIN users u ON x.kind = 'user' AND lower(u.email) = lower(x.subject)
			ORDER BY x.ord`, kinds, subjects)
		if err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, r := range resolved {
			id := r.groupID
			if id == nil {
				id = r.userID
			}
			if id == nil {
				return catalogtypes.InvalidUnknownSubject
			}
			if seen[*id] {
				return ownershiptypes.InvalidOwners
			}
			seen[*id] = true
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			DELETE FROM node_owners
			WHERE node_id = $1`, nodeID); err != nil {
			return err
		}
		if err := setCleared(ctx, tx, nodeID, none); err != nil {
			return err
		}
		for i, r := range resolved {
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

func (s *OwnerRepository) Effective(ctx context.Context, nodeID uuid.UUID) (*ownershiptypes.Source, []ownershiptypes.Owner, error) {
	q := s.db.From(ctx)
	src, err := postgres.Find(ctx, q, constraints, scanSource, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT up.id, NOT EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = up.id) AS none
		FROM up
		WHERE EXISTS (SELECT 1 FROM node_owners o WHERE o.node_id = up.id)
			OR EXISTS (SELECT 1 FROM node_owner_clears x WHERE x.node_id = up.id)
		ORDER BY up.lvl
		LIMIT 1`, nodeID)
	if err != nil || src == nil || src.None {
		return src, nil, err
	}
	owners, err := postgres.List(ctx, q, constraints, scanOwner, `
		SELECT o.node_id, o.node_id <> $2 AS inherited, o.group_id IS NOT NULL AS is_group,
			COALESCE(o.group_id, o.user_id) AS subject_id, COALESCE(g.name, u.display_name) AS name,
			u.email, u.status, g.on_call_url
		FROM node_owners o
		LEFT JOIN groups g ON g.id = o.group_id
		LEFT JOIN users u ON u.id = o.user_id
		WHERE o.node_id = $1
		ORDER BY o.position`, src.NodeID, nodeID)
	if err != nil {
		return nil, nil, err
	}
	owners, err = withContacts(ctx, q, owners)
	return src, owners, err
}

func setCleared(ctx context.Context, tx pgtx.Querier, nodeID uuid.UUID, none bool) error {
	if !none {
		_, err := postgres.Exec(ctx, tx, constraints, `
			DELETE FROM node_owner_clears
			WHERE node_id = $1`, nodeID)
		return err
	}
	_, err := postgres.Exec(ctx, tx, constraints, `
		INSERT INTO node_owner_clears (node_id)
		VALUES ($1)
		ON CONFLICT (node_id)
		DO NOTHING`, nodeID)
	return err
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
