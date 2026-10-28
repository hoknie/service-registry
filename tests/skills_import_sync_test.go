package tests

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"svc-registry/pkg/testing/hostingfake"
)

func cyberSkill(name, subdomain string) string {
	return "---\nname: " + name + "\ndescription: Helps with " + name + "\nsubdomain: " + subdomain + "\n---\n# " + name + "\n"
}

func (s *skillSync) folderNames(parent string) string {
	s.t.Helper()
	return fieldsOf(s.t, s.get(skillsPath+"/folders?parent_id="+parent, s.admin).json(s.t), "name")
}

func TestSkillSourceReadsAnArchiveAndGroupsByField(t *testing.T) {
	t.Parallel()
	s := startSkillSyncWith(t, "SECRETS_KEYS", secretsKey)
	acme := scalar[string](t, s.db, "SELECT id::text FROM nodes WHERE slug = 'acme'")
	f := hostingfake.Start(t, "github", hostingToken, "mukul975")
	files := map[string]string{
		"README.md":                  cyberSkill("x", "y"),
		"skills/volatility/SKILL.md": cyberSkill("volatility", "Digital Forensics"),
		"skills/volatility/ref.md":   "# ref\n",
		"skills/nmap/SKILL.md":       cyberSkill("nmap", "Network Security"),
		"skills/plain/SKILL.md":      skillMD("plain", ""),
	}
	f.Put(hostingfake.Repo{ID: 1, Path: "mukul975/cyber", Name: "cyber", Files: files})
	src := s.source(obj{"kind": "remote", "hosting": "github", "url": "https://github.com/mukul975/cyber/tree/main/skills",
		"api_url": f.APIURL(), "credentials": s.tokenOn(s.admin, acme, hostingToken), "layout": "field", "group_field": "subdomain"})
	eq(t, src["url"], any("https://github.com/mukul975/cyber"))
	eq(t, src["branch"], any("main"))
	eq(t, jsonText(src["roots"]), `["skills"]`)
	eq(t, src["root"], any("skills"))

	got := s.runSource(idOf(src))
	eq(t, at(got, "last_run", "status"), any("succeeded"), jsonText(got))
	eq(t, at(got, "last_run", "created"), any(float64(3)))
	eq(t, f.Calls("archive"), 1)
	eq(t, f.Calls("blob"), 0)
	eq(t, s.folderNames(s.folder), "Digital Forensics,Network Security")
	forensics := scalar[string](t, s.db, "SELECT id::text FROM skill_folders WHERE slug = 'digital-forensics'")
	eq(t, fieldsOf(t, s.get(skillsPath+"?folder_id="+forensics, s.admin).json(t), "name"), "volatility")
	eq(t, fieldsOf(t, s.skills(), "name"), "plain")

	nmap := scalar[string](t, s.db, "SELECT id::text FROM skills WHERE name = 'nmap'")
	eq(t, s.send("PATCH", skillPath(nmap), s.admin, obj{"status": "disabled"}).status, 200)
	files["skills/nmap/SKILL.md"] = cyberSkill("nmap", "Web Security")
	delete(files, "skills/volatility/SKILL.md")
	delete(files, "skills/volatility/ref.md")
	f.Put(hostingfake.Repo{ID: 1, Path: "mukul975/cyber", Name: "cyber", Files: files,
		Branches: []hostingfake.Branch{{Name: "main", SHA: "2222222222222222222222222222222222222222"}}})
	got = s.runSource(idOf(src))
	eq(t, at(got, "last_run", "removed"), any(float64(1)), jsonText(got))
	eq(t, s.get(skillPath(nmap), s.admin).json(t)["status"], any("disabled"))
	eq(t, s.folderNames(s.folder), "Web Security")

	f.Put(hostingfake.Repo{ID: 1, Path: "mukul975/cyber", Name: "cyber", Files: files, NoArchive: true,
		Branches: []hostingfake.Branch{{Name: "main", SHA: "3333333333333333333333333333333333333333"}}})
	got = s.runSource(idOf(src))
	eq(t, at(got, "last_run", "status"), any("succeeded"), jsonText(got))
	eq(t, f.Calls("blob") > 0, true, "falls back to reading files")
}

func TestSkillSourceRootsAndTreeLayout(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	dir := filepath.Join(s.root, "repo")
	writeFile(t, filepath.Join(dir, "skills/lang/python-pro/SKILL.md"), skillMD("python-pro", ""))
	writeFile(t, filepath.Join(dir, "skills/go/SKILL.md"), skillMD("go", ""))
	writeFile(t, filepath.Join(dir, "plugins/sec/skills/audit/SKILL.md"), skillMD("audit", ""))
	writeFile(t, filepath.Join(dir, "other/x/SKILL.md"), skillMD("other", ""))
	r := s.send("POST", skillsPath+"/folders/"+s.folder+"/sources", s.admin, obj{"kind": "local_dir", "path": dir, "root": "a", "roots": []string{"b"}})
	eq(t, code(t, r), any("validation.invalid_skill_source"))
	r = s.send("POST", skillsPath+"/folders/"+s.folder+"/sources", s.admin, obj{"kind": "local_dir", "path": dir, "layout": "field"})
	eq(t, code(t, r), any("validation.invalid_skill_source"))
	src := s.source(obj{"kind": "local_dir", "path": dir, "roots": []string{"skills", "plugins/sec/skills", "missing"}, "layout": "tree"})
	got := s.runSource(idOf(src))
	eq(t, at(got, "last_run", "created"), any(float64(3)), jsonText(got))
	eq(t, at(got, "last_run", "problems", 0, "code"), any("skills.root_missing"))
	eq(t, at(got, "last_run", "problems", 0, "path"), any("missing"))
	eq(t, sortedCSV(fieldsOf(t, s.skills(), "name")), "audit,go")
	eq(t, s.folderNames(s.folder), "lang")
}

func sortedCSV(s string) string {
	parts := strings.Split(s, ",")
	slices.Sort(parts)
	return strings.Join(parts, ",")
}
