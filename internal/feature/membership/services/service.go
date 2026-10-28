package services

import (
	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	membershipports "svc-registry/internal/feature/membership/ports"
	"svc-registry/internal/feature/membership/repositories"
	"svc-registry/internal/feature/membership/usecases"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Access  *accessservices.AccessServices
	Mailer  membershipports.MailerPort
	Limiter *auth.PublicDocsLimiter
}

type MembershipServices struct {
	Members     *MembersService
	Invitations *InvitationsService
	Ports       *PortsService
}

type parts struct {
	db          *pgtx.DB
	cfg         *config.Config
	catalog     *catalogservices.CatalogServices
	access      *accessservices.AccessServices
	mailer      membershipports.MailerPort
	limiter     *auth.PublicDocsLimiter
	members     *repositories.MemberRepository
	invitations *repositories.InvitationRepository
	keepAdmin   *usecases.KeepAdminUseCase
}

func New(d Deps) *MembershipServices {
	p := parts{db: d.DB, cfg: d.Config, catalog: d.Catalog, access: d.Access, mailer: d.Mailer, limiter: d.Limiter,
		members: repositories.NewMemberRepository(d.DB), invitations: repositories.NewInvitationRepository(d.DB)}
	p.keepAdmin = usecases.NewKeepAdmin(usecases.KeepAdminDeps{DB: d.DB, Members: p.members})
	accept := usecases.NewAcceptInvitation(usecases.AcceptInvitationDeps{DB: d.DB, Members: p.members, Invitations: p.invitations,
		Roles: d.Catalog.OrgRoles})
	return &MembershipServices{
		Members: &MembersService{parts: p, remove: usecases.NewRemoveMember(p.keepAdmin, p.members)},
		Invitations: &InvitationsService{parts: p, accept: accept, register: usecases.NewRegisterInvited(usecases.RegisterInvitedDeps{
			DB: d.DB, Members: p.members, Invitations: p.invitations, Access: d.Access, Accept: accept})},
		Ports: &PortsService{parts: p, accept: accept},
	}
}
