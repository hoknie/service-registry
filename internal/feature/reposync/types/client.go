package types

import "time"

type Endpoint struct {
	Kind    Kind
	APIURL  string
	Owner   string
	Token   string
	Timeout time.Duration
}

type TreeEntry struct {
	Path    string
	BlobSHA string
	Size    int64
}
