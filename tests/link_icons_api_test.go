package tests

import (
	"bytes"
	"mime/multipart"
	"strings"
	"testing"
)

func iconsPath(node string) string { return nodePath(node) + "/link-icons" }

func multipartFile(t *testing.T, name string, data []byte) (string, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.String(), w.FormDataContentType()
}

func (a *testApp) upload(cookie, node, name string, data []byte, extra ...string) reply {
	a.t.Helper()
	body, ct := multipartFile(a.t.(*testing.T), name, data)
	headers := append([]string{"content-type", ct}, extra...)
	if cookie != "" {
		headers = append(headers, "cookie", cookie)
	}
	return request(a.t, a.addr, "POST", iconsPath(node), body, headers...)
}

func TestLinkIconsAreUploadedAndServed(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	org := a.nodeID(admin, "organization", "", "acme")

	r := a.upload(admin, org, "logo.png", tinyPNG)
	eq(t, r.status, 201, r.text())
	id := r.json(t)["id"].(string)
	eq(t, strings.HasSuffix(id, ".png") && len(id) == 68, true, id)
	eq(t, r.json(t)["url"], any("/api/v1/link-icons/"+id))
	eq(t, a.upload(admin, org, "again.png", tinyPNG).json(t)["id"], any(id), "same content, same id")

	got := a.get("/api/v1/link-icons/"+id, admin)
	eq(t, got.status, 200)
	eq(t, got.headers.Get("Content-Type"), "image/png")
	eq(t, got.headers.Get("X-Content-Type-Options"), "nosniff")
	contains(t, got.headers.Get("Content-Security-Policy"), "sandbox")
	contains(t, got.headers.Get("Cache-Control"), "immutable")
	eq(t, bytes.Equal(got.body, tinyPNG), true)

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	r = a.upload(admin, org, "x.svg", svg)
	eq(t, r.status, 201, r.text())
	got = a.get(r.json(t)["url"].(string), admin)
	eq(t, got.headers.Get("Content-Type"), "image/svg+xml")
	contains(t, got.headers.Get("Content-Security-Policy"), "sandbox")

	r = a.upload(admin, org, "icon.png", []byte("<html><body>hi</body></html>"))
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_icon_format"))
	r = a.upload(admin, org, "big.png", append(append([]byte{}, tinyPNG...), make([]byte, 65536)...))
	eq(t, r.status, 413)
	eq(t, code(t, r), any("validation.icon_too_large"))
	r = request(t, a.addr, "POST", iconsPath(org), `{"file":"x"}`, "content-type", "application/json", "cookie", admin)
	eq(t, code(t, r), any("validation.invalid_body"))

	eq(t, a.get("/api/v1/link-icons/..%2F..%2Fetc%2Fpasswd", admin).status, 404)
	eq(t, a.get("/api/v1/link-icons/"+strings.Repeat("b", 64)+".png", admin).status, 404)
	eq(t, a.get("/api/v1/link-icons/"+id, "").status, 401)

	a.user("viewer@example.com", false)
	viewer := a.session("viewer@example.com", password)
	a.grant(admin, org, "user", "viewer@example.com", "viewer")
	eq(t, a.upload(viewer, org, "logo.png", tinyPNG).status, 403)
	eq(t, a.get("/api/v1/link-icons/"+id, viewer).status, 200)
	eq(t, a.bearer("GET", "/api/v1/link-icons/"+id, a.pat(viewer, "read"), nil).status, 200)
}

func TestLinkIconUploadsCanBeTurnedOff(t *testing.T) {
	t.Parallel()
	a := startApp(t, "UPLOADS_DIR", "off")
	admin := a.admin()
	org := a.nodeID(admin, "organization", "", "acme")
	r := a.upload(admin, org, "logo.png", tinyPNG)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.uploads_disabled"))
}
