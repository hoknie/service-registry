package config

type JobsConfig struct {
	Enabled           bool   `env:"BACKGROUND_JOBS_ENABLED" envDefault:"true"`
	ForgeConcurrency  uint32 `env:"FORGE_SYNC_CONCURRENCY" envDefault:"2" validate:"min=1,max=32"`
	ForgeIntervalSecs uint32 `env:"FORGE_SYNC_INTERVAL_SECS" envDefault:"900" validate:"min=60,max=86400"`
	ForgeRunsKept     uint32 `env:"FORGE_SYNC_RUNS_KEPT" envDefault:"20" validate:"min=1,max=1000"`
}
