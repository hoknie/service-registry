package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
)

func status(raw string) (access.UserStatus, error) {
	s, ok := access.ParseStatus(raw)
	if !ok {
		return "", &access.InternalError{Detail: "unknown status " + raw}
	}
	return s, nil
}

func scanUser(row pgx.Row, extra ...any) (access.User, error) {
	var u access.User
	var st string
	dest := append([]any{&u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return access.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func scanUserAfter(row pgx.Row, leading ...any) (access.User, error) {
	var u access.User
	var st string
	dest := append(leading, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if err := row.Scan(dest...); err != nil {
		return access.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func parseScopes(raw []string) (access.Scopes, error) {
	out := make(access.Scopes, 0, len(raw))
	for _, r := range raw {
		sc, ok := access.ParseScope(r)
		if !ok {
			return nil, &access.InternalError{Detail: "unknown scope " + r}
		}
		out = append(out, sc)
	}
	ordered := make(access.Scopes, 0, len(out))
	for _, sc := range access.AllScopes {
		if out.Has(sc) {
			ordered = append(ordered, sc)
		}
	}
	return ordered, nil
}

func scanToken(row pgx.Row) (access.Token, error) {
	var t access.Token
	var scopes []string
	var st string
	err := row.Scan(&t.ID, &t.UserID, &t.UserEmail, &t.Name, &t.Prefix, &scopes,
		&t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &st)
	if err != nil {
		return access.Token{}, err
	}
	if t.Scopes, err = parseScopes(scopes); err != nil {
		return access.Token{}, err
	}
	var ok bool
	if t.Status, ok = access.ParseTokenStatus(st); !ok {
		return access.Token{}, &access.InternalError{Detail: "unknown token status " + st}
	}
	return t, nil
}

func scanGroup(row pgx.Row) (access.Group, error) {
	var g access.Group
	err := row.Scan(&g.ID, &g.Name, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var domainErr access.Conflict
	var refusal access.Refusal
	if errors.As(err, &domainErr) || errors.As(err, &refusal) || errors.Is(err, access.ErrNotFound) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.ConstraintName {
		case "users_email_lower_key":
			return access.ConflictEmailTaken
		case "groups_name_lower_key":
			return access.ConflictGroupNameTaken
		case "group_members_group_id_fkey", "group_members_user_id_fkey", "personal_access_tokens_user_id_fkey":
			return access.ErrNotFound
		case "user_identities_subject_key":
			return access.RefuseIdentityTaken
		}
	}
	if apperr.IsUnavailable(err) {
		return access.ErrUnavailable
	}
	return &access.InternalError{Detail: err.Error()}
}
