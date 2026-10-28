package hostingfake

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
)

func (f *Fake) archive(w http.ResponseWriter, x *Repo) {
	f.calls["archive"]++
	if x == nil || x.NoArchive {
		notFound(w)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	prefix := x.Name + "-0123abc/"
	_ = tw.WriteHeader(&tar.Header{Name: prefix, Typeflag: tar.TypeDir, Mode: 0o755})
	for _, p := range x.tree() {
		body := x.Files[p]
		_ = tw.WriteHeader(&tar.Header{Name: prefix + p, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.Close()
	_ = gz.Close()
}
