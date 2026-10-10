package links

import "io"

const MaxIconBytes = 64 << 10

type IconFiles interface {
	Put(data []byte) (string, error)
	Open(id string) (io.ReadCloser, int64, error)
	Exists(id string) bool
	ContentType(id string) string
}
