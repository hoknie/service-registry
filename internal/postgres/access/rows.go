package access

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domain "svc-registry/internal/access"
	"svc-registry/internal/apperr"
)

func status(raw string) (domain.UserStatus, error) {
	s, ok := domain.ParseStatus(raw)
	if !ok {
		return "", &domain.InternalError{Detail: "unknown status " + raw}
	}
	return s, nil
}

func scanUser(row pgx.Row, extra ...any) (domain.User, error) {
	var u domain.User
	var st string
	dest := append([]any{&u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return domain.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func scanUserAfter(row pgx.Row, leading ...any) (domain.User, error) {
	var u domain.User
	var st string
	dest := append(leading, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if err := row.Scan(dest...); err != nil {
		return domain.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func parseScopes(raw []string) (domain.Scopes, error) {
	out := make(domain.Scopes, 0, len(raw))
	for _, r := range raw {
		sc, ok := domain.ParseScope(r)
		if !ok {
			return nil, &domain.InternalError{Detail: "unknown scope " + r}
		}
		out = append(out, sc)
	}
	ordered := make(domain.Scopes, 0, len(out))
	for _, sc := range domain.AllScopes {
		if out.Has(sc) {
			ordered = append(ordered, sc)
		}
	}
	return ordered, nil
}

func scanToken(row pgx.Row) (domain.Token, error) {
	var t domain.Token
	var scopes []string
	var st string
	err := row.Scan(&t.ID, &t.UserID, &t.UserEmail, &t.Name, &t.Prefix, &scopes,
		&t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &st)
	if err != nil {
		return domain.Token{}, err
	}
	if t.Scopes, err = parseScopes(scopes); err != nil {
		return domain.Token{}, err
	}
	var ok bool
	if t.Status, ok = domain.ParseTokenStatus(st); !ok {
		return domain.Token{}, &domain.InternalError{Detail: "unknown token status " + st}
	}
	return t, nil
}

func scanGroup(row pgx.Row) (domain.Group, error) {
	var g domain.Group
	err := row.Scan(&g.ID, &g.Name, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var domainErr domain.Conflict
	var refusal domain.Refusal
	if errors.As(err, &domainErr) || errors.As(err, &refusal) || errors.Is(err, domain.ErrNotFound) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.ConstraintName {
		case "users_email_lower_key":
			return domain.ConflictEmailTaken
		case "groups_name_lower_key":
			return domain.ConflictGroupNameTaken
		case "group_members_group_id_fkey", "group_members_user_id_fkey", "personal_access_tokens_user_id_fkey":
			return domain.ErrNotFound
		case "user_identities_subject_key":
			return domain.RefuseIdentityTaken
		}
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}
