package services

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
	"svc-registry/pkg/password"
)

type SignedIn struct {
	User  accesstypes.User
	Token string
}

func (s *AuthService) Login(ctx context.Context, ip string, creds accesstypes.Credentials, previousToken *string) (SignedIn, error) {
	key := strings.ToLower(strings.TrimFunc(creds.Email, unicode.IsSpace))
	if secs, ok := s.loginLimiter.Check(ip, key, time.Now()); !ok {
		return SignedIn{}, apperr.RateLimitedFor(secs)
	}
	found, err := s.userRepository.FindCredentialsByEmail(ctx, key)
	if err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	mode := accesstypes.PasswordLogin(s.cfg.OAuth.PasswordLogin)
	if found == nil || found.PasswordHash == "" {
		s.hasher.VerifyDummy(creds.Password)
		s.loginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if !s.hasher.Verify(creds.Password, found.PasswordHash) || !mode.Allows(found.User.IsSuperadmin) || found.User.IsService {
		s.loginLimiter.RecordFailure(ip, key, time.Now())
		return SignedIn{}, apperr.New(apperr.InvalidCredentials)
	}
	if found.User.Status != accesstypes.StatusActive {
		return SignedIn{}, apperr.New(apperr.AccountDisabled)
	}
	s.loginLimiter.Reset(ip, key)
	return s.openSession(ctx, found.User, accesstypes.MethodPassword, previousToken)
}

func (s *AuthService) SignIn(ctx context.Context, user accesstypes.User, method string, previousToken *string) (SignedIn, error) {
	return s.openSession(ctx, user, method, previousToken)
}

func (s *AuthService) openSession(ctx context.Context, user accesstypes.User, method string, previousToken *string) (SignedIn, error) {
	if previousToken != nil {
		if err := s.sessionRepository.DeleteByToken(ctx, auth.TokenHash(*previousToken)); err != nil {
			return SignedIn{}, apperr.Wrap(err)
		}
	}
	cfg := s.cfg.Session
	if _, err := s.sessionRepository.DeleteExpiredForUser(ctx, user.ID, cfg.IdleTimeoutSecs); err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		return SignedIn{}, apperr.Internalf("no randomness: %v", err)
	}
	err = s.sessionRepository.Create(ctx, accesstypes.NewSession{
		ID:                  uuid.Must(uuid.NewV7()),
		UserID:              user.ID,
		TokenHash:           token.Hash,
		AbsoluteTimeoutSecs: cfg.AbsoluteTimeoutSecs,
		Method:              method,
	})
	if err != nil {
		return SignedIn{}, apperr.Wrap(err)
	}
	return SignedIn{User: user, Token: token.Token}, nil
}

func (s *AuthService) Logout(ctx context.Context, token *string) error {
	if token == nil {
		return nil
	}
	return apperr.Wrap(s.sessionRepository.DeleteByToken(ctx, auth.TokenHash(*token)))
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (accesstypes.Principal, accesstypes.User, error) {
	found, err := s.sessionRepository.FindValid(ctx, auth.TokenHash(token), s.cfg.Session.IdleTimeoutSecs)
	if err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.Unauthenticated)
	}
	p := accesstypes.Principal{UserID: found.User.ID, SessionID: found.SessionID, IsSuperadmin: found.User.IsSuperadmin, Method: found.Method}
	if p.Rights, err = s.rightsOf(ctx, found.User); err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, err
	}
	return p, found.User, nil
}

func (s *AuthService) rightsOf(ctx context.Context, user accesstypes.User) (accesstypes.Rights, error) {
	if user.IsSuperadmin {
		return nil, nil
	}
	rights, err := s.permissionRepository.ForUser(ctx, user.ID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return rights.Canonical(), nil
}

func (s *AuthService) authenticateApp(ctx context.Context, hash [32]byte) (accesstypes.Principal, accesstypes.User, error) {
	if s.cfg == nil || !s.cfg.MCPOAuth.Enabled {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
	}
	found, err := s.grantRepository.FindValidAccess(ctx, hash)
	if err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
	}
	p := accesstypes.Principal{UserID: found.User.ID, TokenID: found.GrantID, Scopes: accesstypes.Scopes{accesstypes.ScopeMCP},
		Nodes: found.Nodes, CatalogAdmin: found.CatalogAdmin && found.User.IsSuperadmin, App: true, Method: "oauth_app",
		MCPServers: found.MCPServers}
	return p, found.User, nil
}

