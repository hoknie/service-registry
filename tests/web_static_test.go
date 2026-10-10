package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"svc-registry/internal/presentation/http/webui"
	"svc-registry/internal/testsupport"
)

func get(t *testing.T, app, path string, headers ...string) reply {
	t.Helper()
	return request(t, app, "GET", path, "", headers...)
}

func TestPageWithoutExtensionIsServedFromItsHTMLFile(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := get(t, app, "/ru/catalog?node=1")
	eq(t, r.status, 200)
	eq(t, r.header("Content-Type"), "text/html; charset=utf-8")
	eq(t, r.header("X-Content-Type-Options"), "nosniff")
	eq(t, r.header("Cache-Control"), "no-cache")
	eq(t, strings.Contains(r.text(), "ru catalog"), true)

	r = get(t, app, "/ru")
	eq(t, r.status, 200)
	eq(t, strings.Contains(r.text(), "ru home"), true)
}

func TestNavigationDataAndAssetsHaveTheirTypes(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := get(t, app, "/ru/catalog.txt")
	eq(t, r.status, 200)
	eq(t, r.header("Content-Type"), "text/plain; charset=utf-8")
	eq(t, get(t, app, "/ru/catalog/__next._tree.txt").text(), "ru catalog tree")

	r = get(t, app, "/_next/static/chunks/app.js")
	eq(t, r.status, 200)
	eq(t, r.header("Content-Type"), "text/javascript; charset=utf-8")
	eq(t, r.header("Cache-Control"), "public, max-age=31536000, immutable")
	eq(t, r.header("Vary"), "Accept-Encoding")
	eq(t, r.text(), "console.log('app');")

	r = get(t, app, "/favicon.ico")
	eq(t, r.header("Content-Type"), "image/x-icon")
	eq(t, r.header("Cache-Control"), "no-cache")
	eq(t, get(t, app, "/data.weird").header("Content-Type"), "application/octet-stream")
}

func TestContentTypeTable(t *testing.T) {
	for name, ct := range map[string]string{
		"a.html": "text/html; charset=utf-8", "a.css": "text/css; charset=utf-8",
		"a.MJS": "text/javascript; charset=utf-8", "a.json": "application/json",
		"a.js.map": "application/json", "a.svg": "image/svg+xml", "a.png": "image/png",
		"a.jpeg": "image/jpeg", "a.webp": "image/webp", "a.woff2": "font/woff2",
		"a.webmanifest": "application/manifest+json", "a.wasm": "application/wasm",
		"noext": "application/octet-stream",
	} {
		eq(t, webui.ContentType(name), ct, name)
	}
}

func TestHeadHasTheHeadersOfGetWithoutABody(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	g := get(t, app, "/ru/catalog")
	h := request(t, app, "HEAD", "/ru/catalog", "")
	eq(t, h.status, 200)
	eq(t, len(h.body), 0)
	eq(t, h.header("Content-Type"), g.header("Content-Type"))
	eq(t, h.header("ETag"), g.header("ETag"))
	eq(t, h.header("Content-Length"), itoa(len(g.body)))
}

func TestOtherMethodsAre405(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		r := request(t, app, m, "/en/login", "x")
		eq(t, r.status, 405, m)
		eq(t, r.header("Allow"), "GET, HEAD", m)
	}
}

func TestTrailingSlashRedirectsPermanentlyKeepingTheQuery(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := get(t, app, "/ru/catalog/?node=1")
	eq(t, r.status, 308)
	eq(t, r.header("Location"), "/ru/catalog?node=1")
	r = get(t, app, "/ru/")
	eq(t, r.status, 308)
	eq(t, r.header("Location"), "/ru")
	for _, path := range []string{"//evil.example/", "//evil.example/x/", "/%5cevil.example/"} {
		eq(t, rawStatus(t, app, path), 400, path)
	}
}

func TestEtagRevalidationGives304UntilTheFileChanges(t *testing.T) {
	t.Parallel()
	dist := testsupport.ExportDist(t)
	app := spawnApp(t, deadDB, dist.Dir)
	etag := get(t, app, "/ru/catalog").header("ETag")

	r := get(t, app, "/ru/catalog", "if-none-match", etag)
	eq(t, r.status, 304)
	eq(t, len(r.body), 0)
	eq(t, r.header("ETag"), etag)
	eq(t, r.header("Cache-Control"), "no-cache")

	eq(t, get(t, app, "/ru/catalog", "if-none-match", `"other", W/`+etag).status, 304, "weak comparison")
	eq(t, get(t, app, "/ru/catalog", "if-none-match", "*").status, 304)

	dist.Write(t, "ru/catalog.html", `<!DOCTYPE html><html lang="ru"><body>ru catalog v2, longer</body></html>`)
	r = get(t, app, "/ru/catalog", "if-none-match", etag)
	eq(t, r.status, 200)
	eq(t, strings.Contains(r.text(), "v2"), true)
	eq(t, r.header("ETag") != etag, true)
}

