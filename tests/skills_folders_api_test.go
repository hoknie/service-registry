package tests

import (
	"strings"
	"testing"
)

func TestSkillFoldersInheritAndMoveScopes(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	backend := a.nodeID(admin, "folder", acme, "backend")
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ed := a.session("ed@example.com", password)

	goFolder := a.skillFolder(ed, obj{"organization_id": acme, "slug": "go", "name": "Go"})
	eq(t, goFolder["organization_id"], any(acme))
	eq(t, at(goFolder, "organization", "slug"), any("acme"))
	testing := a.skillFolder(ed, obj{"parent_id": idOf(goFolder), "organization_id": nil, "slug": "testing", "name": "Testing"})
	eq(t, testing["organization_id"], any(acme))
	eq(t, at(testing, "ancestors", 0, "slug"), any("go"))

	r := a.send("POST", skillsPath+"/folders", admin, obj{"organization_id": backend, "slug": "x", "name": "X"})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_skill_scope"))
	r = a.send("POST", skillsPath+"/folders", ed, obj{"slug": "global", "name": "Global"})
	eq(t, r.status, 403, "an editor of acme cannot create global folders")
	eq(t, a.send("POST", skillsPath+"/folders", ed, obj{"organization_id": acme, "slug": "go", "name": "Again"}).status, 409)

	shared := a.skillFolder(admin, obj{"slug": "shared", "name": "Shared"})
	s := a.skill(admin, "go-testing", "folder_id", idOf(shared), "status", "published")
	eq(t, s["organization_id"], nil)
	r = a.send("PATCH", skillsPath+"/folders/"+idOf(shared), admin, obj{"parent_id": idOf(goFolder)})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["organization_id"], any(acme))
	moved := a.get(skillPath(idOf(s)), admin).json(t)
	eq(t, moved["organization_id"], any(acme))
	eq(t, moved["path"], any("go/shared/go-testing"))

	r = a.send("PATCH", skillsPath+"/folders/"+idOf(goFolder), admin, obj{"parent_id": idOf(testing)})
	eq(t, code(t, r), any("conflict.skill_folder_cycle"))
	eq(t, code(t, a.call("DELETE", skillsPath+"/folders/"+idOf(goFolder), admin)), any("conflict.skill_folder_not_empty"))
	eq(t, a.call("DELETE", skillsPath+"/folders/"+idOf(testing), ed).status, 204)

	a.skill(admin, "top-level", "status", "published")
	eq(t, fieldsOf(t, a.get(skillsPath+"?folder_id=root", admin).json(t), "name"), "top-level")
	eq(t, len(list(t, a.get(skillsPath, admin).json(t), "items")), 2)
	folders := a.get(skillsPath+"/folders?parent_id="+idOf(goFolder), ed).json(t)
	eq(t, fieldsOf(t, folders, "slug"), "shared")
	eq(t, at(folders, "items", 0, "skills"), any(float64(1)))
	eq(t, at(folders, "items", 0, "folders"), any(float64(0)))
	top := a.get(skillsPath+"/folders", ed).json(t)
	eq(t, fieldsOf(t, top, "slug"), "go")
	eq(t, at(top, "items", 0, "folders"), any(float64(1)))

	gone := a.nodeID(admin, "organization", "", "gone")
	g := a.skillFolder(admin, obj{"organization_id": gone, "slug": "g", "name": "G"})
	a.skill(admin, "bye", "folder_id", idOf(g))
	before := a.count("skill_folders")
	eq(t, a.call("DELETE", nodePath(gone), admin).status, 204)
	eq(t, a.count("skill_folders"), before-1)
	eq(t, a.count("skills"), int64(2))
}

func TestSkillFolderDepthIsLimited(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	parent := ""
	for i := range 8 {
		body := obj{"slug": "f" + string(rune('a'+i)), "name": "F"}
		if parent != "" {
			body["parent_id"] = parent
		}
		parent = idOf(a.skillFolder(admin, body))
	}
	r := a.send("POST", skillsPath+"/folders", admin, obj{"parent_id": parent, "slug": "deep", "name": "Deep"})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.skill_folder_too_deep"))
}

func TestSkillSpaceDescriptionOfAnOrganization(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	nested := a.nodeID(admin, "folder", acme, "backend")
	a.user("bob@example.com", false)
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	bob, ed := a.session("bob@example.com", password), a.session("ed@example.com", password)
	path := skillsPath + "/organizations/" + acme

	got := a.get(path, bob).json(t)
	eq(t, got["description"], any(""))
	eq(t, got["can_write"], any(false))
	eq(t, at(got, "organization", "slug"), any("acme"))

	r := a.send("PUT", path, ed, obj{"description": "  Skills of the **acme** teams.  "})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["description"], any("Skills of the **acme** teams."))
	eq(t, a.get(path, bob).json(t)["description"], any("Skills of the **acme** teams."))

	eq(t, a.send("PUT", path, bob, obj{"description": "x"}).status, 403)
	r = a.send("PUT", path, ed, obj{"description": strings.Repeat("a", 2001)})
	eq(t, code(t, r), any("validation.invalid_skill_space"))
	eq(t, a.get(skillsPath+"/organizations/"+nested, admin).status, 404)
	a.user("eve@example.com", false)
	eq(t, a.get(path, a.session("eve@example.com", password)).status, 404)
}
