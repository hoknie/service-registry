package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	skillsports "svc-registry/internal/feature/skills/ports"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/feature/skills/usecases"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Store   skillsports.BlobStorePort
	Readers knowledgeports.ReaderFactoryPort
}

type SkillsServices struct {
	Folders  *FoldersService
	Skills   *SkillsService
	Content  *ContentService
	Images   *ImagesService
	Versions *VersionsService
	Spaces   *SpacesService
	Sources  *SourcesService
	Ports    *PortsService
}

type parts struct {
	db       *pgtx.DB
	cfg      *config.Config
	access   *accessRules
	folders  *repositories.FolderRepository
	skills   *repositories.SkillRepository
	versions *repositories.VersionRepository
	blobs    *repositories.BlobRepository
	sources  *repositories.SourceRepository
	store    skillsports.BlobStorePort
	save     *usecases.SaveVersionUseCase
}

func limitsOf(c config.SkillsConfig) skillstypes.Limits {
	return skillstypes.Limits{TotalBytes: c.MaxTotalBytes, FileBytes: c.MaxFileBytes, Files: int(c.MaxFiles)}
}

func New(d Deps) *SkillsServices {
	p := parts{db: d.DB, cfg: d.Config, access: &accessRules{catalog: d.Catalog}, folders: repositories.NewFolderRepository(d.DB),
		skills: repositories.NewSkillRepository(d.DB), versions: repositories.NewVersionRepository(d.DB),
		blobs: repositories.NewBlobRepository(d.DB), sources: repositories.NewSourceRepository(d.DB), store: d.Store}
	p.save = usecases.NewSaveVersion(usecases.SaveVersionDeps{DB: d.DB, Skills: p.skills, Versions: p.versions, Blobs: p.blobs,
		Store: d.Store, Keep: int(d.Config.Skills.KeepVersions)})
	cards := &cardBuilder{parts: p}
	versions := &VersionsService{parts: p, cards: cards}
	content := &ContentService{parts: p, cards: cards, limits: limitsOf(d.Config.Skills)}
	return &SkillsServices{
		Folders:  &FoldersService{parts: p},
		Skills:   &SkillsService{parts: p, cards: cards},
		Content:  content,
		Images:   &ImagesService{content: content},
		Versions: versions,
		Spaces:   &SpacesService{parts: p, spaces: repositories.NewSpaceRepository(d.DB)},
		Sources: &SourcesService{parts: p, catalog: d.Catalog, readers: d.Readers, limits: limitsOf(d.Config.Skills),
			sync: usecases.NewSyncSource(usecases.SyncSourceDeps{Skills: p.skills, Save: p.save, Limits: limitsOf(d.Config.Skills)})},
		Ports: &PortsService{parts: p},
	}
}
