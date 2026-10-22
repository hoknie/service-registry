package console

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
)

func secretsRotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secrets:rotate",
		Short: "Re-encrypt the secrets in the database with the first key of SECRETS_KEYS",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failed(secretsRotate(cmd))
		},
	}
}

func secretsRotate(cmd *cobra.Command) error {
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
	rotate, closeDB, err := app.OpenSecretRotation(cfg.DB, keys)
	if err != nil {
		return err
	}
	defer closeDB()
	n, err := rotate(context.Background())
	if err != nil {
		return fmt.Errorf("cannot rotate secrets: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "rotated %d secret(s)\n", n)
	return nil
}
