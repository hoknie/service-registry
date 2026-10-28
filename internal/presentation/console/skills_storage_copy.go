package console

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
)

func skillsStorageCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skills:storage-copy",
		Short: "Copy the skill files the database refers to from SKILLS_DIR to the S3 bucket of SKILLS_S3_*",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failed(skillsStorageCopy(cmd))
		},
	}
}

func skillsStorageCopy(cmd *cobra.Command) error {
	type slice struct {
		DB       config.DbConfig
		Log      config.LogConfig
		Outbound config.OutboundConfig
	}
	cfg, err := config.Load[slice](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	skills, err := config.Load[config.SkillsConfig](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	if errs := skills.CheckS3(); len(errs) > 0 {
		return invalidConfig(errs)
	}
	app.InitLogging(cfg.Log)
	copyAll, closeDB, err := app.OpenSkillsStorageCopy(cfg.DB, skills, cfg.Outbound)
	if err != nil {
		return err
	}
	defer closeDB()
	n, err := copyAll(context.Background())
	if err != nil {
		return fmt.Errorf("cannot copy skill files after %d copied: %w", n.Copied, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "copied %d blob(s), skipped %d\n", n.Copied, n.Skipped)
	return nil
}
