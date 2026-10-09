package tests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/config"
	"svc-registry/internal/testsupport"
)

func migrated(t *testing.T) *testsupport.TestDB {
	t.Helper()
	tdb := testsupport.NewTestDB(t)
	migrate(t, tdb)
	return tdb
}

func userCreate(t *testing.T, url string, args []string, stdin string, env ...string) (int, string, string) {
	t.Helper()
	env = append([]string{"DATABASE_URL", url, "PASSWORD_HASH_MEMORY_KIB", "1024", "PASSWORD_HASH_ITERATIONS", "1"}, env...)
	return run(t, append([]string{"user:create"}, args...), stdin, env...)
}

func TestUserCreateReadsThePasswordFromStdin(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	code, out, errOut := userCreate(t, tdb.URL, []string{"--email", "Root@Example.com", "--name", "Root", "--superadmin"}, "long enough pw\n")
	eq(t, code, 0, errOut)
	rest, ok := strings.CutPrefix(strings.TrimSpace(out), "created user ")
	eq(t, ok, true, out)
	id, email, _ := strings.Cut(rest, " ")
	eq(t, uuid.MustParse(id).Version(), uuid.Version(7))
	eq(t, email, "root@example.com")

	app := appOver(t, tdb)
	r := app.login("root@example.com", "long enough pw")
	eq(t, r.status, 200)
	eq(t, r.json(t)["is_superadmin"], any(true))
	eq(t, r.json(t)["display_name"], any("Root"))
}

func TestUserCreatePrefersUserPasswordAndReportsConflicts(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	code, _, errOut := userCreate(t, tdb.URL, []string{"--email=ann@example.com", "--name=Ann"}, "ignored stdin line\n", "USER_PASSWORD", "from the environment")
	eq(t, code, 0, errOut)

	code, _, errOut = userCreate(t, tdb.URL, []string{"--email", "ANN@example.com", "--name", "Ann again"}, "", "USER_PASSWORD", "another password")
	eq(t, code, 1)
	contains(t, errOut, "conflict.email_taken")

	code, _, errOut = userCreate(t, tdb.URL, []string{"--email", "c@example.com", "--name", "C"}, "short\n")
	eq(t, code, 1)
	contains(t, errOut, "validation.password_too_short")

	app := appOver(t, tdb)
	r := app.login("ann@example.com", "from the environment")
	eq(t, r.status, 200)
	eq(t, r.json(t)["is_superadmin"], any(false))
}

func TestServeBootstrapsTheFirstSuperadminOnAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	addr := closedPort(t)
	cmd := exec.Command(bin, "serve")
	cmd.Env = []string{
		"DATABASE_URL=" + tdb.URL, "HTTP_ADDR=" + addr, "WEB_DIST_DIR=/nonexistent/svc-registry-web",
		"PASSWORD_HASH_MEMORY_KIB=1024", "BOOTSTRAP_ADMIN_EMAIL=Admin@Example.com",
		"BOOTSTRAP_ADMIN_PASSWORD=bootstrap pass 1",
	}
	must(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	waitListening(t, addr)
	body := `{"email":"admin@example.com","password":"bootstrap pass 1"}`
	deadline := time.Now().Add(20 * time.Second)
	for {
		r := request(t, addr, "POST", "/api/v1/auth/login", body, "content-type", "application/json")
		if r.status == 200 {
			eq(t, r.json(t)["is_superadmin"], any(true))
			eq(t, r.json(t)["display_name"], any("Administrator"))
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("admin never appeared: %d %s", r.status, r.body)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestBootstrapDoesNothingWhenUsersExist(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	<-svcapp.SpawnBootstrapAdmin(context.Background(), app.state, config.BootstrapAdmin{Email: "admin@example.com", Password: "bootstrap pass 1"})
	rows, err := app.db.Pool.Query(context.Background(), "SELECT email FROM users")
	must(t, err)
	var emails []string
	for rows.Next() {
		var e string
		must(t, rows.Scan(&e))
		emails = append(emails, e)
	}
	eq(t, strings.Join(emails, ","), "ann@example.com")
}

func TestBootstrapRetriesUntilTheSchemaExists(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	app := appOver(t, tdb)
	done := svcapp.SpawnBootstrapAdmin(context.Background(), app.state, config.BootstrapAdmin{Email: "admin@example.com", Password: "bootstrap pass 1"})
	time.Sleep(300 * time.Millisecond)
	migrate(t, tdb)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("bootstrap did not finish after the schema appeared")
	}
	eq(t, app.login("admin@example.com", "bootstrap pass 1").status, 200)
}
