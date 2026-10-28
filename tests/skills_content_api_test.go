package tests

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSkillUploadFromAZipWithAFolder(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	r := a.uploadSkill(admin, map[string]string{
		"go-testing/SKILL.md":       skillMD("go-testing", "  title: Go testing\n  tags: [go, Testing]\n  author: Platform team\n"),
		"go-testing/scripts/run.sh": "#!/bin/sh\necho ok\n",
		"go-testing/.DS_Store":      "junk",
	}, "status", "published")
	eq(t, r.status, 201, r.text())
	s := r.json(t)
	eq(t, s["name"], any("go-testing"))
	eq(t, s["slug"], any("go-testing"))
	eq(t, s["title"], any("Go testing"))
	eq(t, jsonText(s["tags"]), `["go","testing"]`)
	eq(t, s["author"], any("Platform team"))
	eq(t, at(s, "version", "version"), any("0.1.0"))
	eq(t, at(s, "version", "files"), any(float64(2)))
	eq(t, s["path"], any("go-testing"))
	id := idOf(s)

	files := a.get(skillPath(id)+"/files", admin)
	eq(t, files.status, 200, files.text())
	eq(t, fieldsOf(t, files.json(t), "path"), "SKILL.md,scripts/run.sh")

	md := a.get(skillPath(id)+"/file?path=SKILL.md", admin).json(t)
	eq(t, md["kind"], any("markdown"))
	contains(t, md["content"].(string), "name: go-testing")
	eq(t, a.get(skillPath(id)+"/file?path=missing.md", admin).status, 404)

	raw := a.get(skillPath(id)+"/raw?path=scripts/run.sh", admin)
	eq(t, raw.status, 200)
	eq(t, raw.header("Content-Type"), "text/plain; charset=utf-8")
	eq(t, raw.header("Content-Security-Policy"), "sandbox")
	contains(t, raw.header("Content-Disposition"), "attachment")
}

func TestSkillUploadFromBrowserFolderParts(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	r := a.sendForm("POST", skillsPath, admin, form{files: map[string]string{
		"go-testing/SKILL.md": skillMD("go-testing", ""), "go-testing/docs/reference.md": "# Ref\n"}})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["status"], any("draft"))
	files := a.get(skillPath(idOf(r.json(t)))+"/files", admin).json(t)
	eq(t, fieldsOf(t, files, "path"), "SKILL.md,docs/reference.md")
	eq(t, at(files, "items", 1, "title"), any("Ref"))
}

func TestSkillUploadRefusals(t *testing.T) {
	t.Parallel()
	a := startApp(t, "SKILLS_MAX_UPLOAD_BYTES", "65536")
	admin := a.admin()
	for name, c := range map[string]struct {
		files  map[string]string
		status int
		code   string
	}{
		"escape":      {map[string]string{"SKILL.md": skillMD("x", ""), "../../etc/passwd": "x"}, 400, "validation.skill_archive_invalid"},
		"no manifest": {map[string]string{"README.md": "x"}, 400, "validation.skill_manifest_missing"},
		"bad name":    {map[string]string{"SKILL.md": "---\nname: Go Testing\ndescription: d\n---\n"}, 400, "validation.skill_manifest_invalid"},
	} {
		r := a.uploadSkill(admin, c.files)
		eq(t, r.status, c.status, name, r.text())
		eq(t, code(t, r), any(c.code), name)
	}
	r := a.uploadSkill(admin, map[string]string{"SKILL.md": "---\nname: Go Testing\ndescription: d\n---\n"})
	eq(t, at(r.json(t), "fields", 0, "path"), any("name"))
	eq(t, at(r.json(t), "fields", 0, "code"), any("invalid"))
	big := map[string]string{"SKILL.md": skillMD("big", ""), "big.bin": strings.Repeat("\xff\xfe", 70000)}
	r = a.sendForm("POST", skillsPath, admin, form{files: big})
	eq(t, r.status, 413, r.text())
	eq(t, code(t, r), any("validation.skill_too_large"))
	eq(t, a.count("skills"), int64(0))
}

