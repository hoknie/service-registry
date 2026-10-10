package catalog

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type SecretStorage string

const (
	SecretStored    SecretStorage = "stored"
	SecretReference SecretStorage = "reference"

	SecretTable       = "secrets"
	SecretColumnValue = "value_enc"
)

type NodeRef struct {
	ID   uuid.UUID
	Name string
}

type Secret struct {
	ID          uuid.UUID
	NodeID      *uuid.UUID
	Name        string
	Description string
	Storage     SecretStorage
	Fingerprint *string
	Ref         *string
	UsedBy      int64
	From        *NodeRef
	CreatedAt   string
	UpdatedAt   string
}

type SecretRef struct {
	ID   uuid.UUID
	Name string
	From *NodeRef
}

type SecretInput struct {
	Name        *string
	Description *string
	Value       *string
	Ref         *string
}

type SecretValue struct {
	Value *string
	Ref   *string
}

type ValidSecret struct {
	Name        *string
	Description *string
	Value       *SecretValue
}

type StoredSecret struct {
	Enc         *string
	Ref         *string
	Fingerprint *string
}

type NewSecret struct {
	ID          uuid.UUID
	NodeID      *uuid.UUID
	Name        string
	Description string
	Stored      StoredSecret
}

type SecretChanges struct {
	Name        *string
	Description *string
	Stored      *StoredSecret
}

type CredentialsInput struct {
	SecretID *string
	Inline   bool
}

type CredentialsKind string

const (
	CredentialsSecret CredentialsKind = "secret"
	CredentialsLegacy CredentialsKind = "legacy"
	CredentialsNone   CredentialsKind = "none"
)

type Credentials struct {
	Kind        CredentialsKind
	SecretID    *uuid.UUID
	Secret      *SecretRef
	Storage     SecretStorage
	Fingerprint *string
	Ref         *string
}

var envRef = regexp.MustCompile(`^env:[A-Za-z_][A-Za-z0-9_]{0,127}$`)

func ValidSecretRef(ref string) bool {
	if envRef.MatchString(ref) {
		return true
	}
	p, ok := strings.CutPrefix(ref, "file:")
	return ok && strings.HasPrefix(p, "/") && len(p) <= 4096 && !strings.ContainsRune(p, 0) &&
		strings.IndexFunc(p, unicode.IsControl) < 0
}

func ValidSecretToken(t string) bool {
	return t != "" && len(t) <= 4096 && strings.IndexFunc(t, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}

func ValidateSecret(in SecretInput, create bool) (ValidSecret, error) {
	var out ValidSecret
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return ValidSecret{}, InvalidSecret
		}
		out.Name = &name
	} else if create {
		return ValidSecret{}, InvalidSecret
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(d) > 500 {
			return ValidSecret{}, InvalidSecret
		}
		out.Description = &d
	}
	switch {
	case in.Value != nil && in.Ref != nil:
		return ValidSecret{}, InvalidSecret
	case in.Value != nil:
		if !ValidSecretToken(*in.Value) {
			return ValidSecret{}, InvalidSecret
		}
		v := *in.Value
		out.Value = &SecretValue{Value: &v}
	case in.Ref != nil:
		r := strings.TrimSpace(*in.Ref)
		if !ValidSecretRef(r) {
			return ValidSecret{}, InvalidSecret
		}
		out.Value = &SecretValue{Ref: &r}
	case create:
		return ValidSecret{}, InvalidSecret
	}
	return out, nil
}

func ParseCredentials(in *CredentialsInput, invalid error) (uuid.UUID, error) {
	if in == nil {
		return uuid.Nil, invalid
	}
	if in.Inline {
		return uuid.Nil, InvalidCredentialsInline
	}
	if in.SecretID == nil {
		return uuid.Nil, invalid
	}
	id, err := uuid.Parse(*in.SecretID)
	if err != nil {
		return uuid.Nil, invalid
	}
	return id, nil
}

func LegacyCredentials(enc, ref, fingerprint *string) Credentials {
	switch {
	case ref != nil:
		return Credentials{Kind: CredentialsLegacy, Storage: SecretReference, Ref: ref}
	case enc != nil:
		return Credentials{Kind: CredentialsLegacy, Storage: SecretStored, Fingerprint: fingerprint}
	}
	return Credentials{Kind: CredentialsNone}
}

type StoredSecretRow struct {
	ID  uuid.UUID
	Enc string
}
