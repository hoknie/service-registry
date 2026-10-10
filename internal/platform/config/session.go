package config

import "fmt"

type SessionConfig struct {
	CookieSecure        bool   `env:"SESSION_COOKIE_SECURE" envDefault:"true"`
	IdleTimeoutSecs     uint64 `env:"SESSION_IDLE_TIMEOUT_SECS" envDefault:"43200" validate:"min=60,max=2592000"`
	AbsoluteTimeoutSecs uint64 `env:"SESSION_ABSOLUTE_TIMEOUT_SECS" envDefault:"604800" validate:"min=60,max=31536000"`
}

func (c *SessionConfig) check() Errors {
	if c.AbsoluteTimeoutSecs < c.IdleTimeoutSecs {
		return Errors{{Var: "SESSION_ABSOLUTE_TIMEOUT_SECS", Reason: fmt.Sprintf(
			"must not be less than SESSION_IDLE_TIMEOUT_SECS (%d), got %d", c.IdleTimeoutSecs, c.AbsoluteTimeoutSecs)}}
	}
	return nil
}
