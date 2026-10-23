package config

import "svc-registry/pkg/netguard"

type NotifyConfig struct {
	Concurrency    uint32   `env:"NOTIFY_CONCURRENCY" envDefault:"4" validate:"min=1,max=64"`
	TimeoutSecs    uint32   `env:"NOTIFY_TIMEOUT_SECS" envDefault:"10" validate:"min=1,max=60"`
	MaxAttempts    uint32   `env:"NOTIFY_MAX_ATTEMPTS" envDefault:"8" validate:"min=1,max=8"`
	RetentionDays  uint32   `env:"NOTIFY_RETENTION_DAYS" envDefault:"30" validate:"min=1,max=365"`
	DriftAfterSecs uint32   `env:"NOTIFY_DRIFT_AFTER_SECS" envDefault:"900" validate:"min=60,max=86400"`
	AllowPrivate   bool     `env:"NOTIFY_ALLOW_PRIVATE" envDefault:"true"`
	AllowHostsRaw  string   `env:"NOTIFY_ALLOW_HOSTS"`
	DenyHostsRaw   string   `env:"NOTIFY_DENY_HOSTS"`
	UseProxy       bool     `env:"NOTIFY_USE_PROXY" envDefault:"false"`
	AllowHosts     []string `env:"-"`
	DenyHosts      []string `env:"-"`
}

func (c *NotifyConfig) check() Errors {
	var errs Errors
	var ok bool
	if c.AllowHosts, ok = hostPatterns(c.AllowHostsRaw); !ok {
		errs = append(errs, &Error{Var: "NOTIFY_ALLOW_HOSTS", Reason: "must be host names or *.domain separated by commas"})
	}
	if c.DenyHosts, ok = hostPatterns(c.DenyHostsRaw); !ok {
		errs = append(errs, &Error{Var: "NOTIFY_DENY_HOSTS", Reason: "must be host names or *.domain separated by commas"})
	}
	return errs
}

func (c NotifyConfig) Policy() netguard.Policy {
	return netguard.Policy{AllowPrivate: c.AllowPrivate, AllowHosts: c.AllowHosts, DenyHosts: c.DenyHosts}
}
