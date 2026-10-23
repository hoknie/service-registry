package types

import (
	"github.com/google/uuid"
)

type ProjectName struct{ Path, Name string }

type IndexFile struct {
	Branch string
	Commit string
	Path   string
	Kind   Kind
	SHA256 []byte
}

type StoredChunk struct {
	Ord    int
	Span   Span
	Vector []float32
}

type BlobContent struct {
	SHA256  []byte
	Content string
}

type IndexState struct {
	Engine      string
	Model       string
	Fingerprint string
}

type StoredSource struct {
	Source
	CredentialsSecretID *uuid.UUID
}

type NewSource struct {
	Source
	SecretID *uuid.UUID
	Keep     bool
}

type LegacySource struct {
	ProjectID uuid.UUID
	ParentID  *uuid.UUID
	Slug      string
	Enc       *string
	Ref       *string
}
