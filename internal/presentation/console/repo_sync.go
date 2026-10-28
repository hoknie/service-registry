package console

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	reposyncservices "svc-registry/internal/feature/reposync/services"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

func repoSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repo:sync <connection-id>",
		Short: "Run one repository synchronization of a connection now",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid connection id %q", args[0])
			}
			return failed(repoSync(cmd, id))
		},
	}
}

func repoSync(cmd *cobra.Command, id uuid.UUID) error {
	cfg, err := config.Load[config.Config](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(cfg.Log)
	registry, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer registry.Close()
	run, err := registry.RepoSync.Sync.RunSyncNow(context.Background(), id)
	switch {
	case errors.Is(err, reposyncservices.ErrSyncBusy):
		return err
	case err != nil:
		code, message := apperr.From(err).Public()
		if code == "not_found" {
			return fmt.Errorf("no repository sync connection %s", id)
		}
		return fmt.Errorf("sync failed: %s (%s)", code, message)
	}
	c := run.Counts
	fmt.Fprintf(cmd.OutOrStdout(), "%s created=%d updated=%d orphaned=%d skipped=%d\n",
		run.Status, c.Created, c.Updated, c.Orphaned, c.Skipped)
	if run.Status == reposynctypes.RunFailed {
		code := ""
		if run.ErrorCode != nil {
			code = *run.ErrorCode
		}
		return fmt.Errorf("sync failed: %s", code)
	}
	return nil
}
