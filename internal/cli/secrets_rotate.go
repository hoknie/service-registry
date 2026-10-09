package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/config"
	"svc-registry/internal/postgres"
	deploystore "svc-registry/internal/postgres/deploy"
	forgestore "svc-registry/internal/postgres/forge"
	knowledgestore "svc-registry/internal/postgres/knowledge"
	"svc-registry/internal/service"
	"svc-registry/pkg/secretbox"
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
	pool, err := postgres.ConnectLazy(cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	state := &service.State{
		DB:          pool,
		Connections: forgestore.NewConnectionStore(pool),
		Clusters:    deploystore.NewClusterStore(pool),
		Sources:     knowledgestore.NewSourceStore(pool),
		Secrets:     secretbox.New(keys.Keys),
	}
	n, err := service.RotateSecrets(context.Background(), state)
	if err != nil {
		return fmt.Errorf("cannot rotate secrets: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "rotated %d secret(s)\n", n)
	return nil
}
