package tests

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func run(t *testing.T, args []string, stdin string, env ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{}
	for i := 0; i+1 < len(env); i += 2 {
		cmd.Env = append(cmd.Env, env[i]+"="+env[i+1])
	}
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), out.String(), errOut.String()
	} else if err != nil {
		t.Fatal(err)
	}
	return 0, out.String(), errOut.String()
}

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}

func lacks(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Fatalf("%q contains %q", s, sub)
	}
}

func TestNoCommandPrintsHelp(t *testing.T) {
	code, out, _ := run(t, nil, "")
	eq(t, code, 0)
	for _, cmd := range []string{"serve", "db:migrate", "db:rollback", "user:create"} {
		contains(t, out, cmd)
	}
}

func TestVersionPrintsThePackageVersion(t *testing.T) {
	code, out, _ := run(t, []string{"--version"}, "")
	eq(t, code, 0)
	eq(t, strings.TrimSpace(out), "svc-registry 0.1.0")
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	code, _, errOut := run(t, []string{"frobnicate"}, "")
	eq(t, code, 2)
	contains(t, errOut, `unknown command "frobnicate"`)
	contains(t, errOut, "Usage:")
}

func TestBadRollbackCountIsAUsageError(t *testing.T) {
	for _, args := range [][]string{{"db:rollback", "--count", "0"}, {"db:rollback", "--count"}, {"db:rollback", "-x"}} {
		code, _, _ := run(t, args, "")
		eq(t, code, 2, args)
	}
}

func TestServeWithoutDatabaseURLFailsNamingTheVariable(t *testing.T) {
	code, _, errOut := run(t, []string{"serve"}, "")
	eq(t, code, 1)
	contains(t, errOut, "DATABASE_URL is required")
}

func TestServeReportsEveryInvalidVariable(t *testing.T) {
	code, _, errOut := run(t, []string{"serve"}, "",
		"DATABASE_URL", "postgres://u:p@127.0.0.1:1/x", "SHUTDOWN_TIMEOUT_SECS", "999", "HTTP_ADDR", "nowhere")
	eq(t, code, 1)
	contains(t, errOut, "SHUTDOWN_TIMEOUT_SECS")
	contains(t, errOut, "HTTP_ADDR")
	eq(t, strings.Count(strings.TrimSpace(errOut), "\n"), 0, "one line")
}

func TestMigrateReadsOnlyTheDatabaseSlice(t *testing.T) {
	code, _, errOut := run(t, []string{"db:migrate"}, "", "HTTP_ADDR", "nowhere")
	eq(t, code, 1)
	contains(t, errOut, "DATABASE_URL is required")
	lacks(t, errOut, "HTTP_ADDR")
}

func TestUserCreateRefusesAPasswordArgument(t *testing.T) {
	for _, args := range [][]string{
		{"user:create", "--email", "a@example.com", "--name", "A", "--password", "secret"},
		{"user:create", "--email", "a@example.com", "--name", "A", "--password=secret"},
	} {
		code, _, errOut := run(t, args, "", "DATABASE_URL", "postgres://u:p@127.0.0.1:1/x")
		eq(t, code, 2, args)
		contains(t, errOut, "USER_PASSWORD")
		contains(t, errOut, "stdin")
	}
}

func TestUserCreateRequiresEmailAndName(t *testing.T) {
	code, _, errOut := run(t, []string{"user:create", "--name", "A"}, "")
	eq(t, code, 2)
	contains(t, errOut, `required flag(s) "email" not set`)
	code, _, errOut = run(t, []string{"user:create", "--email", "a@example.com"}, "")
	eq(t, code, 2)
	contains(t, errOut, `required flag(s) "name" not set`)
	code, _, _ = run(t, []string{"user:create", "--email"}, "")
	eq(t, code, 2)
}

func TestUserCreateReadsOnlyItsSlice(t *testing.T) {
	code, _, errOut := run(t, []string{"user:create", "--email", "a@example.com", "--name", "A"}, "",
		"SESSION_IDLE_TIMEOUT_SECS", "1", "HTTP_ADDR", "nowhere", "USER_PASSWORD", "long enough pw")
	eq(t, code, 1)
	contains(t, errOut, "DATABASE_URL is required")
	lacks(t, errOut, "SESSION_IDLE_TIMEOUT_SECS")
	lacks(t, errOut, "HTTP_ADDR")
}

