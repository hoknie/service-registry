package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type EnginePort interface {
	Name() string
	Modes() []knowledgetypes.Mode
	DefaultMode() knowledgetypes.Mode
	Handles(m knowledgetypes.Mode) bool
	Search(ctx context.Context, q knowledgetypes.EngineQuery) ([]knowledgetypes.Hit, bool, error)
}

type ExternalIndexPort interface {
	Sync(ctx context.Context, project uuid.UUID, docs []knowledgetypes.IndexDoc) error
	Retain(ctx context.Context, projects []uuid.UUID) error
	Target() string
}

type ReaderPort interface {
	Heads(ctx context.Context) (heads map[string]string, defaultBranch string, err error)
	Tree(ctx context.Context, branch, head string) ([]knowledgetypes.Entry, bool, error)
	Read(ctx context.Context, e knowledgetypes.Entry, limit int64) ([]byte, error)
}

type RepoMetadataPort interface {
	Metadata(ctx context.Context, known *time.Time) (forgetypes.RemoteRepo, *forgetypes.Details, error)
}

type ReaderFactoryPort interface {
	Open(ctx context.Context, s knowledgetypes.Source, settings knowledgetypes.Settings, token string) (ReaderPort, error)
}

type EmbedderPort interface {
	Model() string
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
