package services

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type IdentityView struct {
	accesstypes.Identity
	DisplayName string
}

func (s *IdentitiesService) identityViews(items []accesstypes.Identity) []IdentityView {
	out := make([]IdentityView, 0, len(items))
	for _, i := range items {
		name := i.Provider
		if cfg, ok := s.oauthService.providerConfig(i.Provider); ok {
			name = cfg.DisplayName
		}
		out = append(out, IdentityView{Identity: i, DisplayName: name})
	}
	return out
}

func (s *IdentitiesService) OwnIdentities(ctx context.Context, p accesstypes.Principal) ([]IdentityView, error) {
	items, err := s.identityRepository.ListForUser(ctx, p.UserID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *IdentitiesService) UserIdentities(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]IdentityView, error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionRead); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	items, err := s.identityRepository.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.identityViews(items), nil
}

func (s *IdentitiesService) UnlinkIdentity(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		hasPassword, err := s.userRepository.LockPassword(ctx, p.UserID)
		if err != nil {
			return err
		}
		if hasPassword == nil {
			return apperr.ErrNotFound
		}
		count, err := s.identityRepository.CountForUser(ctx, p.UserID)
		if err != nil {
			return err
		}
		if err := s.identityRepository.Delete(ctx, p.UserID, id); err != nil {
			return err
		}
		if accesstypes.LastLoginMethod(*hasPassword, count) {
			return accesstypes.ConflictLastLoginMethod
		}
		return nil
	})
	return apperr.Wrap(err)
}

type IdentitiesService struct {
	db                 *pgtx.DB
	identityRepository *repositories.IdentityRepository
	userRepository     *repositories.UserRepository
	oauthService       *OAuthService
}

func (s *OAuthService) policyFor(ctx context.Context, cfg config.OAuthProvider, ls *accesstypes.LoginState, claims accesstypes.Claims,
	email *string, done *LoginDone) (accesstypes.ProviderPolicy, bool, error) {
	policy := accesstypes.ProviderPolicy{Key: cfg.Key, AutoProvision: cfg.AutoProvision, AllowedDomains: cfg.AllowedDomains}
	invited, mismatch, err := s.invitedBy(ctx, ls.InviteHash, claims, email)
	if invited {
		policy.AutoProvision, policy.AllowedDomains = true, nil
	}
	if mismatch {
		done.Next = accesstypes.WithError(done.Next, "invitation.email_mismatch")
	}
	return policy, invited, err
}

func (s *OAuthService) acceptInvitation(ctx context.Context, invited bool, hash []byte, user accesstypes.User) {
	if !invited {
		return
	}
	if err := s.invitations.AcceptInvitation(ctx, [32]byte(hash), user.ID, user.Email); err != nil {
		slog.Warn("invitation was not accepted after sign-in", "user", user.ID, "error", err)
	}
}

func (s *OAuthService) invitedBy(ctx context.Context, hash []byte, claims accesstypes.Claims, email *string) (bool, bool, error) {
	if len(hash) != sha256.Size || s.invitations == nil {
		return false, false, nil
	}
	invited, ok, err := s.invitations.PendingInvitation(ctx, [32]byte(hash))
	if err != nil || !ok {
		return false, false, apperr.Wrap(err)
	}
	if email == nil || !claims.EmailVerified || !strings.EqualFold(*email, invited) {
		return false, true, nil
	}
	return true, false, nil
}
