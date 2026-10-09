package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/postgres"
)

func dbMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db:migrate",
		Short: "Apply pending database migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := dbConfig()
			if err != nil {
				return err
			}
			applied, err := postgres.Migrate(context.Background(), cfg)
			if err != nil {
				return failed(fmt.Errorf("migration failed: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied %d migration(s)\n", applied)
			return nil
		},
	}
}
