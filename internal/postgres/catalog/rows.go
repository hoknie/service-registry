package catalog

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/catalog"
	"svc-registry/internal/postgres"
)

var nodeColumns = "n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, " +
	"n.forge, n.repo_url, n.default_branch, n.cluster_observation, " +
	postgres.RFC3339("n.created_at") + " AS created_at, " + postgres.RFC3339("n.updated_at") + " AS updated_at"

const boundCTE = "bound AS (SELECT node_id, role FROM role_bindings WHERE user_id = $1 " +
	"UNION ALL SELECT b.node_id, b.role FROM role_bindings b " +
	"JOIN group_members gm ON gm.group_id = b.group_id WHERE gm.user_id = $1), " +
	"lineage AS (SELECT n.id, n.parent_id FROM nodes n WHERE n.id IN (SELECT node_id FROM bound) " +
	"UNION SELECT p.id, p.parent_id FROM nodes p JOIN lineage l ON p.id = l.parent_id)"

const kindRank = "CASE n.kind WHEN 'organization' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END"

type nodeRow struct {
	ID            *uuid.UUID    `db:"id"`
	Kind          *string       `db:"kind"`
	ParentID      *uuid.UUID    `db:"parent_id"`
	Slug          *string       `db:"slug"`
	Name          *string       `db:"name"`
	Description   *string       `db:"description"`
	Labels        domain.Labels `db:"labels"`
	Forge         *string       `db:"forge"`
	RepoURL       *string       `db:"repo_url"`
	DefaultBranch *string       `db:"default_branch"`
	ClusterObs    *bool         `db:"cluster_observation"`
	CreatedAt     *string       `db:"created_at"`
	UpdatedAt     *string       `db:"updated_at"`
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func internal(format string, v string) error {
	return &domain.InternalError{Detail: "unknown " + format + " " + v}
}

func (r nodeRow) node() (domain.Node, error) {
	kind, ok := domain.ParseKind(str(r.Kind))
	if !ok {
		return domain.Node{}, internal("kind", str(r.Kind))
	}
	labels := r.Labels
	if labels == nil {
		labels = domain.Labels{}
	}
	repo := domain.Repo{RepoURL: r.RepoURL, DefaultBranch: r.DefaultBranch}
	if r.Forge != nil {
		f, ok := domain.ParseForge(*r.Forge)
		if !ok {
			return domain.Node{}, internal("forge", *r.Forge)
		}
		repo.Forge = &f
	}
	return domain.Node{
		ID: *r.ID, Kind: kind, ParentID: r.ParentID, Slug: str(r.Slug), Name: str(r.Name),
		Description: str(r.Description), Labels: labels, Repo: repo,
		ClusterObservation: r.ClusterObs == nil || *r.ClusterObs,
		CreatedAt:          str(r.CreatedAt), UpdatedAt: str(r.UpdatedAt),
	}, nil
}

func oneNode(rows pgx.Rows) (domain.Node, error) {
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nodeRow])
	if err != nil {
		return domain.Node{}, err
	}
	return r.node()
}

func roleOfRank(rank *int32) *domain.Role {
	if rank == nil {
		return nil
	}
	var r domain.Role
	switch *rank {
	case 3:
		r = domain.RoleAdmin
	case 2:
		r = domain.RoleEditor
	case 1:
		r = domain.RoleViewer
	default:
		return nil
	}
	return &r
}

func labelsParam(l domain.Labels) domain.Labels {
	if l == nil {
		return domain.Labels{}
	}
	return l
}

func constraint(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.ConstraintName
	}
	return ""
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &conflict) || errors.As(err, &internalErr) || errors.Is(err, domain.ErrNotFound) {
		return err
	}
	switch constraint(err) {
	case "nodes_parent_slug_key":
		return domain.ConflictSlugTaken
	case "nodes_parent_id_fkey", "role_bindings_node_id_fkey", "project_keys_project_id_fkey":
		return domain.ErrNotFound
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}
