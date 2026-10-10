package service

import (
	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/access/internal/repository"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB           *postgres.DB
	Config       *config.Config
	OAuth        access.OAuthClient
	Hasher       *auth.PasswordHasher
	LoginLimiter *auth.LoginLimiter
}

type Service struct {
	cfg          *config.Config
	users        *repository.Users
	sessions     *repository.Sessions
	groups       *repository.Groups
	tokens       *repository.Tokens
	identities   *repository.Identities
	loginStates  *repository.LoginStates
	oauth        access.OAuthClient
	hasher       *auth.PasswordHasher
	loginLimiter *auth.LoginLimiter
}

func New(d Deps) *Service {
	return &Service{
		cfg:          d.Config,
		users:        repository.NewUsers(d.DB),
		sessions:     repository.NewSessions(d.DB),
		groups:       repository.NewGroups(d.DB),
		tokens:       repository.NewTokens(d.DB),
		identities:   repository.NewIdentities(d.DB),
		loginStates:  repository.NewLoginStates(d.DB),
		oauth:        d.OAuth,
		hasher:       d.Hasher,
		loginLimiter: d.LoginLimiter,
	}
}
