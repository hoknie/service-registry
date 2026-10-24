package services

import (
	catalogports "svc-registry/internal/feature/catalog/ports"
	"svc-registry/internal/feature/catalog/repositories"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Secrets *secretbox.Box
}

type parts struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	nodeRepository       *repositories.NodeRepository
	bindingRepository    *repositories.BindingRepository
	projectKeyRepository *repositories.ProjectKeyRepository
	branchRepository     *repositories.BranchRepository
	activityRepository   *repositories.ActivityRepository
	managed              catalogports.ManagedNodesPort
	sources              []catalogports.ActivitySourcePort
	secretRepository     *repositories.SecretRepository
	box                  *secretbox.Box
	secretUsers          []catalogports.SecretUserPort
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                  d.Config,
		db:                   d.DB,
		nodeRepository:       repositories.NewNodeRepository(d.DB),
		bindingRepository:    repositories.NewBindingRepository(d.DB),
		projectKeyRepository: repositories.NewProjectKeyRepository(d.DB),
		branchRepository:     repositories.NewBranchRepository(d.DB, d.Config.Branches.StaleDays),
		activityRepository:   repositories.NewActivityRepository(d.DB),
		secretRepository:     repositories.NewSecretRepository(d.DB),
		box:                  d.Secrets,
	}
}

func (c *CatalogServices) Use(managed catalogports.ManagedNodesPort, sources ...catalogports.ActivitySourcePort) {
	c.Nodes.managed = managed
	c.Activity.sources = sources
}

func (c *CatalogServices) UseSecretUsers(users ...catalogports.SecretUserPort) {
	c.Secrets.secretUsers = users
}

func (c *CatalogServices) UseOwners(owners catalogports.OwnersFilterPort) {
	c.Table.owners = owners
}

func (c *CatalogServices) UseTokenRoots(roots catalogports.TokenRootsPort) {
	c.Nodes.tokenRoots = roots
}

func (c *CatalogServices) Wired() bool {
	return c.Nodes.managed != nil && len(c.Activity.sources) > 0 && len(c.Secrets.secretUsers) > 0 && c.Table.owners != nil &&
		c.Nodes.tokenRoots != nil
}

type CatalogServices struct {
	Access         *AccessService
	Activity       *ActivityService
	Bindings       *BindingsService
	BranchRecords  *BranchRecordsService
	Branches       *BranchesService
	Labels         *LabelsService
	Nodes          *NodesService
	ProjectKeys    *ProjectKeysService
	Secrets        *SecretsService
	SecretResolver *SecretResolverService
	System         *SystemService
	Table          *TableService
}

func New(d Deps) *CatalogServices {
	p := newParts(d)
	f := &CatalogServices{
		Access:   &AccessService{nodeRepository: p.nodeRepository},
		Activity: &ActivityService{activityRepository: p.activityRepository, sources: p.sources},
		Bindings: &BindingsService{bindingRepository: p.bindingRepository},
		Branches: &BranchesService{branchRepository: p.branchRepository, cfg: p.cfg},
		Labels:   &LabelsService{nodeRepository: p.nodeRepository},
		Nodes: &NodesService{branchRepository: p.branchRepository, managed: p.managed, nodeRepository: p.nodeRepository, db: p.db,
			bindingRepository: p.bindingRepository},
		ProjectKeys:    &ProjectKeysService{projectKeyRepository: p.projectKeyRepository},
		Secrets:        &SecretsService{box: p.box, secretUsers: p.secretUsers, secretRepository: p.secretRepository},
		SecretResolver: &SecretResolverService{box: p.box, secretRepository: p.secretRepository},
		System:         &SystemService{nodeRepository: p.nodeRepository},
		BranchRecords:  &BranchRecordsService{branchRepository: p.branchRepository, db: p.db, nodeRepository: p.nodeRepository},
		Table:          &TableService{nodeRepository: p.nodeRepository},
	}
	f.Access.system = f.System
	f.Activity.access = f.Access
	f.Bindings.access = f.Access
	f.Branches.access = f.Access
	f.Nodes.access = f.Access
	f.Nodes.projectKeysService = f.ProjectKeys
	f.ProjectKeys.access = f.Access
	f.Secrets.access = f.Access
	f.Table.activityService = f.Activity
	f.Table.nodesService = f.Nodes
	return f
}
