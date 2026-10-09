package tests

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"svc-registry/internal/testsupport"
)

func TestHealthIsOKWithoutADatabaseAndWithoutUIFiles(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.MissingDist(t).Dir)
	r := request(t, app, "GET", "/api/health", "")
	eq(t, r.status, 200)
	eq(t, mediaType(t, r), "application/json")
	eq(t, r.text(), `{"status":"ok"}`)
}

func TestReadyIs503WhenTheDatabaseIsDown(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := request(t, app, "GET", "/api/ready", "")
	eq(t, r.status, 503)
	eq(t, r.text(), `{"status":"unavailable"}`)
}

func TestUnknownAPIPathIsJSON404EvenWhenAFileMatches(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	for _, path := range []string{"/api/nope", "/api/v1/projects/x", "/api", "/api/"} {
		r := request(t, app, "GET", path, "")
		eq(t, r.status, 404, path)
		eq(t, mediaType(t, r), "application/json", path)
		body := r.json(t)
		eq(t, body["code"], any("not_found"), path)
		eq(t, body["message"], any("no API route for GET "+path), path)
		eq(t, len(body), 2, "only code + message")
	}
}

func TestWrongMethodOnAPIRouteIsJSON405(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := request(t, app, "POST", "/api/health", "")
	eq(t, r.status, 405)
	eq(t, r.json(t)["code"], any("method_not_allowed"))
	eq(t, request(t, app, "HEAD", "/api/health", "").status, 200)
}

func TestPathsThatOnlyStartWithAPIBelongToTheUI(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := request(t, app, "GET", "/apiary", "")
	eq(t, r.status, 307)
	eq(t, r.header("Location"), "/en/apiary")
}

func TestOversizedJSONBodyIsAValidationError(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.MissingDist(t).Dir)
	conn, err := net.Dial("tcp", app)
	must(t, err)
	defer conn.Close()
	fmt.Fprintf(conn, "POST /api/v1/auth/login HTTP/1.1\r\nhost: x\r\ncontent-type: application/json\r\ncontent-length: %d\r\n\r\n", 3<<20)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	must(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	eq(t, resp.StatusCode, 400)
	contains(t, string(body), `"code":"validation.invalid_body"`)
}
