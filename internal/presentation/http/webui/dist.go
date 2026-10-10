package webui

import (
	"os"
	"path/filepath"

	"svc-registry/internal/platform/config"
)

const ExportMarker = "404.html"

type Dist struct {
	root string
}

func NewDist(cfg config.WebConfig) Dist { return Dist{root: cfg.DistDir} }

func (d Dist) Root() string { return d.root }

func (d Dist) HasExport() bool {
	info, err := os.Stat(filepath.Join(d.root, ExportMarker))
	return err == nil && info.Mode().IsRegular()
}
