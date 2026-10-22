package repositories

import (
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

func status(raw string) (accesstypes.UserStatus, error) {
	s, ok := accesstypes.ParseStatus(raw)
	if !ok {
		return "", apperr.InternalDetail("unknown status " + raw)
	}
	return s, nil
}

func scanUser(row pgx.Row, extra ...any) (accesstypes.User, error) {
	var u accesstypes.User
	var st string
	dest := append([]any{&u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return accesstypes.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func scanUserAfter(row pgx.Row, leading ...any) (accesstypes.User, error) {
	var u accesstypes.User
	var st string
	dest := append(leading, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if err := row.Scan(dest...); err != nil {
		return accesstypes.User{}, err
	}
	var err error
	u.Status, err = status(st)
	return u, err
}

func parseScopes(raw []string) (accesstypes.Scopes, error) {
	out := make(accesstypes.Scopes, 0, len(raw))
	for _, r := range raw {
		sc, ok := accesstypes.ParseScope(r)
		if !ok {
			return nil, apperr.InternalDetail("unknown scope " + r)
		}
		out = append(out, sc)
	}
	ordered := make(accesstypes.Scopes, 0, len(out))
	for _, sc := range accesstypes.AllScopes {
		if out.Has(sc) {
			ordered = append(ordered, sc)
		}
	}
	return ordered, nil
}

func scanToken(row pgx.Row) (accesstypes.Token, error) {
	var t accesstypes.Token
	var scopes []string
	var st string
	err := row.Scan(&t.ID, &t.UserID, &t.UserEmail, &t.Name, &t.Prefix, &scopes,
		&t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &st)
	if err != nil {
		return accesstypes.Token{}, err
	}
	if t.Scopes, err = parseScopes(scopes); err != nil {
		return accesstypes.Token{}, err
	}
	var ok bool
	if t.Status, ok = accesstypes.ParseTokenStatus(st); !ok {
		return accesstypes.Token{}, apperr.InternalDetail("unknown token status " + st)
	}
	return t, nil
}

func scanGroup(row pgx.Row) (accesstypes.Group, error) {
	var g accesstypes.Group
	err := row.Scan(&g.ID, &g.Name, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}
