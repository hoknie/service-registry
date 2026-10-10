package links

import (
	"context"

	"github.com/google/uuid"
)

type ProjectData struct {
	Templates    []Template
	Vars         []Var
	RepoFullPath *string
	Deployments  []Deployment
}

type Target struct {
	ID  uuid.UUID
	URL string
}

type Checker interface {
	Check(ctx context.Context, url string) Result
}
