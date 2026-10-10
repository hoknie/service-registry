package config

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"svc-registry/pkg/secretbox"
)

type SecretsConfig struct {
	Raw  string      `env:"SECRETS_KEYS"`
	Keys []SecretKey `env:"-"`
}

type SecretKey = secretbox.Key

var secretKeyID = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

func (c *SecretsConfig) check() Errors {
	c.Keys = nil
	raw := strings.TrimSpace(c.Raw)
	if raw == "" {
		return nil
	}
	seen := map[string]bool{}
	for i, part := range strings.Split(raw, ",") {
		id, b64, ok := strings.Cut(strings.TrimSpace(part), ":")
		bad := func(reason string) Errors {
			c.Keys = nil
			return Errors{{Var: "SECRETS_KEYS", Reason: fmt.Sprintf("key %d: %s", i+1, reason)}}
		}
		if !ok || !secretKeyID.MatchString(id) {
			return bad("must be <id>:<base64>, id of 1..16 characters a-z0-9")
		}
		if seen[id] {
			return bad("repeats id " + id)
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			key, err = base64.RawURLEncoding.DecodeString(b64)
		}
		if err != nil || len(key) != 32 {
			return bad("must be base64 of exactly 32 bytes")
		}
		seen[id] = true
		k := SecretKey{ID: id}
		copy(k.Key[:], key)
		c.Keys = append(c.Keys, k)
	}
	return nil
}

func (c SecretsConfig) String() string { return fmt.Sprintf("SecretsConfig{%d key(s)}", len(c.Keys)) }

func (c SecretsConfig) GoString() string { return c.String() }
