package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

type nodeRow struct {
	ID            *uuid.UUID     `db:"id"`
	Kind          *string        `db:"kind"`
	ParentID      *uuid.UUID     `db:"parent_id"`
	Slug          *string        `db:"slug"`
	Name          *string        `db:"name"`
	Description   *string        `db:"description"`
	Labels        catalog.Labels `db:"labels"`
	Forge         *string        `db:"forge"`
	RepoURL       *string        `db:"repo_url"`
	DefaultBranch *string        `db:"default_branch"`
	ClusterObs    *bool          `db:"cluster_observation"`
	CreatedAt     *string        `db:"created_at"`
	UpdatedAt     *string        `db:"updated_at"`
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func internal(format string, v string) error {
	return &catalog.InternalError{Detail: "unknown " + format + " " + v}
}

func (r nodeRow) node() (catalog.Node, error) {
	kind, ok := catalog.ParseKind(str(r.Kind))
	if !ok {
		return catalog.Node{}, internal("kind", str(r.Kind))
	}
	labels := r.Labels
	if labels == nil {
		labels = catalog.Labels{}
	}
	repo := catalog.Repo{RepoURL: r.RepoURL, DefaultBranch: r.DefaultBranch}
	if r.Forge != nil {
		f, ok := catalog.ParseForge(*r.Forge)
		if !ok {
			return catalog.Node{}, internal("forge", *r.Forge)
		}
		repo.Forge = &f
	}
	return catalog.Node{
		ID: *r.ID, Kind: kind, ParentID: r.ParentID, Slug: str(r.Slug), Name: str(r.Name),
		Description: str(r.Description), Labels: labels, Repo: repo,
		ClusterObservation: r.ClusterObs == nil || *r.ClusterObs,
		CreatedAt:          str(r.CreatedAt), UpdatedAt: str(r.UpdatedAt),
	}, nil
}

func oneNode(rows pgx.Rows) (catalog.Node, error) {
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nodeRow])
	if err != nil {
		return catalog.Node{}, err
	}
	return r.node()
}

func roleOfRank(rank *int32) *catalog.Role {
	if rank == nil {
		return nil
	}
	var r catalog.Role
	switch *rank {
	case 3:
		r = catalog.RoleAdmin
	case 2:
		r = catalog.RoleEditor
	case 1:
		r = catalog.RoleViewer
	default:
		return nil
	}
	return &r
}

func labelsParam(l catalog.Labels) catalog.Labels {
	if l == nil {
		return catalog.Labels{}
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
	var conflict catalog.Conflict
	var internalErr *catalog.InternalError
	if errors.As(err, &conflict) || errors.As(err, &internalErr) || errors.Is(err, catalog.ErrNotFound) {
		return err
	}
	switch constraint(err) {
	case "nodes_parent_slug_key":
		return catalog.ConflictSlugTaken
	case "nodes_parent_id_fkey", "role_bindings_node_id_fkey", "project_keys_project_id_fkey":
		return catalog.ErrNotFound
	}
	if apperr.IsUnavailable(err) {
		return catalog.ErrUnavailable
	}
	return &catalog.InternalError{Detail: err.Error()}
}

type containedRow struct {
	ContainerID uuid.UUID `db:"container_id"`
	ProjectID   uuid.UUID `db:"project_id"`
}
