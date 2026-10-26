package repositories

import (
	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	publishingtypes "svc-registry/internal/feature/publishing/types"
)

type versionRow struct {
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"`
}

func versionsOf(rows *[]versionRow) *[]publishingtypes.Version {
	if rows == nil {
		return nil
	}
	out := make([]publishingtypes.Version, len(*rows))
	for i, r := range *rows {
		out[i] = publishingtypes.Version{Pattern: r.Pattern, Kind: knowledgetypes.RefKind(r.Kind)}
	}
	return &out
}

func versionRows(versions *[]publishingtypes.Version) *[]versionRow {
	if versions == nil {
		return nil
	}
	out := make([]versionRow, len(*versions))
	for i, v := range *versions {
		out[i] = versionRow{Pattern: v.Pattern, Kind: string(v.Kind)}
	}
	return &out
}

type nodeRow struct {
	ID             uuid.UUID     `db:"id"`
	ParentID       *uuid.UUID    `db:"parent_id"`
	Kind           string        `db:"kind"`
	Slug           string        `db:"slug"`
	Name           string        `db:"name"`
	Description    string        `db:"description"`
	Publish        *bool         `db:"publish"`
	Versions       *[]versionRow `db:"versions"`
	DefaultBranch  *bool         `db:"default_branch"`
	CurrentRelease *bool         `db:"current_release"`
	PublicName     *string       `db:"public_name"`
	Host           *string       `db:"host"`
}

func (r nodeRow) own() publishingtypes.NodeSettings {
	return publishingtypes.NodeSettings{Publish: r.Publish, Versions: versionsOf(r.Versions), DefaultBranch: r.DefaultBranch,
		CurrentRelease: r.CurrentRelease, Name: r.PublicName, Host: r.Host}
}

func (r nodeRow) chain() publishingtypes.ChainNode {
	return publishingtypes.ChainNode{ID: r.ID, ParentID: r.ParentID, Kind: r.Kind, Slug: r.Slug, Name: r.Name, Own: r.own()}
}

func (r nodeRow) node() publishingtypes.Node {
	return publishingtypes.Node{ID: r.ID, ParentID: r.ParentID, Kind: r.Kind, Slug: r.Slug, Name: r.Name, Description: r.Description, Own: r.own()}
}
