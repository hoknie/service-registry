package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	GzBody = []byte("\x1f\x8bfake-gzip")
	BrBody = []byte("fake-brotli")
)

type Dist struct {
	Dir string
}

func EmptyDist(t testing.TB) *Dist { return &Dist{Dir: t.TempDir()} }

func MissingDist(t testing.TB) *Dist { return &Dist{Dir: filepath.Join(t.TempDir(), "nope")} }

func ExportDist(t testing.TB) *Dist {
	d := EmptyDist(t)
	d.WriteExport(t)
	return d
}

func (d *Dist) WriteExport(t testing.TB) {
	t.Helper()
	d.Write(t, "404.html", "<!DOCTYPE html><html><body>root 404</body></html>")
	for _, l := range []string{"en", "es", "ru", "zh"} {
		d.Write(t, l+".html", `<!DOCTYPE html><html lang="`+l+`"><body>`+l+` home</body></html>`)
		d.Write(t, l+".txt", l+" home data")
		d.Write(t, l+"/login.html", `<!DOCTYPE html><html lang="`+l+`"><body>`+l+` login</body></html>`)
		d.Write(t, l+"/404.html", `<!DOCTYPE html><html lang="`+l+`"><body>`+l+` not found</body></html>`)
	}
	d.Write(t, "ru/catalog.html", `<!DOCTYPE html><html lang="ru"><body>ru catalog</body></html>`)
	d.Write(t, "ru/catalog.txt", "ru catalog data")
	d.Write(t, "ru/catalog/__next._tree.txt", "ru catalog tree")
	d.Write(t, "_next/static/chunks/app.js", "console.log('app');")
	d.WriteBytes(t, "_next/static/chunks/app.js.gz", GzBody)
	d.Write(t, "_next/static/chunks/app.css", "body{}")
	d.WriteBytes(t, "_next/static/chunks/app.css.br", BrBody)
	d.WriteBytes(t, "_next/static/chunks/app.css.gz", GzBody)
	d.Write(t, "favicon.ico", "ico")
	d.Write(t, "data.weird", "bytes")
	d.Write(t, "api.html", "<html>api file</html>")
	d.Write(t, "api/nope.html", "<html>api nope file</html>")
	d.Write(t, ".env", "SECRET=1")
	d.Write(t, "en/.env", "SECRET=2")
}

func (d *Dist) Write(t testing.TB, rel, content string) { d.WriteBytes(t, rel, []byte(content)) }

func (d *Dist) WriteBytes(t testing.TB, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(d.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
