package cli

import (
	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/config"
	"svc-registry/internal/service"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server (does not migrate)",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := config.FromEnv()
			if err != nil {
				return invalidConfig(err)
			}
			if admin := cfg.Bootstrap.Admin; admin != nil {
				if err := service.CheckBootstrapAdmin(*admin); err != nil {
					return invalidConfig(err)
				}
			}
			app.InitLogging(cfg.Log)
			return failed(app.Serve(cfg))
		},
	}
}
