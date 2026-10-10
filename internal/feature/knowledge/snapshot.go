package knowledge

import (
	"bytes"
	"crypto/sha256"
	"slices"
	"unicode/utf8"
)

type Entry struct {
	Path    string
	BlobSHA string
	Size    int64
}

type SkipReason string

const (
	SkipTooLarge SkipReason = "too_large"
	SkipBinary   SkipReason = "binary"
	SkipLimit    SkipReason = "limit"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusPartial Status = "partial"
	StatusFailed  Status = "failed"
)

type Limits struct {
	MaxFileBytes     int64
	MaxFiles         int
	MaxSnapshotBytes int64
}

type File struct {
	Path       string
	GitBlobSHA string
	Bytes      int64
	Content    []byte
	SHA256     [32]byte
	Skip       SkipReason
	Kind       Kind
	Meta       Meta
}

type Fetch func(e Entry, limit int64) ([]byte, error)

func IsText(content []byte) bool {
	return bytes.IndexByte(content, 0) < 0 && utf8.Valid(content)
}

func Collect(entries []Entry, s Settings, l Limits, fetch Fetch) ([]File, error) {
	var picked []Entry
	for _, e := range entries {
		if s.Collects(e.Path) {
			picked = append(picked, e)
		}
	}
	slices.SortFunc(picked, func(a, b Entry) int {
		switch {
		case a.Path < b.Path:
			return -1
		case a.Path > b.Path:
			return 1
		}
		return 0
	})
	files := make([]File, 0, len(picked))
	var stored int
	var total int64
	for _, e := range picked {
		f := File{Path: e.Path, GitBlobSHA: e.BlobSHA, Bytes: max(e.Size, 0)}
		f.Kind, f.Meta = Classify(e.Path)
		switch {
		case stored >= l.MaxFiles:
			f.Skip = SkipLimit
		case e.Size > l.MaxFileBytes:
			f.Skip = SkipTooLarge
		}
		if f.Skip == "" {
			content, err := fetch(e, l.MaxFileBytes)
			if err != nil {
				return nil, err
			}
			f.Bytes = int64(len(content))
			switch {
			case f.Bytes > l.MaxFileBytes:
				f.Skip = SkipTooLarge
			case !IsText(content):
				f.Skip = SkipBinary
			case total+f.Bytes > l.MaxSnapshotBytes:
				f.Skip = SkipLimit
			default:
				f.Content = content
				f.SHA256 = sha256.Sum256(content)
				f.Meta = Parse(f.Kind, e.Path, string(content), f.Meta)
				stored++
				total += f.Bytes
			}
		}
		files = append(files, f)
	}
	return files, nil
}

func SnapshotStatus(files []File, truncated bool) Status {
	if truncated {
		return StatusPartial
	}
	for _, f := range files {
		if f.Skip != "" {
			return StatusPartial
		}
	}
	return StatusOK
}

func Tree(files []File) map[string]string {
	out := make(map[string]string, len(files))
	for _, f := range files {
		if f.Skip != "" {
			out[f.Path] = "skip:" + string(f.Skip)
		} else {
			out[f.Path] = string(f.SHA256[:])
		}
	}
	return out
}