func TestPrecompressedSiblingsFollowAcceptEncoding(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	plain := get(t, app, "/_next/static/chunks/app.js")
	eq(t, plain.hasHeader("Content-Encoding"), false)

	r := get(t, app, "/_next/static/chunks/app.js", "accept-encoding", "gzip, deflate")
	eq(t, r.header("Content-Encoding"), "gzip")
	eq(t, r.header("Content-Type"), "text/javascript; charset=utf-8")
	eq(t, bytes.Equal(r.body, testsupport.GzBody), true)
	eq(t, r.header("ETag") != plain.header("ETag"), true)

	css := "/_next/static/chunks/app.css"
	r = get(t, app, css, "accept-encoding", "gzip, br")
	eq(t, r.header("Content-Encoding"), "br")
	eq(t, bytes.Equal(r.body, testsupport.BrBody), true)
	eq(t, get(t, app, css, "accept-encoding", "br;q=0, gzip").header("Content-Encoding"), "gzip")
	r = get(t, app, css, "accept-encoding", "identity")
	eq(t, r.hasHeader("Content-Encoding"), false)
	eq(t, r.text(), "body{}")
}

func TestTraversalAndEncodedSeparatorsAre400(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	for _, path := range []string{
		"/en/../../etc/passwd",
		"/en/%2e%2e/%2e%2e/etc/passwd",
		"/en/..%2f..%2fetc%2fpasswd",
		"/en/a%2fb",
		"/en/a%5cb",
		`/en/a\b`,
		"/en/%00",
		"/en//login",
		"/en/%zz",
		"/_next/static/../../../etc/passwd",
	} {
		eq(t, rawStatus(t, app, path), 400, path)
	}
}

func TestSymlinkOutOfTheExportAndHiddenFilesAre404(t *testing.T) {
	t.Parallel()
	dist := testsupport.ExportDist(t)
	outside := testsupport.EmptyDist(t)
	outside.Write(t, "secret.txt", "top secret")
	must(t, os.Symlink(filepath.Join(outside.Dir, "secret.txt"), filepath.Join(dist.Dir, "en/leak.txt")))
	must(t, os.Symlink(filepath.Join(dist.Dir, "en/login.html"), filepath.Join(dist.Dir, "en/alias.html")))
	app := spawnApp(t, deadDB, dist.Dir)

	r := get(t, app, "/en/leak.txt")
	eq(t, r.status, 404)
	eq(t, strings.Contains(r.text(), "top secret"), false)
	for _, path := range []string{"/.env", "/en/.env", "/en/%2eenv"} {
		r := get(t, app, path)
		eq(t, r.status, 404, path)
		eq(t, strings.Contains(r.text(), "SECRET"), false, path)
	}
	r = get(t, app, "/en/alias")
	eq(t, r.status, 200)
	eq(t, strings.Contains(r.text(), "en login"), true)
	eq(t, get(t, app, "/_next/static").status, 404)
}

func TestMissingFileIsTheLocalized404Page(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	r := get(t, app, "/ru/nope")
	eq(t, r.status, 404)
	eq(t, r.header("Content-Type"), "text/html; charset=utf-8")
	eq(t, r.header("Cache-Control"), "no-cache")
	eq(t, strings.Contains(r.text(), `<html lang="ru">`), true, r.text())

	missing := "/_next/static/chunks/missing.js"
	r = get(t, app, missing, "accept-language", "zh-CN")
	eq(t, r.status, 404)
	eq(t, strings.Contains(r.text(), "zh not found"), true)
	r = get(t, app, missing, "accept-language", "zh-CN", "cookie", "NEXT_LOCALE=es")
	eq(t, strings.Contains(r.text(), "es not found"), true)

	r = get(t, app, "/ru/404")
	eq(t, r.status, 404)
	eq(t, strings.Contains(r.text(), "ru not found"), true)
}

