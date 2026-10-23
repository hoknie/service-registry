package repositories

import (
	"github.com/google/uuid"

	topologytypes "svc-registry/internal/feature/topology/types"
)

type edgeRow struct {
	ID              uuid.UUID  `db:"id"`
	ProjectID       uuid.UUID  `db:"project_id"`
	Source          string     `db:"source"`
	Origin          string     `db:"origin"`
	TargetProjectID *uuid.UUID `db:"target_project_id"`
	TargetExternal  *string    `db:"target_external"`
	TargetRef       string     `db:"target_ref"`
	Note            *string    `db:"note"`
	SeenAt          string     `db:"seen_at"`
}

func (r edgeRow) edge() topologytypes.Edge {
	return topologytypes.Edge{ID: r.ID, ProjectID: r.ProjectID, Source: topologytypes.Source(r.Source), Origin: r.Origin,
		TargetProjectID: r.TargetProjectID, TargetExternal: r.TargetExternal, TargetRef: r.TargetRef, Note: r.Note, SeenAt: r.SeenAt}
}
