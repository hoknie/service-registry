package config

import (
	"path/filepath"
	"slices"
	"strings"
)

type UploadsConfig struct {
	DirRaw string `env:"UPLOADS_DIR" envDefault:"data/uploads"`
	Dir    string `env:"-"`
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
