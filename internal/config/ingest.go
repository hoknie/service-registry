package config

type IngestConfig struct {
	RetentionDays      uint32 `env:"INGEST_EVENT_RETENTION_DAYS" envDefault:"90" validate:"min=1,max=3650"`
	RateLimitPerMinute uint32 `env:"INGEST_RATE_LIMIT_PER_MINUTE" envDefault:"600" validate:"min=1,max=100000"`
}
