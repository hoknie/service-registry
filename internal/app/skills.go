package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"svc-registry/internal/feature/knowledge/clients/source"
	"svc-registry/internal/feature/skills/clients/blobstore/local"
	"svc-registry/internal/feature/skills/clients/blobstore/s3"
	skillsports "svc-registry/internal/feature/skills/ports"
	skillsservices "svc-registry/internal/feature/skills/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/outbound"
	"svc-registry/pkg/pgtx"
)

func WithSkillsStore(s skillsports.BlobStorePort) Option {
	return func(o *overrides) { o.skillsStore = s }
}

func skillsStore(o overrides, cfg config.Config) (skillsports.BlobStorePort, error) {
	if o.skillsStore != nil {
		return o.skillsStore, nil
	}
	switch cfg.Skills.Storage {
	case config.SkillsStorageLocal:
		return local.New(cfg.Skills.Dir), nil
	case config.SkillsStorageS3:
		return s3.New(cfg.Skills.S3(), outbound.New(cfg.Outbound.Options()).Transport)
	}
	return nil, nil
}

func (a *App) buildSkills(o overrides, readers *source.FactoryClient) error {
	if !a.Config.Skills.Enabled {
		return nil
	}
	store, err := skillsStore(o, *a.Config)
	if err != nil {
		return err
	}
	a.Skills = skillsservices.New(skillsservices.Deps{DB: a.DB, Config: a.Config, Catalog: a.Catalog, Store: store, Readers: readers})
	a.skillsStore = store
	return nil
}

func (a *App) CheckSkillsStorage(ctx context.Context) {
	if a.skillsStore == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := a.skillsStore.Has(ctx, strings.Repeat("0", 64)); err != nil {
		slog.Warn("skills storage unavailable", "storage", a.skillsStore.Name(), "error", err)
	}
}

func OpenSkillsStorageCopy(db config.DbConfig, skills config.SkillsConfig, out config.OutboundConfig) (func(context.Context) (skillsservices.CopyResult, error), func(), error) {
	pool, err := postgres.ConnectLazy(db)
	if err != nil {
		return nil, nil, err
	}
	dir := skills.Dir
	if dir == "" {
		dir = filepath.Clean(skills.DirRaw)
	}
	to, err := s3.New(skills.S3(), outbound.New(out.Options()).Transport)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	cfg := &config.Config{Skills: skills}
	services := skillsservices.New(skillsservices.Deps{DB: pgtx.New(pool), Config: cfg})
	copyAll := func(ctx context.Context) (skillsservices.CopyResult, error) {
		return services.Ports.CopyBlobs(ctx, local.New(dir), to)
	}
	return copyAll, pool.Close, nil
}
