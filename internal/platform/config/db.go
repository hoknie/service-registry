package config

import "strings"

type DbConfig struct {
	URL                string `env:"DATABASE_URL,required"`
	MaxConnections     uint32 `env:"DATABASE_MAX_CONNECTIONS" envDefault:"10" validate:"min=1,max=1000"`
	AcquireTimeoutSecs uint64 `env:"DATABASE_ACQUIRE_TIMEOUT_SECS" envDefault:"3" validate:"min=1,max=300"`
}

func (c *DbConfig) check() Errors {
	if !strings.HasPrefix(c.URL, "postgres://") && !strings.HasPrefix(c.URL, "postgresql://") {
		return Errors{{Var: "DATABASE_URL", Reason: "must start with postgres:// or postgresql://"}}
	}
	return nil
}
