package uploads

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"svc-registry/internal/links"
)

const MaxIconBytes = links.MaxIconBytes

var (
	ErrDisabled  error = links.ConflictUploadsDisabled
	ErrFormat    error = links.InvalidIconFormat
	ErrTooLarge        = links.ErrIconTooLarge
	ErrNotFound        = links.ErrNotFound
	idPattern          = regexp.MustCompile(`^[0-9a-f]{64}\.(png|webp|ico|svg)$`)
	contentTypes       = map[string]string{"png": "image/png", "webp": "image/webp", "ico": "image/x-icon", "svg": "image/svg+xml"}
)

type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }

func (s *Store) Enabled() bool { return s != nil && s.root != "" }

func ValidID(id string) bool { return idPattern.MatchString(id) }

func ContentType(id string) string { return contentTypes[strings.TrimPrefix(filepath.Ext(id), ".")] }

func Detect(data []byte) (string, error) {
	switch http.DetectContentType(data) {
	case "image/png":
		return "png", nil
	case "image/webp":
		return "webp", nil
	case "image/x-icon", "image/vnd.microsoft.icon":
		return "ico", nil
	}
	if isSVG(data) {
		return "svg", nil
	}
	return "", ErrFormat
}

func isSVG(data []byte) bool {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		if el, ok := tok.(xml.StartElement); ok {
			return strings.EqualFold(el.Name.Local, "svg")
		}
	}
}

func (s *Store) Put(data []byte) (string, error) {
	if !s.Enabled() {
		return "", ErrDisabled
	}
	if len(data) > MaxIconBytes {
		return "", ErrTooLarge
	}
	ext, err := Detect(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:]) + "." + ext
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if _, err := root.Stat(id); err == nil {
		return id, nil
	}
	tmp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.root, id)); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) Open(id string) (io.ReadCloser, int64, error) {
	if !s.Enabled() || !ValidID(id) {
		return nil, 0, ErrNotFound
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, 0, ErrNotFound
	}
	defer root.Close()
	f, err := root.Open(id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, 0, ErrNotFound
	}
	return f, info.Size(), nil
}

func (s *Store) Exists(id string) bool {
	f, _, err := s.Open(id)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
