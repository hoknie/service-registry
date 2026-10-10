package config

type PatConfig struct {
	MaxLifetimeDays uint32 `env:"PAT_MAX_LIFETIME_DAYS" envDefault:"365" validate:"min=1,max=3650"`
}
