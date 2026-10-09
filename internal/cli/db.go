package cli

import (
	"svc-registry/internal/app"
	"svc-registry/internal/config"
)

func dbConfig() (config.DbConfig, error) {
	logCfg, err := config.Load[config.LogConfig](config.ProcessEnv())
	if err != nil {
		return config.DbConfig{}, invalidConfig(err)
	}
	app.InitLogging(logCfg)
	cfg, err := config.Load[config.DbConfig](config.ProcessEnv())
	if err != nil {
		return config.DbConfig{}, invalidConfig(err)
	}
	return cfg, nil
}
