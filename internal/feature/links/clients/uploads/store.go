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
	"time"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
)

const MaxIconBytes = linkstypes.MaxIconBytes

var (
	ErrDisabled  error = linkstypes.ConflictUploadsDisabled
	ErrFormat    error = linkstypes.InvalidIconFormat
	ErrTooLarge        = linkstypes.ErrIconTooLarge
	ErrNotFound        = apperr.ErrNotFound
	idPattern          = regexp.MustCompile(`^[0-9a-f]{64}\.(png|webp|ico|svg)$`)
	contentTypes       = map[string]string{"png": "image/png", "webp": "image/webp", "ico": "image/x-icon", "svg": "image/svg+xml"}
)

type FilesClient struct{ root string }

func New(root string) *FilesClient { return &FilesClient{root: root} }

func (s *FilesClient) Enabled() bool { return s != nil && s.root != "" }

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

func (s *FilesClient) Put(data []byte) (string, error) {
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
		now := time.Now()
		return id, root.Chtimes(id, now, now)
	}
	tmp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
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

func (s *FilesClient) Open(id string) (io.ReadCloser, int64, error) {
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
		_ = f.Close()
		return nil, 0, ErrNotFound
	}
	return f, info.Size(), nil
}

func (s *FilesClient) ContentType(id string) string { return ContentType(id) }

func (s *FilesClient) Touch(id string) bool {
	f, _, err := s.Open(id)
	if err != nil {
		return false
	}
	_ = f.Close()
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return false
	}
	defer root.Close()
	now := time.Now()
	return root.Chtimes(id, now, now) == nil
}

func (s *FilesClient) List() ([]linkstypes.IconFile, error) {
	if !s.Enabled() {
		return nil, nil
	}
	root, err := os.OpenRoot(s.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	out := make([]linkstypes.IconFile, 0, len(entries))
	for _, e := range entries {
		upload := strings.HasPrefix(e.Name(), ".upload-")
		if !e.Type().IsRegular() || (!upload && !ValidID(e.Name())) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, linkstypes.IconFile{Name: e.Name(), Upload: upload, Modified: info.ModTime()})
	}
	return out, nil
}

func (s *FilesClient) Remove(name string) error {
	if !s.Enabled() || (!ValidID(name) && !strings.HasPrefix(name, ".upload-")) {
		return ErrNotFound
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
