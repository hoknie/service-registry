package config

type LoginLimitConfig struct {
	MaxFailures uint32 `env:"LOGIN_MAX_FAILURES" envDefault:"5" validate:"min=1,max=1000"`
	WindowSecs  uint64 `env:"LOGIN_FAILURE_WINDOW_SECS" envDefault:"900" validate:"min=1,max=86400"`
}
