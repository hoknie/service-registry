package console

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/platform/postgres"
)

func dbRollbackCmd() *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "db:rollback",
		Short: "Revert the last N applied migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if count < 1 {
				return fmt.Errorf("--count expects a positive integer, got %d", count)
			}
			cfg, err := dbConfig()
			if err != nil {
				return err
			}
			reverted, err := postgres.Rollback(context.Background(), cfg, count)
			if err != nil {
				return failed(fmt.Errorf("rollback failed: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reverted %d migration(s)\n", reverted)
			return nil
		},
	}
	cmd.Flags().IntVarP(&count, "count", "n", 1, "how many migrations to revert")
	return cmd
}