func (s *AuthService) AuthenticateToken(ctx context.Context, secret string) (accesstypes.Principal, accesstypes.User, error) {
	if hash, ok := auth.ParseAppAccessToken(secret); ok {
		return s.authenticateApp(ctx, hash)
	}
	hash, ok := auth.ParsePersonalToken(secret)
	if !ok {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
	}
	found, err := s.tokenRepository.FindValid(ctx, hash)
	if err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.Wrap(err)
	}
	if found == nil {
		return accesstypes.Principal{}, accesstypes.User{}, apperr.New(apperr.InvalidToken)
	}
	superadmin := found.User.IsSuperadmin && found.Scopes.AnyGlobal() && found.Nodes == nil
	p := accesstypes.Principal{
		UserID:       found.User.ID,
		TokenID:      found.TokenID,
		Scopes:       found.Scopes,
		Nodes:        found.Nodes,
		MCPServers:   found.MCPServers,
		IsSuperadmin: superadmin,
		CatalogAdmin: found.CatalogAdmin && found.User.IsSuperadmin && !superadmin,
		Method:       "token",
	}
	rights, err := s.rightsOf(ctx, found.User)
	if err != nil {
		return accesstypes.Principal{}, accesstypes.User{}, err
	}
	p.Rights = rights.ForScopes(found.Scopes)
	return p, found.User, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, p accesstypes.Principal, change accesstypes.PasswordChange) error {
	current, err := s.userRepository.FindCredentials(ctx, p.UserID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if current == nil {
		return apperr.New(apperr.Unauthenticated)
	}
	switch {
	case change.CurrentPassword == nil && current.PasswordHash != "":
		return &apperr.Error{Kind: apperr.Validation, Code: "validation.invalid_body", Message: "current_password is required"}
	case change.CurrentPassword != nil && (current.PasswordHash == "" || !s.hasher.Verify(*change.CurrentPassword, current.PasswordHash)):
		return apperr.New(apperr.WrongPassword)
	}
	if err := accesstypes.ValidatePassword(change.NewPassword); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := s.hasher.Hash(change.NewPassword)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := s.userRepository.SetPassword(ctx, p.UserID, hash); err != nil {
		return apperr.Wrap(err)
	}
	_, err = s.sessionRepository.DeleteForUser(ctx, p.UserID, new(p.SessionID))
	return apperr.Wrap(err)
}

type AuthService struct {
	permissionRepository *repositories.GroupPermissionRepository
	cfg                  *config.Config
	hasher               *password.Hasher
	loginLimiter         *auth.LoginLimiter
	sessionRepository    *repositories.SessionRepository
	tokenRepository      *repositories.TokenRepository
	grantRepository      *repositories.OAuthGrantRepository
	userRepository       *repositories.UserRepository
	metrics              *metrics.Metrics
	instance             string
}

func (s *AuthService) MetricsView(p accesstypes.Principal) (metrics.View, error) {
	if err := accesstypes.RequireMetrics(p); err != nil {
		return metrics.View{}, err
	}
	v, err := s.metrics.View(s.instance, int(s.cfg.Metrics.RefreshSecs))
	return v, apperr.Wrap(err)
}

func (s *AuthService) MetricsFamily(p accesstypes.Principal, name string) (*metrics.Family, error) {
	if err := accesstypes.RequireMetrics(p); err != nil {
		return nil, err
	}
	f, ok, err := s.metrics.Family(name)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return f, nil
}
