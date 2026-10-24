package services

import (
	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/access/usecases"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/password"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB             *pgtx.DB
	Config         *config.Config
	OAuth          accesstypes.OAuthClient
	ClientMetadata accessports.ClientMetadataPort
	Hasher         *password.Hasher
	LoginLimiter   *auth.LoginLimiter
}

type parts struct {
	cfg                     *config.Config
	db                      *pgtx.DB
	registrations           *auth.RegistrationLimiter
	fetches                 *auth.MetadataFetchLimiter
	metadata                accessports.ClientMetadataPort
	clientRepository        *repositories.OAuthClientRepository
	authorizationRepository *repositories.OAuthAuthorizationRepository
	grantRepository         *repositories.OAuthGrantRepository
	userRepository          *repositories.UserRepository
	sessionRepository       *repositories.SessionRepository
	groupRepository         *repositories.GroupRepository
	groupContactRepository  *repositories.GroupContactRepository
	permissionRepository    *repositories.GroupPermissionRepository
	tokenRepository         *repositories.TokenRepository
	identityRepository      *repositories.IdentityRepository
	loginStateRepository    *repositories.LoginStateRepository
	oauth                   accesstypes.OAuthClient
	hasher                  *password.Hasher
	loginLimiter            *auth.LoginLimiter
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                     d.Config,
		db:                      d.DB,
		registrations:           auth.NewRegistrationLimiter(),
		fetches:                 auth.NewMetadataFetchLimiter(),
		metadata:                d.ClientMetadata,
		clientRepository:        repositories.NewOAuthClientRepository(d.DB),
		authorizationRepository: repositories.NewOAuthAuthorizationRepository(d.DB),
		grantRepository:         repositories.NewOAuthGrantRepository(d.DB),
		userRepository:          repositories.NewUserRepository(d.DB),
		sessionRepository:       repositories.NewSessionRepository(d.DB),
		groupRepository:         repositories.NewGroupRepository(d.DB),
		groupContactRepository:  repositories.NewGroupContactRepository(d.DB),
		permissionRepository:    repositories.NewGroupPermissionRepository(d.DB),
		tokenRepository:         repositories.NewTokenRepository(d.DB),
		identityRepository:      repositories.NewIdentityRepository(d.DB),
		loginStateRepository:    repositories.NewLoginStateRepository(d.DB),
		oauth:                   d.OAuth,
		hasher:                  d.Hasher,
		loginLimiter:            d.LoginLimiter,
	}
}

type AccessServices struct {
	Apps                *AppsService
	AuthorizationServer *AuthorizationServerService
	Auth                *AuthService
	Bootstrap           *BootstrapService
	GroupContacts       *GroupContactsService
	Groups              *GroupsService
	Identities          *IdentitiesService
	OAuth               *OAuthService
	Tokens              *TokensService
	Users               *UsersService
}

func New(d Deps) *AccessServices {
	p := newParts(d)
	f := &AccessServices{
		Apps: &AppsService{grantRepository: p.grantRepository, userRepository: p.userRepository},
		AuthorizationServer: &AuthorizationServerService{cfg: p.cfg, db: p.db, registrations: p.registrations,
			fetches: p.fetches, metadata: p.metadata,
			clientRepository: p.clientRepository, authorizationRepository: p.authorizationRepository, grantRepository: p.grantRepository},
		Auth: &AuthService{cfg: p.cfg, hasher: p.hasher, loginLimiter: p.loginLimiter, sessionRepository: p.sessionRepository,
			tokenRepository: p.tokenRepository, grantRepository: p.grantRepository, userRepository: p.userRepository,
			permissionRepository: p.permissionRepository},
		Bootstrap: &BootstrapService{hasher: p.hasher, userRepository: p.userRepository},
		GroupContacts: &GroupContactsService{db: p.db, groupRepository: p.groupRepository, groupContactRepository: p.groupContactRepository,
			permissionRepository: p.permissionRepository},
		Groups:     &GroupsService{db: p.db, groupRepository: p.groupRepository, userRepository: p.userRepository, permissionRepository: p.permissionRepository},
		Identities: &IdentitiesService{db: p.db, identityRepository: p.identityRepository, userRepository: p.userRepository},
		OAuth: &OAuthService{cfg: p.cfg, db: p.db, syncMemberships: usecases.NewSyncMemberships(p.db, p.groupRepository),
			identityRepository: p.identityRepository, loginStateRepository: p.loginStateRepository, oauth: p.oauth, userRepository: p.userRepository},
		Tokens: &TokensService{cfg: p.cfg, tokenRepository: p.tokenRepository, userRepository: p.userRepository, permissionRepository: p.permissionRepository},
		Users: &UsersService{db: p.db, hasher: p.hasher, sessionRepository: p.sessionRepository, userRepository: p.userRepository,
			permissionRepository: p.permissionRepository, tokenRepository: p.tokenRepository, grantRepository: p.grantRepository,
			updateUser: usecases.NewUpdateUser(usecases.UpdateUserDeps{DB: p.db, Users: p.userRepository, Sessions: p.sessionRepository,
				Tokens: p.tokenRepository, Grants: p.grantRepository})},
	}
	f.Identities.oauthService = f.OAuth
	f.OAuth.auth = f.Auth
	return f
}

func (f *AccessServices) Use(roots accessports.NodeRootsPort) {
	f.Tokens.roots = roots
	f.Apps.roots = roots
	f.AuthorizationServer.roots = roots
	f.Groups.roots = roots
}

func (f *AccessServices) UseStewards(stewards accessports.GroupStewardsPort) {
	f.Groups.stewards = stewards
	f.GroupContacts.stewards = stewards
}

func (f *AccessServices) UseOrganizations(organizations accessports.OrganizationsPort) {
	f.Groups.organizations = organizations
}
