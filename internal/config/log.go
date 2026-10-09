package config

import (
	"fmt"
	"strings"
)

type LogConfig struct {
	Filter string `env:"LOG_LEVEL" envDefault:"info"`
}

var levels = map[string]bool{"off": true, "error": true, "warn": true, "info": true, "debug": true, "trace": true}

func (c *LogConfig) check() Errors {
	c.Filter = strings.ToLower(strings.TrimSpace(c.Filter))
	for _, directive := range strings.Split(c.Filter, ",") {
		_, level, _ := strings.Cut(directive, "=")
		if !strings.Contains(directive, "=") {
			level = directive
		}
		if !levels[strings.TrimSpace(level)] {
			return Errors{{Var: "LOG_LEVEL", Reason: fmt.Sprintf(
				"expected a level (off|error|warn|info|debug|trace) or target=level list, got %q", c.Filter)}}
		}
	}
	return nil
}
