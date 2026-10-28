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
	Hosting       *string             `db:"hosting"`
	RepoURL       *string             `db:"repo_url"`
	DefaultBranch *string             `db:"default_branch"`
	ClusterObs    *bool               `db:"cluster_observation"`
	Visibility    *string             `db:"visibility"`
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
	if r.Hosting != nil {
		f, ok := catalogtypes.ParseHosting(*r.Hosting)
		if !ok {
			return catalogtypes.Node{}, apperr.UnknownValue("hosting", *r.Hosting)
		}
		repo.Hosting = &f
	}
	return catalogtypes.Node{
		ID: *r.ID, Kind: kind, ParentID: r.ParentID, Slug: ptr.Value(r.Slug), Name: ptr.Value(r.Name),
		Description: ptr.Value(r.Description), Labels: labels, Repo: repo,
		ClusterObservation: r.ClusterObs == nil || *r.ClusterObs,
		Visibility:         catalogtypes.OrgVisibility(ptr.Value(r.Visibility)),
		CreatedAt:          ptr.Value(r.CreatedAt), UpdatedAt: ptr.Value(r.UpdatedAt),
	}, nil
}

func scanNode(row pgx.CollectableRow) (catalogtypes.Node, error) {
	r, err := pgx.RowToStructByName[nodeRow](row)
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

type keyUseRow struct {
	ID    uuid.UUID `db:"id"`
	Stale bool      `db:"stale"`
}

func (r keyUseRow) activeKey() catalogtypes.ActiveKey {
	return catalogtypes.ActiveKey{ID: r.ID, Stale: r.Stale}
}

type containedRow struct {
	ContainerID uuid.UUID `db:"container_id"`
	ProjectID   uuid.UUID `db:"project_id"`
}

func scanKey(row pgx.CollectableRow) (catalogtypes.ProjectKey, error) {
	var k catalogtypes.ProjectKey
	var status string
	if err := row.Scan(&k.ID, &k.ProjectID, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &status); err != nil {
		return catalogtypes.ProjectKey{}, err
	}
	st, ok := catalogtypes.ParseKeyStatus(status)
	if !ok {
		return catalogtypes.ProjectKey{}, apperr.UnknownValue("key status", status)
	}
	k.Status = st
	return k, nil
}

func scanBinding(row pgx.CollectableRow) (catalogtypes.Binding, error) {
	var b catalogtypes.Binding
	var isGroup bool
	var role string
	if err := row.Scan(&b.ID, &b.NodeID, &b.NodeName, &isGroup, &b.SubjectID, &b.SubjectName, &role, &b.CreatedAt); err != nil {
		return catalogtypes.Binding{}, err
	}
	b.SubjectKind = catalogtypes.SubjectUser
	if isGroup {
		b.SubjectKind = catalogtypes.SubjectGroup
	}
	r, ok := catalogtypes.ParseRole(role)
	if !ok {
		return catalogtypes.Binding{}, apperr.UnknownValue("role", role)
	}
	b.Role = r
	return b, nil
}

type groupAccessRow struct {
	ID    uuid.UUID `db:"id"`
	Name  string    `db:"name"`
	Role  string    `db:"role"`
	Users int64     `db:"users"`
}
