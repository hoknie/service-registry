package config

import (
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type UploadsConfig struct {
	DirRaw              string `env:"UPLOADS_DIR" envDefault:"data/uploads"`
	Dir                 string `env:"-"`
	UnreferencedTTLSecs uint32 `env:"UPLOADS_UNREFERENCED_TTL_SECS" envDefault:"86400" validate:"min=600,max=2592000"`
}

func (c UploadsConfig) UnreferencedTTL() time.Duration {
	return time.Duration(c.UnreferencedTTLSecs) * time.Second
}

func (c *UploadsConfig) check() Errors {
	c.Dir = ""
	if strings.TrimSpace(c.DirRaw) == "off" {
		return nil
	}
	if slices.Contains(strings.Split(filepath.ToSlash(c.DirRaw), "/"), "..") {
		return Errors{{Var: "UPLOADS_DIR", Reason: "must not contain .."}}
	}
	c.Dir = filepath.Clean(c.DirRaw)
	return nil
}