func TestSkillVersionsFromUploads(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	s := a.skill(admin, "go-testing", "status", "published")
	id := idOf(s)
	put := func(files map[string]string, version string) reply {
		f := form{archive: skillZip(t, files), fields: map[string]string{}}
		if version != "" {
			f.fields["version"] = version
		}
		return a.sendForm("PUT", skillPath(id)+"/content", admin, f)
	}
	same := map[string]string{"SKILL.md": skillMD("go-testing", ""), "reference.md": "# Reference\n"}
	r := put(same, "")
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "version", "version"), any("0.1.0"), "same content, no new version")
	r = put(map[string]string{"SKILL.md": skillMD("go-testing", "  version: 1.2.0\n")}, "")
	eq(t, at(r.json(t), "version", "version"), any("1.2.0"))
	r = put(map[string]string{"SKILL.md": skillMD("go-testing", ""), "x.md": "x"}, "")
	eq(t, at(r.json(t), "version", "version"), any("1.2.1"))
	r = put(map[string]string{"SKILL.md": skillMD("go-testing", ""), "y.md": "y"}, "1.2.0")
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.skill_version_exists"))
	r = put(map[string]string{"SKILL.md": skillMD("other", "")}, "")
	eq(t, code(t, r), any("conflict.skill_name_changed"))
	eq(t, put(map[string]string{"SKILL.md": skillMD("go-testing", ""), "z.md": "z"}, "2.0.0").status, 200)
	eq(t, put(map[string]string{"SKILL.md": skillMD("go-testing", ""), "w.md": "w"}, "1.4.1").status, 200)
	versions := a.get(skillPath(id)+"/versions", admin).json(t)
	eq(t, fieldsOf(t, versions, "version"), "2.0.0,1.4.1,1.2.1,1.2.0,0.1.0")
	eq(t, at(a.get(skillPath(id), admin).json(t), "version", "version"), any("2.0.0"))

	zipped := a.get(skillPath(id)+"/archive?version=1.4.1", admin)
	eq(t, zipped.status, 200, zipped.text())
	eq(t, zipped.header("X-Skill-Version"), "1.4.1")
	contains(t, zipped.header("Content-Disposition"), "go-testing-1.4.1.zip")
	zr, err := zip.NewReader(bytes.NewReader(zipped.body), int64(len(zipped.body)))
	must(t, err)
	var paths []string
	for _, f := range zr.File {
		paths = append(paths, f.Name)
		if f.Name == "go-testing/w.md" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			eq(t, string(data), "w")
		}
	}
	eq(t, strings.Join(paths, ","), "go-testing/SKILL.md,go-testing/w.md")
	a.get(skillPath(id)+"/archive", admin)
	eq(t, a.get(skillPath(id), admin).json(t)["downloads"], any(float64(2)))
	eq(t, a.call("HEAD", skillPath(id)+"/archive", admin).status, 200)
	eq(t, a.get(skillPath(id), admin).json(t)["downloads"], any(float64(2)), "HEAD does not count")

	r = a.send("PUT", skillPath(id)+"/versions/2.0.0", admin, obj{"yanked": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["yanked"], any(true))
	eq(t, at(a.get(skillPath(id), admin).json(t), "version", "version"), any("1.4.1"))
}

func TestSkillKeepVersionsPrunesTheOldest(t *testing.T) {
	t.Parallel()
	a := startApp(t, "SKILLS_KEEP_VERSIONS", "3")
	admin := a.admin()
	id := idOf(a.skill(admin, "go-testing", "status", "published", "version", "1.0.0"))
	eq(t, a.send("PUT", skillPath(id)+"/version-policy", admin, obj{"range": "1.0.0"}).status, 200)
	for i, v := range []string{"1.1.0", "1.2.0", "1.3.0", "1.4.0"} {
		f := form{archive: skillZip(t, map[string]string{"SKILL.md": skillMD("go-testing", ""), "n.md": v}), fields: map[string]string{"version": v}}
		eq(t, a.sendForm("PUT", skillPath(id)+"/content", admin, f).status, 200, i)
	}
	eq(t, fieldsOf(t, a.get(skillPath(id)+"/versions", admin).json(t), "version"), "1.4.0,1.3.0,1.2.0,1.0.0")
}
