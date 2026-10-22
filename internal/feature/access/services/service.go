package services

import (
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB           *postgres.DB
	Config       *config.Config
	OAuth        accesstypes.OAuthClient
	Hasher       *auth.PasswordHasher
	LoginLimiter *auth.LoginLimiter
}

type parts struct {
	cfg                  *config.Config
	userRepository       *repositories.UserRepository
	sessionRepository    *repositories.SessionRepository
	groupRepository      *repositories.GroupRepository
	tokenRepository      *repositories.TokenRepository
	identityRepository   *repositories.IdentityRepository
	loginStateRepository *repositories.LoginStateRepository
	oauth                accesstypes.OAuthClient
	hasher               *auth.PasswordHasher
	loginLimiter         *auth.LoginLimiter
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                  d.Config,
		userRepository:       repositories.NewUserRepository(d.DB),
		sessionRepository:    repositories.NewSessionRepository(d.DB),
		groupRepository:      repositories.NewGroupRepository(d.DB),
		tokenRepository:      repositories.NewTokenRepository(d.DB),
		identityRepository:   repositories.NewIdentityRepository(d.DB),
		loginStateRepository: repositories.NewLoginStateRepository(d.DB),
		oauth:                d.OAuth,
		hasher:               d.Hasher,
		loginLimiter:         d.LoginLimiter,
	}
}

type AccessServices struct {
	Auth       *AuthService
	Bootstrap  *BootstrapService
	Groups     *GroupsService
	Identities *IdentitiesService
	OAuth      *OAuthService
	Tokens     *TokensService
	Users      *UsersService
}

func New(d Deps) *AccessServices {
	p := newParts(d)
	f := &AccessServices{
		Auth:       &AuthService{cfg: p.cfg, hasher: p.hasher, loginLimiter: p.loginLimiter, sessionRepository: p.sessionRepository, tokenRepository: p.tokenRepository, userRepository: p.userRepository},
		Bootstrap:  &BootstrapService{hasher: p.hasher, userRepository: p.userRepository},
		Groups:     &GroupsService{groupRepository: p.groupRepository, userRepository: p.userRepository},
		Identities: &IdentitiesService{identityRepository: p.identityRepository, userRepository: p.userRepository},
		OAuth:      &OAuthService{cfg: p.cfg, groupRepository: p.groupRepository, identityRepository: p.identityRepository, loginStateRepository: p.loginStateRepository, oauth: p.oauth, userRepository: p.userRepository},
		Tokens:     &TokensService{cfg: p.cfg, tokenRepository: p.tokenRepository, userRepository: p.userRepository},
		Users:      &UsersService{hasher: p.hasher, sessionRepository: p.sessionRepository, userRepository: p.userRepository},
	}
	f.Identities.oauthService = f.OAuth
	f.OAuth.auth = f.Auth
	return f
}
