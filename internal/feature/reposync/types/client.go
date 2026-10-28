package types

type Endpoint struct {
	Kind   Kind
	APIURL string
	Owner  string
	Token  string
}

type TreeEntry struct {
	Path    string
	BlobSHA string
	Size    int64
}
