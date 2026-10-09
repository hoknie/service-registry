package access

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	PasswordMinChars    = 8
	PasswordMaxChars    = 256
	PageDefaultLimit    = 50
	PageMaxLimit        = 200
	TokenPrefixMaxChars = 12
)

func trim(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

func ValidateEmail(raw string) (string, error) {
	email := strings.ToLower(trim(raw))
	if utf8.RuneCountInString(email) > 254 {
		return "", InvalidEmail
	}
	for _, r := range email {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", InvalidEmail
		}
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") {
		return "", InvalidEmail
	}
	return email, nil
}

func ValidateDisplayName(raw string) (string, error) {
	if name, ok := validName(raw); ok {
		return name, nil
	}
	return "", InvalidDisplayName
}

func ValidateGroupName(raw string) (string, error) {
	if name, ok := validName(raw); ok {
		return name, nil
	}
	return "", InvalidGroupName
}

func validName(raw string) (string, bool) {
	name := trim(raw)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 100 {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

func ValidatePassword(raw string) error {
	switch n := utf8.RuneCountInString(raw); {
	case n < PasswordMinChars:
		return InvalidPasswordTooShort
	case n > PasswordMaxChars:
		return InvalidPasswordTooLong
	}
	return nil
}

func ValidateStatus(raw string) (UserStatus, error) {
	if s, ok := ParseStatus(raw); ok {
		return s, nil
	}
	return "", InvalidStatus
}

func ValidatePage(q PageQuery) (PageRequest, error) {
	p := PageRequest{Limit: PageDefaultLimit}
	if q.Limit != nil {
		if *q.Limit < 1 || *q.Limit > PageMaxLimit {
			return PageRequest{}, InvalidPagination
		}
		p.Limit = uint32(*q.Limit)
	}
	if q.Offset != nil {
		if *q.Offset < 0 {
			return PageRequest{}, InvalidPagination
		}
		p.Offset = uint64(*q.Offset)
	}
	return p, nil
}

type ValidatedToken struct {
	Name          string
	Scopes        Scopes
	ExpiresInDays *uint32
}

func ValidateCreateToken(in CreateToken, superadmin bool, maxDays uint32) (ValidatedToken, error) {
	name, ok := validName(in.Name)
	if !ok {
		return ValidatedToken{}, InvalidTokenName
	}
	if len(in.Scopes) == 0 {
		return ValidatedToken{}, InvalidScopes
	}
	seen := map[Scope]bool{}
	for _, raw := range in.Scopes {
		sc, ok := ParseScope(raw)
		if !ok || seen[sc] || (sc == ScopeAdmin && !superadmin) {
			return ValidatedToken{}, InvalidScopes
		}
		seen[sc] = true
	}
	scopes := Scopes{}
	for _, sc := range AllScopes {
		if seen[sc] {
			scopes = append(scopes, sc)
		}
	}
	out := ValidatedToken{Name: name, Scopes: scopes}
	switch {
	case in.ExpiresInDays == nil && !superadmin:
		return ValidatedToken{}, InvalidTokenLifetime
	case in.ExpiresInDays != nil:
		if *in.ExpiresInDays < 1 || *in.ExpiresInDays > int64(maxDays) {
			return ValidatedToken{}, InvalidTokenLifetime
		}
		days := uint32(*in.ExpiresInDays)
		out.ExpiresInDays = &days
	}
	return out, nil
}

func ValidateTokenQuery(q TokenQuery) (TokenFilter, error) {
	var f TokenFilter
	if q.Prefix != nil {
		if utf8.RuneCountInString(*q.Prefix) > TokenPrefixMaxChars {
			return TokenFilter{}, InvalidTokenPrefix
		}
		f.Prefix = *q.Prefix
	}
	page, err := ValidatePage(q.Page)
	if err != nil {
		return TokenFilter{}, err
	}
	f.Page = page
	return f, nil
}
