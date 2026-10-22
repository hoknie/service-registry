package repositories

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/ptr"
)

type nodeRow struct {
	ID            *uuid.UUID          `db:"id"`
	Kind          *string             `db:"kind"`
	ParentID      *uuid.UUID          `db:"parent_id"`
	Slug          *string             `db:"slug"`
	Name          *string             `db:"name"`
	Description   *string             `db:"description"`
	Labels        catalogtypes.Labels `db:"labels"`
	Forge         *string             `db:"forge"`
	RepoURL       *string             `db:"repo_url"`
	DefaultBranch *string             `db:"default_branch"`
	ClusterObs    *bool               `db:"cluster_observation"`
	CreatedAt     *string             `db:"created_at"`
	UpdatedAt     *string             `db:"updated_at"`
}

func (r nodeRow) node() (catalogtypes.Node, error) {
	kind, ok := catalogtypes.ParseKind(ptr.Value(r.Kind))
	if !ok {
		return catalogtypes.Node{}, apperr.UnknownValue("kind", ptr.Value(r.Kind))
	}
	labels := r.Labels
	if labels == nil {
		labels = catalogtypes.Labels{}
	}
	repo := catalogtypes.Repo{RepoURL: r.RepoURL, DefaultBranch: r.DefaultBranch}
	if r.Forge != nil {
		f, ok := catalogtypes.ParseForge(*r.Forge)
		if !ok {
			return catalogtypes.Node{}, apperr.UnknownValue("forge", *r.Forge)
		}
		repo.Forge = &f
	}
	return catalogtypes.Node{
		ID: *r.ID, Kind: kind, ParentID: r.ParentID, Slug: ptr.Value(r.Slug), Name: ptr.Value(r.Name),
		Description: ptr.Value(r.Description), Labels: labels, Repo: repo,
		ClusterObservation: r.ClusterObs == nil || *r.ClusterObs,
		CreatedAt:          ptr.Value(r.CreatedAt), UpdatedAt: ptr.Value(r.UpdatedAt),
	}, nil
}

func oneNode(rows pgx.Rows) (catalogtypes.Node, error) {
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nodeRow])
	if err != nil {
		return catalogtypes.Node{}, err
	}
	return r.node()
}

func roleOfRank(rank *int32) *catalogtypes.Role {
	if rank == nil {
		return nil
	}
	var r catalogtypes.Role
	switch *rank {
	case 3:
		r = catalogtypes.RoleAdmin
	case 2:
		r = catalogtypes.RoleEditor
	case 1:
		r = catalogtypes.RoleViewer
	default:
		return nil
	}
	return &r
}

func labelsParam(l catalogtypes.Labels) catalogtypes.Labels {
	if l == nil {
		return catalogtypes.Labels{}
	}
	return l
}

type containedRow struct {
	ContainerID uuid.UUID `db:"container_id"`
	ProjectID   uuid.UUID `db:"project_id"`
}
