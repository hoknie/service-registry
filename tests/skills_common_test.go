package tests

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/textproto"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const skillsPath = "/api/v1/skills"

func skillPath(id string) string { return skillsPath + "/" + id }

func skillMD(name, metadata string) string {
	out := "---\nname: " + name + "\ndescription: Helps with " + name + "\n"
	if metadata != "" {
		out += "metadata:\n" + metadata
	}
	return out + "---\n# " + name + "\n\nSee [reference](reference.md).\n"
}

func skillZip(t testing.TB, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, files[n]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type form struct {
	archive []byte
	files   map[string]string
	fields  map[string]string
}

func (f form) encode(t testing.TB) (string, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	names := make([]string, 0, len(f.fields))
	for k := range f.fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if err := w.WriteField(k, f.fields[k]); err != nil {
			t.Fatal(err)
		}
	}
	if f.archive != nil {
		part, err := w.CreateFormFile("archive", "skill.zip")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(f.archive)
	}
	paths := make([]string, 0, len(f.files))
	for p := range f.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="`+p+`"`)
		h.Set("Content-Type", "application/octet-stream")
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(part, f.files[p])
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.String(), w.FormDataContentType()
}

func (a *testApp) sendForm(method, path, cookie string, f form, extra ...string) reply {
	a.t.Helper()
	body, ct := f.encode(a.t)
	headers := append([]string{"content-type", ct}, extra...)
	if cookie != "" {
		headers = append(headers, "cookie", cookie)
	}
	return request(a.t, a.addr, method, path, body, headers...)
}

func (a *testApp) uploadSkill(cookie string, files map[string]string, fields ...string) reply {
	a.t.Helper()
	f := form{archive: skillZip(a.t, files), fields: map[string]string{}}
	for i := 0; i+1 < len(fields); i += 2 {
		f.fields[fields[i]] = fields[i+1]
	}
	return a.sendForm("POST", skillsPath, cookie, f)
}

func (a *testApp) skill(cookie, name string, fields ...string) obj {
	a.t.Helper()
	r := a.uploadSkill(cookie, map[string]string{name + "/SKILL.md": skillMD(name, ""), name + "/reference.md": "# Reference\n"}, fields...)
	if r.status != 201 {
		a.t.Fatalf("upload %s: %d %s", name, r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) skillFolder(cookie string, body obj) obj {
	a.t.Helper()
	r := a.send("POST", skillsPath+"/folders", cookie, body)
	if r.status != 201 {
		a.t.Fatalf("folder: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}

func fieldsOf(t testing.TB, v any, field string) string {
	t.Helper()
	var out []string
	for _, item := range list(t, v, "items") {
		out = append(out, item.(map[string]any)[field].(string))
	}
	return strings.Join(out, ",")
}

func parseUUID(t testing.TB, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
