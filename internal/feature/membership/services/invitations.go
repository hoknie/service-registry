package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/membership/repositories"
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/feature/membership/usecases"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/apikey"
)

const invitePrefix = "svci_"

type InvitationsService struct {
	parts
	accept   *usecases.AcceptInvitationUseCase
	register *usecases.RegisterInvitedUseCase
}

type NewInvitation struct {
	Email  string
	Role   string
	Locale string
}

type Issued struct {
	Invitation membershiptypes.Invitation
	Link       string
	Mail       membershiptypes.MailState
}

type InvitationView struct {
	Invitation    membershiptypes.Invitation
	AccountExists bool
	PasswordLogin bool
	Providers     []accessservices.Provider
}

func (s *InvitationsService) CreateInvitation(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in NewInvitation) (Issued, error) {
	scoped, err := s.organization(ctx, p, id, catalogtypes.PermAccess)
	if err != nil {
		return Issued{}, err
	}
	email, okEmail := membershiptypes.NormalizeEmail(in.Email)
	role, okRole := membershiptypes.ParseRole(in.Role)
	if !okEmail || !okRole || !membershiptypes.ValidLocale(in.Locale) {
		return Issued{}, apperr.Wrap(membershiptypes.InvalidInvitation)
	}
	if user, err := s.members.UserByEmail(ctx, email); err != nil {
		return Issued{}, apperr.Wrap(err)
	} else if user != nil {
		if member, err := s.members.IsMember(ctx, id, user.ID); err != nil || member {
			return Issued{}, apperr.Wrap(orConflict(err, membershiptypes.AlreadyMember))
		}
	}
	waiting, err := s.invitations.Waiting(ctx, id)
	if err != nil {
		return Issued{}, apperr.Wrap(err)
	}
	if waiting >= membershiptypes.MaxPendingInvitations {
		return Issued{}, apperr.Wrap(membershiptypes.TooManyInvitations)
	}
	token, err := apikey.Generate(invitePrefix)
	if err != nil {
		return Issued{}, apperr.Wrap(err)
	}
	invID, err := uuid.NewV7()
	if err != nil {
		return Issued{}, apperr.Wrap(err)
	}
	expires := time.Now().Add(s.cfg.Mail.InviteTTL())
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.invitations.DropExpired(ctx, id, email); err != nil {
			return err
		}
		return s.invitations.Insert(ctx, membershiptypes.NewInvitation{ID: invID, OrganizationID: id, Email: email, Role: role,
			Locale: in.Locale, TokenHash: token.Hash, InvitedBy: p.UserID, ExpiresAt: expires})
	})
	if err != nil {
		return Issued{}, apperr.Wrap(repositories.Err(err))
	}
	return s.issue(ctx, p, id, invID, token.Secret, scoped.Node.Name)
}

func orConflict(err error, conflict error) error {
	if err != nil {
		return err
	}
	return conflict
}

func (s *InvitationsService) issue(ctx context.Context, p accesstypes.Principal, org, id uuid.UUID, secret, orgName string) (Issued, error) {
	inv, err := s.invitations.Get(ctx, org, id)
	if err != nil || inv == nil {
		return Issued{}, apperr.Wrap(orConflict(err, apperr.ErrNotFound))
	}
	path := "/" + inv.Locale + "/invite?token=" + secret
	out := Issued{Invitation: *inv, Link: path, Mail: membershiptypes.MailDisabled}
	base := strings.TrimSuffix(s.cfg.Web.PublicURL, "/")
	if base == "" || s.mailer == nil || !s.mailer.Enabled() {
		return out, nil
	}
	out.Link = base + path
	inviter := ""
	if who, err := s.members.User(ctx, p.UserID); err == nil && who != nil {
		inviter = who.Name
	}
	expires, _ := time.Parse(time.RFC3339, inv.ExpiresAt)
	letter := membershiptypes.InvitationLetter(inv.Locale, inv.Email, orgName, inviter, inv.Role, out.Link, expires)
	if err := s.mailer.Send(ctx, letter); err != nil {
		slog.Warn("invitation letter was not sent", "organization", org, "invitation", id, "error", err)
		out.Mail = membershiptypes.MailFailed
		return out, nil
	}
	out.Mail = membershiptypes.MailSent
	return out, nil
}

func (s *InvitationsService) ListInvitations(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]membershiptypes.Invitation, error) {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return nil, err
	}
	out, err := s.invitations.List(ctx, id)
	return out, apperr.Wrap(err)
}

