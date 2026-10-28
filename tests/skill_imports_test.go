package tests

import (
	"path/filepath"
	"testing"
)

func TestSkillImportsCreateAFolderWithASource(t *testing.T) {
	t.Parallel()
	s := startSkillSyncWith(t, "SECRETS_KEYS", secretsKey)
	acme := scalar[string](t, s.db, "SELECT id::text FROM nodes WHERE slug = 'acme'")
	eq(t, s.get(skillsPath+"/defaults", s.admin).json(t)["sync_interval_secs"], any(float64(3600)))
	r := s.send("POST", skillsPath+"/imports", s.admin, obj{"parent_id": nil, "organization_id": nil, "kind": "remote", "hosting": "github",
		"url": "https://github.com/apollographql/skills/tree/main/skills"})
	eq(t, r.status, 201, r.text())
	got := r.json(t)
	eq(t, at(got, "folder", "slug"), any("skills"))
	eq(t, at(got, "folder", "organization_id"), nil)
	eq(t, at(got, "source", "branch"), any("main"))
	eq(t, jsonText(at(got, "source", "roots")), `["skills"]`)
	eq(t, at(got, "source", "state"), any("queued"))

	folders := s.count("skill_folders")
	r = s.send("POST", skillsPath+"/imports", s.admin, obj{"organization_id": nil, "kind": "remote", "hosting": "gitlab",
		"url": "https://gitlab.example.com/a/b", "credentials": obj{"secret_id": refSecret(t, s.db, acme)}})
	eq(t, code(t, r), any("validation.secret_not_available"))
	eq(t, s.count("skill_folders"), folders)

	dir := filepath.Join(s.root, "tree")
	writeFile(t, filepath.Join(dir, "skills/lang/python-pro/SKILL.md"), skillMD("python-pro", ""))
	r = s.send("POST", skillsPath+"/imports", s.admin, obj{"parent_id": s.folder, "kind": "local_dir", "path": dir, "roots": []string{"skills"},
		"layout": "tree", "name": "Tree skills"})
	eq(t, r.status, 201, r.text())
	folder := idOf(at(r.json(t), "folder").(map[string]any))
	eq(t, at(r.json(t), "folder", "slug"), any("tree-skills"))
	s.runSource(idOf(at(r.json(t), "source").(map[string]any)))
	lang := scalar[string](t, s.db, "SELECT id::text FROM skill_folders WHERE slug = 'lang'")
	f := s.get(skillsPath+"/folders/"+lang, s.admin).json(t)
	eq(t, f["managed"], any(true))
	eq(t, f["parent_id"], any(folder))
	eq(t, code(t, s.send("PATCH", skillsPath+"/folders/"+lang, s.admin, obj{"name": "Languages"})), any("conflict.skill_managed_by_source"))
	eq(t, code(t, s.call("DELETE", skillsPath+"/folders/"+lang, s.admin)), any("conflict.skill_managed_by_source"))
	r = s.send("PATCH", skillsPath+"/folders/"+lang, s.admin, obj{"description": "Programming languages", "disabled": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["disabled"], any(true))
}
