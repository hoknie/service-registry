package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/apperr"
	"svc-registry/internal/config"
	"svc-registry/internal/forge"
	"svc-registry/internal/httpapi"
	"svc-registry/internal/service"
)

func forgeSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forge:sync <connection-id>",
		Short: "Run one forge synchronization of a connection now",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid connection id %q", args[0])
			}
			return failed(forgeSync(cmd, id))
		},
	}
}

func forgeSync(cmd *cobra.Command, id uuid.UUID) error {
	cfg, err := config.Load[config.Config](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(cfg.Log)
	state, err := app.BuildState(cfg)
	if err != nil {
		return err
	}
	defer state.DB.Close()
	run, err := service.RunSyncNow(context.Background(), state, id)
	switch {
	case errors.Is(err, service.ErrSyncBusy):
		return err
	case err != nil:
		api := httpapi.FromApp(apperr.From(err))
		if api.Code == "not_found" {
			return fmt.Errorf("no forge connection %s", id)
		}
		return fmt.Errorf("sync failed: %s (%s)", api.Code, api.Message)
	}
	c := run.Counts
	fmt.Fprintf(cmd.OutOrStdout(), "%s created=%d updated=%d orphaned=%d skipped=%d\n",
		run.Status, c.Created, c.Updated, c.Orphaned, c.Skipped)
	if run.Status == forge.RunFailed {
		code := ""
		if run.ErrorCode != nil {
			code = *run.ErrorCode
		}
		return fmt.Errorf("sync failed: %s", code)
	}
	return nil
}
