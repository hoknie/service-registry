package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type BuiltinRepository struct{ db *pgtx.DB }

func NewBuiltinRepository(db *pgtx.DB) *BuiltinRepository { return &BuiltinRepository{db: db} }

func (s *BuiltinRepository) Disabled(ctx context.Context) ([]string, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[[]string], `
		SELECT disabled_tools
		FROM mcp_builtin_settings`)
	if err != nil || found == nil {
		return []string{}, err
	}
	return *found, nil
}

func (s *BuiltinRepository) SetDisabled(ctx context.Context, tools []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO mcp_builtin_settings (id, disabled_tools)
		VALUES (true, $1)
		ON CONFLICT (id)
		DO UPDATE SET disabled_tools = $1, updated_at = now()`, tools)
	return err
}
