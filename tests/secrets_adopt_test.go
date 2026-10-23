package tests

import (
	"fmt"
	"strings"
	"testing"

	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
	"svc-registry/pkg/testing/forgefake"
)

type legacySetup struct {
	app     *testApp
	root    string
	acme    string
	conn    string
	cluster string
	source  string
}

func inlineColumns(t *testing.T, a *testApp) int64 {
	t.Helper()
	return scalar[int64](t, a.db, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND table_name IN ('forge_connections', 'knowledge_sources', 'clusters') "+
		"AND column_name IN ('credentials_enc', 'credentials_ref', 'credentials_fingerprint')")
}

func withInlineCredentials(t *testing.T) legacySetup {
	t.Helper()
	app := startForgeApp(t)
	s := legacySetup{app: app, root: app.admin()}
	s.acme = app.nodeID(s.root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	s.conn = idOf(app.connect(s.root, s.acme, f, nil))
	s.cluster = idOf(app.cluster(s.root, "prod", nil))
	s.source = app.nodeID(s.root, "project", s.acme, "docs")
	r := app.send("PUT", nodePath(s.source)+"/knowledge/source", s.root, obj{"kind": "remote", "forge": "github",
		"url": "https://github.example/acme-inc/docs", "api_url": f.APIURL(), "credentials": nil})
	eq(t, r.status, 200, r.text())

	_, err := rollbackThrough(t, embedded(t), app.db, squashMigration)
	must(t, err)
	eq(t, inlineColumns(t, app), int64(9))
	box := secretbox.New(app.services.Config.Secrets.Keys)
	connEnc, err := box.Seal(forgeToken, secretbox.AAD("forge_connections", s.conn, "credentials_enc"))
	must(t, err)
	sourceEnc, err := box.Seal(forgeToken, secretbox.AAD("knowledge_sources", s.source, "credentials_enc"))
	must(t, err)
	execSQL(t, app.db, "UPDATE forge_connections SET credentials_secret_id = NULL, credentials_enc = $2 WHERE id = $1", s.conn, connEnc)
	execSQL(t, app.db, "UPDATE clusters SET credentials_secret_id = NULL, credentials_ref = 'env:K8S_PROD_TOKEN' WHERE id = $1", s.cluster)
	execSQL(t, app.db, "UPDATE knowledge_sources SET credentials_enc = $2 WHERE project_id = $1", s.source, sourceEnc)
	execSQL(t, app.db, "DELETE FROM secrets")
	execSQL(t, app.db, "INSERT INTO secrets (id, node_id, name, value_ref) VALUES (gen_random_uuid(), $1, 'forge-acme-inc', 'env:OTHER')", s.acme)
	return s
}

func TestMigrateStopsBeforeDroppingInlineCredentials(t *testing.T) {
	t.Parallel()
	s := withInlineCredentials(t)
	a := s.app
	env := []string{"DATABASE_URL", a.db.URL, "SECRETS_KEYS", secretsKey}

	code, _, errOut := run(t, []string{"db:migrate"}, "", env...)
	eq(t, code, 1, errOut)
	contains(t, errOut, `error: 3 record(s) still use inline credentials; run "svc-registry secrets:adopt-legacy", then "db:migrate" again`)
	eq(t, scalar[bool](t, a.db, "SELECT dirty FROM "+postgres.Ledger), false)
	eq(t, inlineColumns(t, a), int64(9))

	code, out, errOut := run(t, []string{"secrets:adopt-legacy", "--dry-run"}, "", env...)
	eq(t, code, 0, errOut)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	eq(t, len(lines), 4, out)
	contains(t, lines[0], "forge "+s.conn+" → forge-acme-inc-2 (new)")
	contains(t, lines[1], "source "+s.source+" → forge-acme-inc-2 (reused)")
	contains(t, lines[2], "cluster "+s.cluster+" → cluster-prod (new)")
	eq(t, lines[3], "would adopt 3 credential(s)")
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM secrets"), int64(1), "a dry run changes nothing")

	code, out, errOut = run(t, []string{"secrets:adopt-legacy"}, "", env...)
	eq(t, code, 0, errOut)
	eq(t, strings.TrimSpace(out), "adopted 3 credential(s) into 2 secret(s)")
	lacks(t, out+errOut, forgeToken)
	shared := scalar[string](t, a.db, "SELECT credentials_secret_id::text FROM forge_connections WHERE id = $1", s.conn)
	eq(t, scalar[string](t, a.db, "SELECT credentials_secret_id::text FROM knowledge_sources WHERE project_id = $1", s.source), shared)
	eq(t, scalar[string](t, a.db, "SELECT node_id::text FROM secrets WHERE id = $1", shared), s.acme)
	eq(t, scalar[string](t, a.db, "SELECT value_ref FROM secrets WHERE node_id IS NULL AND name = 'cluster-prod'"), "env:K8S_PROD_TOKEN")
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM forge_connections WHERE credentials_enc IS NOT NULL"), int64(0))

	code, out, errOut = run(t, []string{"secrets:adopt-legacy"}, "", env...)
	eq(t, code, 0, errOut)
	eq(t, strings.TrimSpace(out), "adopted 0 credential(s) into 0 secret(s)")

	code, out, errOut = run(t, []string{"db:migrate"}, "", env...)
	eq(t, code, 0, errOut)
	dropped := migrationsFrom(t, embedded(t), squashMigration)
	eq(t, strings.TrimSpace(out), fmt.Sprintf("applied %d migration(s)", dropped))
	eq(t, inlineColumns(t, a), int64(0))
	c := a.get(connectionPath(s.acme, s.conn), s.root).json(t)
	eq(t, at(c, "credentials", "kind"), any("secret"))
	eq(t, at(c, "credentials", "secret", "name"), any("forge-acme-inc-2"))
	lacks(t, a.get(clustersPath+"/"+s.cluster, s.root).text(), "legacy")
	code, out, errOut = run(t, []string{"forge:sync", s.conn}, "", env...)
	eq(t, code, 0, errOut+out)

	code, out, errOut = run(t, []string{"db:rollback", "--count", fmt.Sprint(dropped)}, "", env...)
	eq(t, code, 0, errOut)
	contains(t, out, fmt.Sprintf("reverted %d migration(s)", dropped))
	eq(t, inlineColumns(t, a), int64(9))
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM forge_connections WHERE credentials_secret_id IS NULL"), int64(0))
}

func TestAdoptLegacyWithoutTheKeyChangesNothing(t *testing.T) {
	t.Parallel()
	s := withInlineCredentials(t)
	a := s.app
	code, _, errOut := run(t, []string{"secrets:adopt-legacy"}, "", "DATABASE_URL", a.db.URL, "SECRETS_KEYS", secretsKey2)
	eq(t, code, 1)
	contains(t, errOut, "forge connection "+s.conn)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM secrets"), int64(1))
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM forge_connections WHERE credentials_enc IS NOT NULL"), int64(1))
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM clusters WHERE credentials_ref IS NOT NULL"), int64(1))
}