func TestMissingLocale404FallsBackToTheRootPage(t *testing.T) {
	t.Parallel()
	dist := testsupport.ExportDist(t)
	must(t, os.Remove(filepath.Join(dist.Dir, "es/404.html")))
	app := spawnApp(t, deadDB, dist.Dir)
	r := get(t, app, "/es/nope")
	eq(t, r.status, 404)
	eq(t, strings.Contains(r.text(), "root 404"), true)
}

func TestPathsWithoutALocaleRedirectByCookieThenAcceptLanguage(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	for _, c := range []struct {
		path     string
		headers  []string
		location string
	}{
		{"/", []string{"accept-language", "ru-RU,ru;q=0.9"}, "/ru"},
		{"/login", []string{"cookie", "NEXT_LOCALE=es", "accept-language", "zh-CN"}, "/es/login"},
		{"/", []string{"accept-language", "fr"}, "/en"},
		{"/", nil, "/en"},
		{"/catalog?node=42", nil, "/en/catalog?node=42"},
		{"/fr/login", []string{"cookie", "theme=dark; NEXT_LOCALE=xx"}, "/en/fr/login"},
	} {
		r := get(t, app, c.path, c.headers...)
		eq(t, r.status, 307, c.path)
		eq(t, r.header("Location"), c.location, c.path)
	}
	eq(t, get(t, app, "/favicon.ico").status, 200)
	eq(t, get(t, app, "/_next/static/chunks/app.js").status, 200)
	r := get(t, app, "/en/fr/login")
	eq(t, r.status, 404)
	eq(t, strings.Contains(r.text(), "en not found"), true)
}

func TestOpeningALocalePathRemembersTheLocale(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.ExportDist(t).Dir)
	cookie := get(t, app, "/zh/login").header("Set-Cookie")
	for _, part := range []string{"Path=/", "Max-Age=31536000", "SameSite=Lax"} {
		eq(t, strings.Contains(cookie, part), true, cookie)
	}
	eq(t, strings.HasPrefix(cookie, "NEXT_LOCALE=zh"), true, cookie)

	r := get(t, app, "/ru/catalog.txt", "cookie", "NEXT_LOCALE=en")
	eq(t, strings.HasPrefix(r.header("Set-Cookie"), "NEXT_LOCALE=ru"), true)
	eq(t, strings.HasPrefix(get(t, app, "/ru.txt").header("Set-Cookie"), "NEXT_LOCALE=ru"), true)

	eq(t, get(t, app, "/zh/login", "cookie", "NEXT_LOCALE=zh").hasHeader("Set-Cookie"), false)
	eq(t, get(t, app, "/_next/static/chunks/app.js").hasHeader("Set-Cookie"), false)
}

func TestLocaleNegotiation(t *testing.T) {
	eq(t, webui.PickLocale("", false), "en")
	for header, want := range map[string]string{
		"zh-CN,zh;q=0.9":       "zh",
		"de,es;q=0.5,ru;q=0.7": "ru",
		"fr":                   "en",
		"es;q=0,ru;q=0.1":      "ru",
		"ES-mx":                "es",
	} {
		eq(t, webui.PickLocale(header, true), want, header)
	}
}

func TestMissingExportIsALocalized503AndTheAPIKeepsWorking(t *testing.T) {
	t.Parallel()
	app := spawnApp(t, deadDB, testsupport.MissingDist(t).Dir)
	r := get(t, app, "/ru/login", "accept-language", "ru-RU,ru;q=0.9,en;q=0.8")
	eq(t, r.status, 503)
	eq(t, r.header("Content-Type"), "text/html; charset=utf-8")
	eq(t, r.header("Cache-Control"), "no-store")
	for _, s := range []string{`<html lang="ru">`, "Веб-интерфейс недоступен", "WEB_DIST_DIR"} {
		eq(t, strings.Contains(r.text(), s), true, s)
	}
	r = get(t, app, "/en/login", "accept-language", "fr-FR")
	eq(t, r.status, 503)
	eq(t, strings.Contains(r.text(), `<html lang="en">`), true)
	eq(t, get(t, app, "/api/health").status, 200)
}

func TestAnExportWithout404HTMLCountsAsMissingAndAppearingLaterIsServed(t *testing.T) {
	t.Parallel()
	dist := testsupport.EmptyDist(t)
	dist.Write(t, "en.html", `<html lang="en">half</html>`)
	app := spawnApp(t, deadDB, dist.Dir)
	eq(t, get(t, app, "/en").status, 503)
	dist.WriteExport(t)
	r := get(t, app, "/en")
	eq(t, r.status, 200)
	eq(t, strings.Contains(r.text(), "en home"), true)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
