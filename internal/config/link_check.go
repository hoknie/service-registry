package config

import (
	"strings"
)

type LinkCheckConfig struct {
	IntervalSecs  uint32   `env:"LINK_CHECK_INTERVAL_SECS" envDefault:"3600" validate:"min=300,max=604800"`
	TimeoutSecs   uint32   `env:"LINK_CHECK_TIMEOUT_SECS" envDefault:"5" validate:"min=1,max=60"`
	Concurrency   uint32   `env:"LINK_CHECK_CONCURRENCY" envDefault:"4" validate:"min=1,max=64"`
	AllowPrivate  bool     `env:"LINK_CHECK_ALLOW_PRIVATE" envDefault:"true"`
	AllowHostsRaw string   `env:"LINK_CHECK_ALLOW_HOSTS"`
	DenyHostsRaw  string   `env:"LINK_CHECK_DENY_HOSTS"`
	AllowHosts    []string `env:"-"`
	DenyHosts     []string `env:"-"`
	History       uint32   `env:"LINK_CHECK_HISTORY" envDefault:"20" validate:"min=1,max=1000"`
	TargetTTLDays uint32   `env:"LINK_CHECK_TARGET_TTL_DAYS" envDefault:"30" validate:"min=1,max=3650"`
}

func (c *LinkCheckConfig) check() Errors {
	var errs Errors
	var ok bool
	if c.AllowHosts, ok = hostPatterns(c.AllowHostsRaw); !ok {
		errs = append(errs, &Error{Var: "LINK_CHECK_ALLOW_HOSTS", Reason: "must be host names or *.domain separated by commas"})
	}
	if c.DenyHosts, ok = hostPatterns(c.DenyHostsRaw); !ok {
		errs = append(errs, &Error{Var: "LINK_CHECK_DENY_HOSTS", Reason: "must be host names or *.domain separated by commas"})
	}
	return errs
}

func hostPatterns(raw string) ([]string, bool) {
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if !validHostName(strings.TrimPrefix(entry, "*.")) {
			return nil, false
		}
		out = append(out, entry)
	}
	return out, true
}

func validHostName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
