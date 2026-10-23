package console

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
)

func secretsAdoptLegacyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "secrets:adopt-legacy",
		Short: "Move inline credentials of earlier versions into named secrets (removed in 1.0.0)",
		Long: "Move inline credentials of earlier versions into named secrets.\n\n" +
			"This command and the db:migrate check before dropping the inline credential columns will be removed in 1.0.0. " +
			"Upgrade an installation that still has inline credentials through a 0.x release first.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failed(secretsAdoptLegacy(cmd, dryRun))
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without changing the database")
	return cmd
}

func secretsAdoptLegacy(cmd *cobra.Command, dryRun bool) error {
	type slice struct {
		DB  config.DbConfig
		Log config.LogConfig
	}
	cfg, err := config.Load[slice](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	keys, err := config.Load[config.SecretsConfig](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(cfg.Log)
	adopt, closeDB, err := app.OpenLegacyAdoption(cfg.DB, keys)
	if err != nil {
		return err
	}
	defer closeDB()
	out, err := adopt(context.Background(), dryRun)
	if err != nil {
		return fmt.Errorf("cannot adopt inline credentials: %w", err)
	}
	w := cmd.OutOrStdout()
	if dryRun {
		for _, a := range out.Adopted {
			state := "reused"
			if a.Created {
				state = "new"
			}
			fmt.Fprintf(w, "%s %s → %s (%s)\n", a.Kind, a.RecordID, a.Secret, state)
		}
		fmt.Fprintf(w, "would adopt %d credential(s)\n", out.Credentials)
		return nil
	}
	fmt.Fprintf(w, "adopted %d credential(s) into %d secret(s)\n", out.Credentials, out.Secrets)
	return nil
}
