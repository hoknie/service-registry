package links

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/links"
)

type kindRow struct {
	ID        uuid.UUID         `db:"id"`
	Key       string            `db:"key"`
	Names     map[string]string `db:"names"`
	Icon      string            `db:"icon"`
	Position  int32             `db:"position"`
	CreatedAt string            `db:"created_at"`
	UpdatedAt string            `db:"updated_at"`
}

func (r kindRow) kind() domain.Kind {
	return domain.Kind{ID: r.ID, Key: r.Key, Names: r.Names, Icon: domain.Icon(r.Icon), Position: r.Position,
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
	CreatedAt    string    `db:"created_at"`
	UpdatedAt    string    `db:"updated_at"`
	Inherited    bool      `db:"inherited"`
}

func (r templateRow) template() domain.Template {
	return domain.Template{ID: r.ID, NodeID: r.NodeID, LinkKey: r.LinkKey, KindKey: r.KindKey,
		KindPosition: r.KindPosition, Template: r.Template, Disabled: r.Disabled, Position: r.Position,
		Title: r.Title, IconURL: r.IconURL, IconFile: r.IconFile, KindIcon: r.KindIcon,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Inherited: r.Inherited}
}

type varRow struct {
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	NodeID    uuid.UUID `db:"node_id"`
	Inherited bool      `db:"inherited"`
}

type deploymentRow struct {
	Service     string  `db:"service"`
	Environment string  `db:"environment"`
	Version     string  `db:"version"`
	Commit      *string `db:"commit_sha"`
	Cluster     *string `db:"cluster"`
	Namespace   *string `db:"namespace"`
	URL         *string `db:"url"`
}

type checkRow struct {
	URL        string `db:"url"`
	Status     string `db:"status"`
	HTTPStatus *int   `db:"http_status"`
	DurationMS int    `db:"duration_ms"`
	CheckedAt  string `db:"checked_at"`
}

func (r checkRow) check() domain.Check {
	return domain.Check{URL: r.URL, Status: domain.Status(r.Status), HTTPStatus: r.HTTPStatus,
		DurationMS: r.DurationMS, CheckedAt: r.CheckedAt}
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var invalid domain.Invalid
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &invalid) || errors.As(err, &conflict) || errors.Is(err, domain.ErrNotFound) || errors.As(err, &internalErr) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.ConstraintName {
		case "link_kinds_key_key":
			return domain.ConflictKindTaken
		case "link_templates_kind_id_fkey":
			return domain.ConflictKindInUse
		case "link_templates_node_id_fkey", "node_vars_node_id_fkey":
			return domain.ErrNotFound
		}
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}