func TestServeRejectsInconsistentSessionTimeouts(t *testing.T) {
	code, _, errOut := run(t, []string{"serve"}, "", "DATABASE_URL", "postgres://u:p@127.0.0.1:1/x",
		"SESSION_IDLE_TIMEOUT_SECS", "7200", "SESSION_ABSOLUTE_TIMEOUT_SECS", "3600")
	eq(t, code, 1)
	contains(t, errOut, "SESSION_ABSOLUTE_TIMEOUT_SECS")
}

func TestServeRejectsHalfOrInvalidBootstrapAdmin(t *testing.T) {
	db := []string{"DATABASE_URL", "postgres://u:p@127.0.0.1:1/x"}
	code, _, errOut := run(t, []string{"serve"}, "", append(db, "BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")...)
	eq(t, code, 1)
	contains(t, errOut, "BOOTSTRAP_ADMIN_PASSWORD")

	code, _, errOut = run(t, []string{"serve"}, "", append(db, "BOOTSTRAP_ADMIN_EMAIL", "admin@example.com", "BOOTSTRAP_ADMIN_PASSWORD", "short")...)
	eq(t, code, 1)
	contains(t, errOut, "BOOTSTRAP_ADMIN_PASSWORD")
	lacks(t, errOut, `short"`)

	code, _, errOut = run(t, []string{"serve"}, "", append(db, "BOOTSTRAP_ADMIN_EMAIL", "not-an-email", "BOOTSTRAP_ADMIN_PASSWORD", "long enough pw")...)
	eq(t, code, 1)
	contains(t, errOut, "BOOTSTRAP_ADMIN_EMAIL")
}

func TestServeStopsCleanlyOnSIGTERM(t *testing.T) {
	addr := closedPort(t)
	cmd := exec.Command(bin, "serve")
	cmd.Env = []string{"DATABASE_URL=" + deadDB, "HTTP_ADDR=" + addr, "WEB_DIST_DIR=/nonexistent/svc-registry-web"}
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitListening(t, addr)
	eq(t, request(t, addr, "GET", "/api/health", "").status, 200)
	must(t, cmd.Process.Signal(sigterm))
	err := cmd.Wait()
	eq(t, err, nil, errOut.String())
	contains(t, errOut.String(), "shutting down")
	contains(t, errOut.String(), "stopped")
}

func TestMCPStdioBridge(t *testing.T) {
	code, out, _ := run(t, nil, "")
	eq(t, code, 0)
	contains(t, out, "mcp:stdio")
	code, _, errOut := run(t, []string{"mcp:stdio", "--token-env", "SVCR_TOKEN"}, "")
	eq(t, code, 2)
	contains(t, errOut, `required flag(s) "url" not set`)
	code, _, errOut = run(t, []string{"mcp:stdio", "--url", "http://127.0.0.1:1", "--token-env", "SVCR_TOKEN"}, "")
	eq(t, code, 1)
	contains(t, errOut, "SVCR_TOKEN")

	d, token := mcpDocs(t)
	in := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_doc","arguments":{"project":"acme/api","path":"README.md"}}}` + "\n"
	code, out, errOut = run(t, []string{"mcp:stdio", "--url", "http://" + d.app.addr, "--token-env", "SVCR_TOKEN"}, in,
		"SVCR_TOKEN", token, "DATABASE_URL", "not-a-url", "HTTP_ADDR", "nowhere")
	eq(t, code, 0, errOut)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	eq(t, len(lines), 2, out)
	eq(t, lines[0], `{"jsonrpc":"2.0","id":1,"result":{}}`)
	contains(t, lines[1], "idempotency key")
	lacks(t, out+errOut, token)
	code, out, _ = run(t, []string{"mcp:stdio", "--url", "http://" + d.app.addr, "--token-env", "SVCR_TOKEN"},
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`+"\n", "SVCR_TOKEN", "svcp_wrong")
	eq(t, code, 0)
	contains(t, out, `"id":7`)
	contains(t, out, "auth.invalid_token")
}
