package ports

import (
	"context"
	"io"
)

type BlobStorePort interface {
	Name() string
	Put(ctx context.Context, key string, size int64, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Has(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
}
