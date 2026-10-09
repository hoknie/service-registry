package config

type BranchesConfig struct {
	RetentionDays  uint32 `env:"BRANCH_RETENTION_DAYS" envDefault:"30" validate:"min=1,max=3650"`
	StaleDays      uint32 `env:"BRANCH_STALE_DAYS" envDefault:"90" validate:"min=1,max=3650"`
	SyncMaxPerRepo uint32 `env:"BRANCH_SYNC_MAX_PER_REPO" envDefault:"500" validate:"min=1,max=10000"`
}
