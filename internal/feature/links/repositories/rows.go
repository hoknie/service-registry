package repositories

import (
	"github.com/google/uuid"

	linkstypes "svc-registry/internal/feature/links/types"
)

type kindRow struct {
	ID        uuid.UUID         `db:"id"`
	Key       string            `db:"key"`
	Names     map[string]string `db:"names"`
	Icon      string            `db:"icon"`
	IconURL   *string           `db:"icon_url"`
	Position  int32             `db:"position"`
	CreatedAt string            `db:"created_at"`
	UpdatedAt string            `db:"updated_at"`
}

func (r kindRow) kind() linkstypes.Kind {
	return linkstypes.Kind{ID: r.ID, Key: r.Key, Names: r.Names, Icon: r.Icon, IconURL: r.IconURL, Position: r.Position,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type templateRow struct {
	ID           uuid.UUID `db:"id"`
	NodeID       uuid.UUID `db:"node_id"`
	LinkKey      string    `db:"link_key"`
	KindKey      string    `db:"kind_key"`
	KindPosition int32     `db:"kind_position"`
	Template     *string   `db:"template"`
	Disabled     bool      `db:"disabled"`
	Position     int32     `db:"position"`
	Title        *string   `db:"title"`
	IconURL      *string   `db:"icon_url"`
	IconFile     *string   `db:"icon_file"`
	KindIcon     string    `db:"kind_icon"`
	KindIconURL  *string   `db:"kind_icon_url"`
	CreatedAt    string    `db:"created_at"`
	UpdatedAt    string    `db:"updated_at"`
	Inherited    bool      `db:"inherited"`
}

func (r templateRow) template() linkstypes.Template { return linkstypes.Template(r) }

type varRow struct {
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	NodeID    uuid.UUID `db:"node_id"`
	Inherited bool      `db:"inherited"`
}

type deploymentRow struct {
	Service     string                 `db:"service"`
	Environment string                 `db:"environment"`
	Version     string                 `db:"version"`
	Commit      *string                `db:"commit_sha"`
	Cluster     *string                `db:"cluster"`
	Namespace   *string                `db:"namespace"`
	URL         *string                `db:"url"`
	Links       []linkstypes.EventLink `db:"links"`
}

type checkRow struct {
	URL        string `db:"url"`
	Status     string `db:"status"`
	HTTPStatus *int   `db:"http_status"`
	DurationMS int    `db:"duration_ms"`
	CheckedAt  string `db:"checked_at"`
}

func (r checkRow) check() linkstypes.Check {
	return linkstypes.Check{URL: r.URL, Status: linkstypes.Status(r.Status), HTTPStatus: r.HTTPStatus,
		DurationMS: r.DurationMS, CheckedAt: r.CheckedAt}
}
