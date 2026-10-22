package ports

import (
	"context"
	"io"

	linkstypes "svc-registry/internal/feature/links/types"
)

type IconFilesPort interface {
	Put(data []byte) (string, error)
	Open(id string) (io.ReadCloser, int64, error)
	Exists(id string) bool
	ContentType(id string) string
}

type CheckerPort interface {
	Check(ctx context.Context, url string) linkstypes.Result
}