func (s *InvitationsService) ResendInvitation(ctx context.Context, p accesstypes.Principal, id, iid uuid.UUID, locale *string) (Issued, error) {
	scoped, err := s.organization(ctx, p, id, catalogtypes.PermAccess)
	if err != nil {
		return Issued{}, err
	}
	inv, err := s.invitations.Get(ctx, id, iid)
	if err != nil || inv == nil {
		return Issued{}, apperr.Wrap(orConflict(err, apperr.ErrNotFound))
	}
	if st := inv.State(); st == membershiptypes.Accepted || st == membershiptypes.Revoked {
		return Issued{}, apperr.Wrap(membershiptypes.InvitationClosed)
	}
	lang := inv.Locale
	if locale != nil {
		if !membershiptypes.ValidLocale(*locale) {
			return Issued{}, apperr.Wrap(membershiptypes.InvalidInvitation)
		}
		lang = *locale
	}
	token, err := apikey.Generate(invitePrefix)
	if err != nil {
		return Issued{}, apperr.Wrap(err)
	}
	if err := s.invitations.Rotate(ctx, iid, token.Hash, lang, time.Now().Add(s.cfg.Mail.InviteTTL())); err != nil {
		return Issued{}, apperr.Wrap(err)
	}
	return s.issue(ctx, p, id, iid, token.Secret, scoped.Node.Name)
}

func (s *InvitationsService) RevokeInvitation(ctx context.Context, p accesstypes.Principal, id, iid uuid.UUID) error {
	if _, err := s.organization(ctx, p, id, catalogtypes.PermAccess); err != nil {
		return err
	}
	return apperr.Wrap(s.invitations.Revoke(ctx, id, iid))
}

func hashOf(token string) ([32]byte, error) {
	hash, ok := apikey.Parse(invitePrefix, strings.TrimSpace(token))
	if !ok {
		return hash, apperr.New(apperr.NotFound)
	}
	return hash, nil
}

func (s *InvitationsService) allow(ip string) error {
	if s.limiter == nil {
		return nil
	}
	if retry, ok := s.limiter.Hit(ip, time.Now()); !ok {
		return &apperr.Error{Kind: apperr.PublicRateLimited, RetryAfterSecs: retry}
	}
	return nil
}

func (s *InvitationsService) ViewInvitation(ctx context.Context, ip, token string) (InvitationView, error) {
	if err := s.allow(ip); err != nil {
		return InvitationView{}, err
	}
	hash, err := hashOf(token)
	if err != nil {
		return InvitationView{}, err
	}
	inv, err := s.invitations.ByHash(ctx, hash)
	if err != nil || inv == nil {
		return InvitationView{}, apperr.Wrap(orConflict(err, apperr.ErrNotFound))
	}
	user, err := s.members.UserByEmail(ctx, inv.Email)
	if err != nil {
		return InvitationView{}, apperr.Wrap(err)
	}
	providers, mode := s.access.OAuth.Providers()
	return InvitationView{Invitation: *inv, AccountExists: user != nil, PasswordLogin: mode == string(accesstypes.PasswordAll),
		Providers: providers}, nil
}

func (s *InvitationsService) AcceptInvitation(ctx context.Context, p accesstypes.Principal, token string) (membershiptypes.Invitation, error) {
	hash, err := hashOf(token)
	if err != nil {
		return membershiptypes.Invitation{}, err
	}
	who, err := s.members.User(ctx, p.UserID)
	if err != nil || who == nil {
		return membershiptypes.Invitation{}, apperr.Wrap(orConflict(err, apperr.New(apperr.Unauthenticated)))
	}
	out, err := s.accept.Run(ctx, hash, usecases.Acceptor{ID: who.ID, Email: who.Email})
	return out, apperr.Wrap(err)
}

func (s *InvitationsService) RegisterInvited(ctx context.Context, ip, token, name, password string, previous *string) (accessservices.SignedIn, error) {
	if err := s.allow(ip); err != nil {
		return accessservices.SignedIn{}, err
	}
	hash, err := hashOf(token)
	if err != nil {
		return accessservices.SignedIn{}, err
	}
	if _, mode := s.access.OAuth.Providers(); mode != string(accesstypes.PasswordAll) {
		return accessservices.SignedIn{}, apperr.Wrap(membershiptypes.PasswordLoginOff)
	}
	out, err := s.register.Run(ctx, usecases.Registration{Hash: hash, Name: strings.TrimSpace(name), Password: password, Previous: previous})
	return out, apperr.Wrap(err)
}

func (s *InvitationsService) PruneInvitations(ctx context.Context) (int64, error) {
	n, err := s.invitations.Prune(ctx)
	return n, apperr.Wrap(err)
}
