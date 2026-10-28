package tests

import (
	"archive/zip"
	"bytes"
	"slices"
	"strings"
	"testing"
)

func zipNames(t *testing.T, body []byte) []string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	must(t, err)
	var out []string
	for _, f := range z.File {
		out = append(out, f.Name)
	}
	slices.Sort(out)
	return out
}

func TestSkillsAndFoldersAreDisabledAndArchivedAsAFolder(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	a.user("bob@example.com", false)
	bob := a.session("bob@example.com", password)
	examples := idOf(a.skillFolder(admin, obj{"slug": "examples", "name": "Examples"}))
	offensive := idOf(a.skillFolder(admin, obj{"parent_id": examples, "slug": "offensive", "name": "Offensive"}))
	other := idOf(a.skillFolder(admin, obj{"parent_id": examples, "slug": "other", "name": "Other"}))
	ids := map[string]string{}
	for _, c := range []struct{ name, folder string }{{"nmap", offensive}, {"burp", offensive}, {"rover", examples}, {"rover", other}} {
		r := a.uploadSkill(admin, map[string]string{"SKILL.md": skillMD(c.name, ""), "notes.md": "# " + c.name}, "folder_id", c.folder, "status", "published")
		eq(t, r.status, 201, r.text())
		if _, ok := ids[c.name]; !ok {
			ids[c.name] = idOf(r.json(t))
		}
	}

	r := a.call("GET", skillsPath+"/folders/"+examples+"/archive", bob)
	eq(t, r.status, 200, r.text())
	eq(t, r.header("X-Skills-Count"), "3")
	eq(t, r.header("X-Skills-Skipped"), "1")
	eq(t, strings.Join(zipNames(t, []byte(r.body)), ","), "burp/SKILL.md,burp/notes.md,nmap/SKILL.md,nmap/notes.md,rover/SKILL.md,rover/notes.md")
	eq(t, a.get(skillPath(ids["nmap"]), admin).json(t)["downloads"], any(float64(1)))
	head := a.call("HEAD", skillsPath+"/folders/"+examples+"/archive", bob)
	eq(t, head.header("X-Skills-Count"), "3")
	eq(t, a.get(skillPath(ids["nmap"]), admin).json(t)["downloads"], any(float64(1)))

	r = a.send("PATCH", skillsPath+"/folders/"+offensive, admin, obj{"disabled": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["disabled"], any(true))
	nmap := a.get(skillPath(ids["nmap"]), bob).json(t)
	eq(t, nmap["status"], any("published"))
	eq(t, nmap["active"], any(false))
	eq(t, at(nmap, "disabled_by", "id"), any(offensive))
	eq(t, code(t, a.call("GET", skillPath(ids["nmap"])+"/archive", bob)), any("conflict.skill_disabled"))
	eq(t, code(t, a.call("GET", skillsPath+"/folders/"+offensive+"/archive", bob)), any("conflict.skill_disabled"))
	eq(t, a.call("GET", skillsPath+"/folders/"+examples+"/archive", bob).header("X-Skills-Count"), "1")
	eq(t, sortedCSV(fieldsOf(t, a.get(skillsPath+"?folder_id="+examples+"&recursive=true&active=true", bob).json(t), "name")), "rover,rover")
	eq(t, len(list(t, a.get(skillsPath+"?folder_id="+examples+"&recursive=true", bob).json(t), "items")), 4)

	eq(t, a.send("PATCH", skillPath(ids["burp"]), admin, obj{"status": "disabled"}).status, 200)
	eq(t, a.send("PATCH", skillsPath+"/folders/"+offensive, admin, obj{"disabled": false}).status, 200)
	eq(t, a.get(skillPath(ids["nmap"]), bob).json(t)["active"], any(true))
	burp := a.get(skillPath(ids["burp"]), bob).json(t)
	eq(t, burp["active"], any(false))
	eq(t, burp["disabled_by"], nil)
	eq(t, a.call("GET", skillsPath+"/folders/"+offensive+"/archive", bob).header("X-Skills-Count"), "1")

	empty := idOf(a.skillFolder(admin, obj{"parent_id": examples, "slug": "empty", "name": "Empty"}))
	eq(t, code(t, a.call("GET", skillsPath+"/folders/"+empty+"/archive", bob)), any("conflict.skill_folder_empty"))
}
