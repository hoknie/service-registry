package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/presentation/jobs"
	"svc-registry/internal/testsupport"
)

var greenSVG = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect fill="green"/></svg>`)

func ageFile(t *testing.T, path string, age time.Duration) {
	t.Helper()
	at := time.Now().Add(-age)
	must(t, os.Chtimes(path, at, at))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestLinkIconRetentionDeletesUnreferencedOldFiles(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	beta := a.nodeID(admin, "organization", "", "beta")
	dir := a.cfg.Uploads.Dir
	upload := func(data []byte) string {
		r := a.upload(admin, acme, "x", data)
		eq(t, r.status, 201, r.text())
		return r.json(t)["id"].(string)
	}
	putIcon := func(node, key, file string) {
		r := a.send("PUT", templatesPath(node)+"/"+key, admin, obj{"kind_key": "grafana",
			"template": "https://grafana.example/{project.slug}", "icon": obj{"file": file}})
		eq(t, r.status, 200, r.text())
	}
	replaced := upload(tinyPNG)
	shared := upload(greenSVG)
	fresh := upload([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle/></svg>`))
	putIcon(acme, "grafana", replaced)
	putIcon(acme, "shared", shared)
	putIcon(beta, "shared", shared)
	current := upload([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><path/></svg>`))
	putIcon(acme, "grafana", current)
	eq(t, a.send("DELETE", templatesPath(acme)+"/shared", admin, nil).status, 204)

	must(t, os.WriteFile(filepath.Join(dir, "README.txt"), []byte("x"), 0o600))
	must(t, os.WriteFile(filepath.Join(dir, ".upload-1"), []byte("x"), 0o600))
	must(t, os.Mkdir(filepath.Join(dir, "backup"), 0o750))
	for _, name := range []string{replaced, shared, current, "README.txt", ".upload-1", "backup"} {
		ageFile(t, filepath.Join(dir, name), 30*time.Hour)
	}

	n, err := a.services.Links.Icons.PruneLinkIcons(context.Background())
	must(t, err)
	eq(t, n, 2)
	eq(t, exists(filepath.Join(dir, replaced)), false, "replaced icon")
	eq(t, exists(filepath.Join(dir, ".upload-1")), false, "stale upload")
	eq(t, a.get("/api/v1/link-icons/"+replaced, admin).status, 404)
	for _, name := range []string{shared, current, fresh, "README.txt", "backup"} {
		eq(t, exists(filepath.Join(dir, name)), true, name)
	}
	putIcon(acme, "fresh", fresh)
}

func TestLinkIconRetentionRespectsRefreshedFiles(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	org := a.nodeID(admin, "organization", "", "acme")
	dir := a.cfg.Uploads.Dir
	r := a.upload(admin, org, "x.svg", greenSVG)
	eq(t, r.status, 201, r.text())
	id := r.json(t)["id"].(string)
	ageFile(t, filepath.Join(dir, id), 30*time.Hour)
	eq(t, a.upload(admin, org, "again.svg", greenSVG).json(t)["id"], any(id))

	n, err := a.services.Links.Icons.PruneLinkIcons(context.Background())
	must(t, err)
	eq(t, n, 0)
	eq(t, exists(filepath.Join(dir, id)), true)

	ageFile(t, filepath.Join(dir, id), 30*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnLinkIconRetention(ctx, a.services.Jobs())
	deadline := time.Now().Add(5 * time.Second)
	for exists(filepath.Join(dir, id)) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	eq(t, exists(filepath.Join(dir, id)), false, "the job deletes on its first pass")
}

func TestLinkIconRetentionFollowsConfiguredTTL(t *testing.T) {
	t.Parallel()
	a := startApp(t, "UPLOADS_UNREFERENCED_TTL_SECS", "600")
	admin := a.admin()
	org := a.nodeID(admin, "organization", "", "acme")
	dir := a.cfg.Uploads.Dir
	upload := func(data []byte) string {
		r := a.upload(admin, org, "x.svg", data)
		eq(t, r.status, 201, r.text())
		return r.json(t)["id"].(string)
	}
	old := upload(greenSVG)
	recent := upload([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle/></svg>`))
	ageFile(t, filepath.Join(dir, old), 20*time.Minute)
	ageFile(t, filepath.Join(dir, recent), 5*time.Minute)

	n, err := a.services.Links.Icons.PruneLinkIcons(context.Background())
	must(t, err)
	eq(t, n, 1)
	eq(t, exists(filepath.Join(dir, old)), false, "older than the configured TTL")
	eq(t, exists(filepath.Join(dir, recent)), true, "younger than the configured TTL")
}

func TestLinkIconRetentionKeepsFilesWithoutDatabase(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "uploads")
	must(t, os.MkdirAll(dir, 0o750))
	name := strings.Repeat("a", 64) + ".png"
	must(t, os.WriteFile(filepath.Join(dir, name), tinyPNG, 0o600))
	ageFile(t, filepath.Join(dir, name), 30*time.Hour)
	cfg := testConfig(t, "postgres://u:p@127.0.0.1:1/x", testsupport.MissingDist(t).Dir, "UPLOADS_DIR", dir)
	services, err := svcapp.New(cfg)
	must(t, err)
	t.Cleanup(services.Close)
	_, err = services.Links.Icons.PruneLinkIcons(context.Background())
	eq(t, err != nil, true)
	eq(t, exists(filepath.Join(dir, name)), true)

	off := testConfig(t, "postgres://u:p@127.0.0.1:1/x", testsupport.MissingDist(t).Dir, "UPLOADS_DIR", "off")
	disabled, err := svcapp.New(off)
	must(t, err)
	t.Cleanup(disabled.Close)
	n, err := disabled.Links.Icons.PruneLinkIcons(context.Background())
	must(t, err)
	eq(t, n, 0)
}
