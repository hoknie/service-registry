package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sync"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

var keyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type StoreClient struct {
	dir  string
	mu   sync.Mutex
	root *os.Root
}

func New(dir string) *StoreClient { return &StoreClient{dir: dir} }

func (s *StoreClient) Name() string { return "local" }

func (s *StoreClient) open() (*os.Root, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != nil {
		return s.root, nil
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return nil, err
	}
	s.root = root
	return root, nil
}

func objectPath(key string) (string, string, error) {
	if !keyPattern.MatchString(key) {
		return "", "", apperr.InternalDetail("invalid blob key " + key)
	}
	return key[:2], key[:2] + "/" + key[2:], nil
}

func unavailable(err error) error {
	if err == nil {
		return nil
	}
	return skillstypes.StorageUnavailable
}

func (s *StoreClient) Put(_ context.Context, key string, _ int64, r io.Reader) error {
	dir, name, err := objectPath(key)
	if err != nil {
		return err
	}
	root, err := s.open()
	if err != nil {
		return unavailable(err)
	}
	if _, err := root.Stat(name); err == nil {
		return nil
	}
	if err := root.Mkdir(dir, 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
		return unavailable(err)
	}
	tmp := name + "." + randomSuffix() + ".tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return unavailable(err)
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	if err != nil {
		_ = root.Remove(tmp)
		return unavailable(err)
	}
	return nil
}

func randomSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *StoreClient) Open(_ context.Context, key string) (io.ReadCloser, error) {
	_, name, err := objectPath(key)
	if err != nil {
		return nil, err
	}
	root, err := s.open()
	if err != nil {
		return nil, unavailable(err)
	}
	f, err := root.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, apperr.ErrNotFound
	}
	return f, unavailable(err)
}

func (s *StoreClient) Has(_ context.Context, key string) (bool, error) {
	_, name, err := objectPath(key)
	if err != nil {
		return false, err
	}
	root, err := s.open()
	if err != nil {
		return false, unavailable(err)
	}
	_, err = root.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, unavailable(err)
}

func (s *StoreClient) Delete(_ context.Context, key string) error {
	_, name, err := objectPath(key)
	if err != nil {
		return err
	}
	root, err := s.open()
	if err != nil {
		return unavailable(err)
	}
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return unavailable(err)
	}
	return nil
}
