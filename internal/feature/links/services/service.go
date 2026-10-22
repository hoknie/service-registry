package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/links/repositories"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Checker linksports.CheckerPort
	Icons   linksports.IconFilesPort
}

type parts struct {
	cfg                *config.Config
	kindRepository     *repositories.KindRepository
	templateRepository *repositories.TemplateRepository
	targetRepository   *repositories.TargetRepository
	checker            linksports.CheckerPort
	icons              linksports.IconFilesPort
	catalog            *catalogservices.CatalogServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                d.Config,
		kindRepository:     repositories.NewKindRepository(d.DB),
		templateRepository: repositories.NewTemplateRepository(d.DB),
		targetRepository:   repositories.NewTargetRepository(d.DB),
		checker:            d.Checker,
		icons:              d.Icons,
		catalog:            d.Catalog,
	}
}

type LinksServices struct {
	Checks    *ChecksService
	Icons     *IconsService
	Kinds     *KindsService
	Projects  *ProjectsService
	Templates *TemplatesService
}

func New(d Deps) *LinksServices {
	p := newParts(d)
	f := &LinksServices{
		Checks:    &ChecksService{catalog: p.catalog, cfg: p.cfg, checker: p.checker, targetRepository: p.targetRepository, templateRepository: p.templateRepository},
		Icons:     &IconsService{catalog: p.catalog, icons: p.icons},
		Kinds:     &KindsService{kindRepository: p.kindRepository},
		Projects:  &ProjectsService{catalog: p.catalog, cfg: p.cfg, checker: p.checker, targetRepository: p.targetRepository, templateRepository: p.templateRepository},
		Templates: &TemplatesService{catalog: p.catalog, icons: p.icons, templateRepository: p.templateRepository},
	}
	f.Checks.projects = f.Projects
	f.Templates.projects = f.Projects
	return f
}
