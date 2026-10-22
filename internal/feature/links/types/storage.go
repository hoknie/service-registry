package types

import (
	"github.com/google/uuid"
)

type ProjectData struct {
	Templates    []Template
	Vars         []Var
	RepoFullPath *string
	Deployments  []Deployment
	Kinds        map[string]KindInfo
}

type Target struct {
	ID  uuid.UUID
	URL string
}
