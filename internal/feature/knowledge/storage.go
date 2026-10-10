package knowledge

import (
	"context"

	"github.com/google/uuid"
)

type ProjectName struct{ Path, Name string }

type Embedder interface {
	Model() string
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

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
	CredentialsEnc      *string
	CredentialsRef      *string
}

type NewSource struct {
	Source
	SecretID *uuid.UUID
	Keep     bool
}

type SourceSecret struct {
	ProjectID      uuid.UUID
	CredentialsEnc string
}
