package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/access"
	svcapp "svc-registry/internal/app"
	"svc-registry/internal/config"
	"svc-registry/internal/postgres"
	"svc-registry/internal/service"
	"svc-registry/internal/testsupport"
	"svc-registry/internal/webui"
)

const deadDB = "postgres://u:p@127.0.0.1:1/none"

const password = "correct horse battery"

func testConfig(t testing.TB, dbURL, dist string, extra ...string) config.Config {
	t.Helper()
	vars := config.Env{
		"DATABASE_URL":                  dbURL,
		"DATABASE_ACQUIRE_TIMEOUT_SECS": "1",
		"WEB_DIST_DIR":                  dist,
		"PASSWORD_HASH_MEMORY_KIB":      "1024",
		"PASSWORD_HASH_ITERATIONS":      "1",
		"UPLOADS_DIR":                   filepath.Join(t.TempDir(), "uploads"),
	}
	for i := 0; i+1 < len(extra); i += 2 {
		vars[extra[i]] = extra[i+1]
	}
	cfg, err := config.Load[config.Config](vars)
	if err != nil {
		t.Fatalf("test config: %v", err)
	}
	return cfg
}

func serveState(t testing.TB, state *service.State, dist string) string {
	t.Helper()
	app := svcapp.BuildRouter(state, webui.NewDist(config.WebConfig{DistDir: dist}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		_ = app.ShutdownWithTimeout(time.Second)
		state.DB.Close()
	})
	return ln.Addr().String()
}

func spawnApp(t testing.TB, dbURL, dist string, extra ...string) string {
	t.Helper()
	state, err := svcapp.BuildState(testConfig(t, dbURL, dist, extra...))
	if err != nil {
		t.Fatal(err)
	}
	return serveState(t, state, dist)
}

func closedPort(t testing.TB) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

type reply struct {
	status  int
	headers http.Header
	body    []byte
}

func (r reply) text() string { return string(r.body) }

func mediaType(t testing.TB, r reply) string {
	t.Helper()
	mt, _, err := mime.ParseMediaType(r.header("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type %q: %v", r.header("Content-Type"), err)
	}
	return mt
}
func (r reply) header(name string) string { return r.headers.Get(name) }
func (r reply) hasHeader(name string) bool {
	_, ok := r.headers[http.CanonicalHeaderKey(name)]
	return ok
}

func (r reply) json(t testing.TB) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("JSON body: %v: %s", err, r.body)
	}
	return v
}

var client = &http.Client{
	Transport: &http.Transport{DisableCompression: true, DisableKeepAlives: true},
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Timeout: 30 * time.Second,
}

func request(t testing.TB, addr, method, path string, body string, headers ...string) reply {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+addr+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body == "" {
		req.Body, req.ContentLength = http.NoBody, 0
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if strings.EqualFold(headers[i], "host") {
			req.Host = headers[i+1]
			continue
		}
		req.Header.Add(headers[i], headers[i+1])
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{status: resp.StatusCode, headers: resp.Header, body: b}
}

func rawStatus(t testing.TB, addr, path string) int {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nhost: x\r\nconnection: close\r\n\r\n", path)
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var proto string
	var status int
	_, _ = fmt.Sscanf(line, "%s %d", &proto, &status)
	return status
}

type testApp struct {
	t     testing.TB
	addr  string
	state *service.State
	db    *testsupport.TestDB
}

func startApp(t testing.TB, extra ...string) *testApp {
	t.Helper()
	tdb := testsupport.NewTestDB(t)
	migrate(t, tdb)
	return appOver(t, tdb, extra...)
}

func migrate(t testing.TB, tdb *testsupport.TestDB) {
	t.Helper()
	if _, err := postgres.Embedded().Migrate(context.Background(), tdb.Pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func appOver(t testing.TB, tdb *testsupport.TestDB, extra ...string) *testApp {
	t.Helper()
	dist := testsupport.MissingDist(t).Dir
	state, err := svcapp.BuildState(testConfig(t, tdb.URL, dist, extra...))
	if err != nil {
		t.Fatal(err)
	}
	return &testApp{t: t, addr: serveState(t, state, dist), state: state, db: tdb}
}

func (a *testApp) user(email string, superadmin bool) access.User {
	a.t.Helper()
	name, _, _ := strings.Cut(email, "@")
	u, err := service.CreateUserWith(context.Background(), a.state.Users, a.state.Hasher,
		access.CreateUser{Email: email, DisplayName: name, Password: password, IsSuperadmin: superadmin})
	if err != nil {
		a.t.Fatalf("create user: %v", err)
	}
	return u
}

func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (a *testApp) login(email, pw string) reply {
	return a.send("POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": pw})
}

func (a *testApp) session(email, pw string) string {
	a.t.Helper()
	r := a.login(email, pw)
	if r.status != 200 {
		a.t.Fatalf("login %s: %d %s", email, r.status, r.body)
	}
	return cookieOf(a.t, r)
}

func (a *testApp) admin() string {
	a.user("root@example.com", true)
	return a.session("root@example.com", password)
}

func (a *testApp) get(path, cookie string) reply {
	return a.call("GET", path, cookie)
}

func (a *testApp) send(method, path, cookie string, body any) reply {
	a.t.Helper()
	headers := []string{"content-type", "application/json"}
	if cookie != "" {
		headers = append(headers, "cookie", cookie)
	}
	return request(a.t, a.addr, method, path, jsonText(body), headers...)
}

func (a *testApp) call(method, path, cookie string) reply {
	a.t.Helper()
	var headers []string
	if cookie != "" {
		headers = append(headers, "cookie", cookie)
	}
	return request(a.t, a.addr, method, path, "", headers...)
}

func scalar[T any](t testing.TB, tdb *testsupport.TestDB, query string, args ...any) T {
	t.Helper()
	var v T
	if err := tdb.Pool.QueryRow(context.Background(), query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func execSQL(t testing.TB, tdb *testsupport.TestDB, query string, args ...any) {
	t.Helper()
	if _, err := tdb.Pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func cookieOf(t testing.TB, r reply) string {
	t.Helper()
	set := r.header("Set-Cookie")
	if set == "" {
		t.Fatal("no Set-Cookie")
	}
	pair, _, _ := strings.Cut(set, ";")
	return pair
}

func tokenOf(cookie string) string {
	_, token, _ := strings.Cut(cookie, "=")
	return token
}

func eq[T comparable](t testing.TB, got, want T, context ...any) {
	t.Helper()
	if got != want {
		t.Fatalf("%v: got %v, want %v", fmt.Sprint(context...), got, want)
	}
}
