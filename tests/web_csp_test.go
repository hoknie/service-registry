package tests

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"svc-registry/internal/testsupport"
)

const cspHeader = "Content-Security-Policy"

func scriptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func scriptSrc(t *testing.T, r reply) string {
	t.Helper()
	values := r.headers.Values(cspHeader)
	eq(t, len(values), 1, "one Content-Security-Policy")
	for _, d := range strings.Split(values[0], ";") {
		if d = strings.TrimSpace(d); strings.HasPrefix(d, "script-src ") {
			return d
		}
	}
	t.Fatalf("no script-src in %q", values[0])
	return ""
}

func TestPagesCarryAPolicyWithTheHashesOfTheirInlineScripts(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := get(t, app, "/en/login")
	eq(t, r.status, 200)
	src := scriptSrc(t, r)
	eq(t, src, "script-src 'self' "+scriptHash(testsupport.InlineScript))
	eq(t, strings.Contains(src, "unsafe"), false)
	for _, d := range []string{"default-src 'self'", "style-src 'self' 'unsafe-inline'", "img-src 'self' data: https:", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'"} {
		eq(t, strings.Contains(r.header(cspHeader), d), true, d)
	}

	head := request(t, app, "HEAD", "/en/login", "")
	eq(t, head.header(cspHeader), r.header(cspHeader))

	br := get(t, app, "/en/login", "accept-encoding", "br")
	eq(t, br.header("Content-Encoding"), "br")
	eq(t, br.header(cspHeader), r.header(cspHeader))

	eq(t, scriptSrc(t, get(t, app, "/ru/catalog")), "script-src 'self'")
}

func TestPolicyFollowsARewrittenPage(t *testing.T) {
	t.Parallel()
	dist := testsupport.ExportDist(t)
	app := spawnApp(t, deadDB, dist.Dir)
	eq(t, strings.Contains(get(t, app, "/es/login").header(cspHeader), scriptHash(testsupport.InlineScript)), true)

	dist.Write(t, "es/login.html", `<!DOCTYPE html><html lang="es"><head><script>window.v=2</script></head><body>es login</body></html>`)
	later := time.Now().Add(time.Minute)
	must(t, os.Chtimes(filepath.Join(dist.Dir, "es/login.html"), later, later))
	src := scriptSrc(t, get(t, app, "/es/login"))
	eq(t, src, "script-src 'self' "+scriptHash("window.v=2"))
}

func TestNotFoundDocsAndUnavailablePagesCarryThePolicy(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	nf := get(t, app, "/en/nothing-here")
	eq(t, nf.status, 404)
	eq(t, scriptSrc(t, nf), "script-src 'self'")
	eq(t, scriptSrc(t, get(t, app, "/doc/a/b")), "script-src 'self'")

	missing := spawnApp(t, deadDB, testsupport.MissingDist(t).Dir)
	r := get(t, missing, "/ru/login")
	eq(t, r.status, 503)
	eq(t, scriptSrc(t, r), "script-src 'self'")
}

func TestAssetsAndTheAPIHaveNoPagePolicy(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	for _, p := range []string{"/_next/static/chunks/app.js", "/ru/catalog.txt", "/favicon.ico", "/api/health"} {
		r := get(t, app, p)
		eq(t, r.status, 200, p)
		eq(t, r.hasHeader(cspHeader), false, p)
	}
}
