package config

import (
	"net/mail"
	"net/url"
	"strings"
	"time"
)

type MailConfig struct {
	URLRaw         string        `env:"SMTP_URL"`
	FromRaw        string        `env:"SMTP_FROM"`
	PasswordRaw    string        `env:"SMTP_PASSWORD"`
	TimeoutSecs    uint32        `env:"SMTP_TIMEOUT_SECS" envDefault:"10" validate:"min=1,max=60"`
	InviteTTLHours uint32        `env:"INVITE_TTL_HOURS" envDefault:"168" validate:"min=1,max=720"`
	Server         *url.URL      `env:"-"`
	From           *mail.Address `env:"-"`
	Username       string        `env:"-"`
	Password       string        `env:"-"`
}

func (c MailConfig) Enabled() bool { return c.Server != nil }

func (c MailConfig) Timeout() time.Duration { return time.Duration(c.TimeoutSecs) * time.Second }

func (c MailConfig) InviteTTL() time.Duration { return time.Duration(c.InviteTTLHours) * time.Hour }

func (c *MailConfig) check() Errors {
	raw := strings.TrimSpace(c.URLRaw)
	if raw == "" {
		return nil
	}
	var errs Errors
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "smtp" && u.Scheme != "smtps") || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" {
		return Errors{&Error{Var: "SMTP_URL", Reason: "must be smtp://[user[:password]@]host[:port] or smtps://…"}}
	}
	c.Server = &url.URL{Scheme: u.Scheme, Host: u.Host}
	if u.User != nil {
		c.Username = u.User.Username()
		c.Password, _ = u.User.Password()
	}
	if pw := strings.TrimSpace(c.PasswordRaw); pw != "" {
		if c.Password != "" {
			errs = append(errs, &Error{Var: "SMTP_PASSWORD", Reason: "cannot be set together with a password in SMTP_URL"})
		}
		value := pw
		if strings.HasPrefix(pw, "env:") || strings.HasPrefix(pw, "file:") {
			if value, err = ResolveSecretRef(pw); err != nil {
				errs = append(errs, &Error{Var: "SMTP_PASSWORD", Reason: "reference cannot be resolved: " + err.Error()})
			}
		}
		c.Password = value
	}
	from := strings.TrimSpace(c.FromRaw)
	if from == "" {
		return append(errs, &Error{Var: "SMTP_FROM", Reason: "is required when SMTP_URL is set"})
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return append(errs, &Error{Var: "SMTP_FROM", Reason: "must be an address like name <addr@domain> or addr@domain"})
	}
	c.From = addr
	return errs
}
