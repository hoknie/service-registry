package tests

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	svcapp "svc-registry/internal/app"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type memoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	down    bool
}

func newMemoryStore() *memoryStore { return &memoryStore{objects: map[string][]byte{}} }

func (m *memoryStore) Name() string { return "memory" }

func (m *memoryStore) Put(_ context.Context, key string, _ int64, r io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		return skillstypes.StorageUnavailable
	}
	data, err := io.ReadAll(r)
	m.objects[key] = data
	return err
}

func (m *memoryStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		return nil, skillstypes.StorageUnavailable
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memoryStore) Has(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok, nil
}

func (m *memoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *memoryStore) setDown(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.down = v
}

func (m *memoryStore) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

func TestSkillsStorageOutageOnlyBreaksContent(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	a := startAppWith(t, []svcapp.Option{svcapp.WithSkillsStore(store)})
	admin := a.admin()
	id := idOf(a.skill(admin, "go-testing", "status", "published"))
	store.setDown(true)
	r := a.get(skillPath(id)+"/archive", admin)
	eq(t, r.status, 503)
	eq(t, code(t, r), any("skills.storage_unavailable"))
	eq(t, a.get(skillsPath, admin).status, 200)
	r = a.uploadSkill(admin, map[string]string{"SKILL.md": skillMD("other", "")})
	eq(t, r.status, 503)
	eq(t, a.count("skills"), int64(1))
	store.setDown(false)
	eq(t, a.get(skillPath(id)+"/archive", admin).status, 200)

	off := startApp(t, "SKILLS_STORAGE", "off")
	r = off.uploadSkill(off.admin(), map[string]string{"SKILL.md": skillMD("x", "")})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.skills_storage_disabled"))
}

func TestSkillFilesAreSharedAndPruned(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	a := startAppWith(t, []svcapp.Option{svcapp.WithSkillsStore(store)})
	admin := a.admin()
	shared := map[string]string{"LICENSE": "MIT"}
	one := a.uploadSkill(admin, map[string]string{"SKILL.md": skillMD("one", ""), "LICENSE": shared["LICENSE"]})
	two := a.uploadSkill(admin, map[string]string{"SKILL.md": skillMD("two", ""), "LICENSE": shared["LICENSE"]})
	eq(t, one.status+two.status, 402)
	eq(t, store.len(), 3)
	eq(t, a.count("skill_blobs"), int64(3))
	eq(t, a.call("DELETE", skillPath(idOf(one.json(t))), admin).status, 204)
	n, err := a.services.Skills.Ports.PruneBlobs(context.Background())
	must(t, err)
	eq(t, n, 0, "fresh files stay for a day")
	execSQL(t, a.db, "UPDATE skill_blobs SET last_referenced_at = now() - interval '2 days'")
	n, err = a.services.Skills.Ports.PruneBlobs(context.Background())
	must(t, err)
	eq(t, n, 1)
	eq(t, store.len(), 2)
	eq(t, a.get(skillPath(idOf(two.json(t)))+"/archive", admin).status, 200)
}

func TestSkillsStorageCopyMovesFilesToS3(t *testing.T) {
	t.Parallel()
	raw := os.Getenv("TEST_S3_URL")
	if raw == "" {
		t.Skip("TEST_S3_URL is not set")
	}
	u, err := url.Parse(raw)
	must(t, err)
	secret, _ := u.User.Password()
	bucket, prefix := "svcr-tests", uuid.Must(uuid.NewV7()).String()+"/"
	client, err := minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(u.User.Username(), secret, ""), Region: "us-east-1",
		BucketLookup: minio.BucketLookupPath})
	must(t, err)
	deadline := time.Now().Add(90 * time.Second)
	for {
		err = client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: "us-east-1"})
		if code := minio.ToErrorResponse(err).Code; code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
			err = nil
		}
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	must(t, err)

	a := startApp(t)
	admin := a.admin()
	id := idOf(a.skill(admin, "go-testing", "status", "published"))
	s3 := []string{"SKILLS_S3_ENDPOINT", "http://" + u.Host, "SKILLS_S3_BUCKET", bucket, "SKILLS_S3_ACCESS_KEY", u.User.Username(),
		"SKILLS_S3_SECRET_KEY", secret, "SKILLS_S3_PATH_STYLE", "true", "SKILLS_S3_PREFIX", prefix}
	env := append([]string{"DATABASE_URL", a.db.URL, "SKILLS_DIR", a.cfg.Skills.DirRaw}, s3...)
	code, out, errOut := run(t, []string{"skills:storage-copy"}, "", env...)
	eq(t, code, 0, errOut)
	eq(t, strings.TrimSpace(out), "copied 2 blob(s), skipped 0")
	_, out, _ = run(t, []string{"skills:storage-copy"}, "", env...)
	eq(t, strings.TrimSpace(out), "copied 0 blob(s), skipped 2")

	a.restart(append([]string{"SKILLS_STORAGE", "s3"}, s3...)...)
	admin = a.session("root@example.com", password)
	eq(t, a.get(skillPath(id)+"/archive", admin).status, 200)

	code, _, errOut = run(t, []string{"skills:storage-copy"}, "", "DATABASE_URL", a.db.URL)
	eq(t, code, 1)
	contains(t, errOut, "SKILLS_S3_BUCKET")
}
