package knowledge

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/access"
)

type ScanStore interface {
	Record(ctx context.Context, s Scan, keep int, maxGap time.Duration) error
	List(ctx context.Context, f ScanFilter, page access.PageRequest) (access.Page[ScanItem], error)
}

type PatternStore interface {
	Chain(ctx context.Context, nodeID uuid.UUID) ([]ChainNode, error)
	Put(ctx context.Context, nodeID uuid.UUID, s NodeSettings) error
}

type SettingsStore interface {
	RequestCollect(ctx context.Context, projectID uuid.UUID) error
	Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error)
	Extend(ctx context.Context, projectID uuid.UUID, leaseSecs int) error
	Release(ctx context.Context, projectID uuid.UUID, afterSecs uint32, clearForce bool) error
	Project(ctx context.Context, projectID uuid.UUID) (Project, error)
}

type SnapshotStore interface {
	Branches(ctx context.Context, projectID uuid.UUID) ([]Candidate, error)
	LastAttempt(ctx context.Context, projectID uuid.UUID, branch string) (*Snapshot, error)
	Latest(ctx context.Context, projectID uuid.UUID, branch string) (*Snapshot, error)
	Known(ctx context.Context, projectID uuid.UUID, gitBlobSHAs []string) (map[string]Known, error)
	Save(ctx context.Context, s NewSnapshot, keep uint32) error
	Files(ctx context.Context, projectID uuid.UUID, branch string, ref Ref) (Snapshot, []FileInfo, error)
	File(ctx context.Context, projectID uuid.UUID, branch string, ref Ref, path string) (Snapshot, FileContent, error)
	PruneBlobs(ctx context.Context) (int64, error)
}

type SearchStore interface {
	Search(ctx context.Context, v Viewer, s Search) ([]Hit, bool, error)
	Readable(ctx context.Context, v Viewer, project *uuid.UUID) ([]uuid.UUID, error)
	DefaultBranches(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID]string, error)
	Projects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]ProjectName, error)
}

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

type IndexStore interface {
	Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error)
	Extend(ctx context.Context, project uuid.UUID, leaseSecs int) error
	Release(ctx context.Context, project uuid.UUID, afterSecs uint32, failure string) error
	State(ctx context.Context, project uuid.UUID) (IndexState, error)
	Synced(ctx context.Context, project uuid.UUID, st IndexState) error
	Files(ctx context.Context, project uuid.UUID) ([]IndexFile, error)
	Missing(ctx context.Context, project uuid.UUID, model string) ([]BlobContent, error)
	Contents(ctx context.Context, shas [][]byte) (map[string]string, error)
	Save(ctx context.Context, sha256 []byte, model string, chunks []StoredChunk) error
	Chunks(ctx context.Context, shas [][]byte, model string) (map[string][]StoredChunk, error)
	Counts(ctx context.Context, model string) (pending, indexed int, err error)
	Indexed(ctx context.Context) ([]uuid.UUID, error)
	Forget(ctx context.Context, keep []uuid.UUID) error
}

type StoredSource struct {
	Source
	CredentialsEnc *string
	CredentialsRef *string
}

type NewSource struct {
	Source
	CredentialsEnc *string
	CredentialsRef *string
	Fingerprint    *string
	Keep           bool
}

type SourceSecret struct {
	ProjectID      uuid.UUID
	CredentialsEnc string
}

type SourceStore interface {
	Get(ctx context.Context, projectID uuid.UUID) (*StoredSource, error)
	Put(ctx context.Context, projectID uuid.UUID, s NewSource) (Source, error)
	Delete(ctx context.Context, projectID uuid.UUID) error
	SetHeads(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string, branches map[string]string) error
	Secrets(ctx context.Context) ([]SourceSecret, error)
	ReplaceSecret(ctx context.Context, projectID uuid.UUID, enc string) error
}
