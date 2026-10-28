package tests

import "testing"

func (a *testApp) skillVersions(cookie, id string, versions ...string) {
	a.t.Helper()
	for i, v := range versions {
		f := form{archive: skillZip(a.t, map[string]string{"SKILL.md": skillMD("go-testing", ""), "v.md": v}), fields: map[string]string{"version": v}}
		if r := a.sendForm("PUT", skillPath(id)+"/content", cookie, f); r.status != 200 {
			a.t.Fatalf("version %d %s: %d %s", i, v, r.status, r.body)
		}
	}
}

func TestSkillVersionPolicies(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("bob@example.com", false)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	bob := a.session("bob@example.com", password)
	id := idOf(a.skill(admin, "go-testing", "organization_id", acme, "status", "published", "version", "1.3.0"))
	a.skillVersions(admin, id, "2.0.0", "1.4.0-rc.1")

	r := a.send("PUT", skillsPath+"/organizations/"+acme+"/version-policy", admin, obj{"range": "^1"})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "effective", "range"), any("^1"))
	eq(t, at(r.json(t), "from", "range"), any("organization"))
	eq(t, at(a.get(skillPath(id), bob).json(t), "version", "version"), any("1.3.0"))
	eq(t, fieldsOf(t, a.get(skillPath(id)+"/versions", bob).json(t), "version"), "1.3.0")
	all := a.get(skillPath(id)+"/versions", admin).json(t)
	eq(t, fieldsOf(t, all, "version"), "2.0.0,1.4.0-rc.1,1.3.0")
	eq(t, at(all, "items", 0, "available"), any(false))
	eq(t, a.get(skillPath(id)+"/archive?version=2.0.0", bob).status, 404)
	eq(t, a.get(skillPath(id)+"/archive?version=2.0.0", admin).status, 200)

	preview := a.get(skillPath(id)+"/versions?range=%5E2", admin).json(t)
	eq(t, at(preview, "items", 0, "available"), any(true))
	eq(t, at(a.get(skillPath(id)+"/version-policy", admin).json(t), "own", "range"), nil, "preview does not save")

	r = a.send("PUT", skillPath(id)+"/version-policy", admin, obj{"range": "^2"})
	eq(t, at(r.json(t), "from", "range"), any("skill"))
	eq(t, at(a.get(skillPath(id), bob).json(t), "version", "version"), any("2.0.0"))
	a.send("PUT", skillPath(id)+"/version-policy", admin, obj{"range": nil, "prerelease": true})
	eq(t, at(a.get(skillPath(id), bob).json(t), "version", "version"), any("1.4.0-rc.1"))

	r = a.send("PUT", skillPath(id)+"/version-policy", admin, obj{"range": "^^1"})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_version_range"))
	eq(t, a.send("PUT", skillsPath+"/organizations/"+acme+"/version-policy", bob, obj{"range": "*"}).status, 403)
	eq(t, a.get(skillsPath+"/organizations/"+acme+"/version-policy", bob).json(t)["can_write"], any(false))
}

func TestSkillTokensNeedSkillsWriteForGlobalSkills(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ed := a.session("ed@example.com", password)
	global := idOf(a.skill(admin, "writing", "status", "published"))
	local := idOf(a.skill(ed, "go-testing", "organization_id", acme))

	readWrite := a.pat(admin, "read", "write")
	r := a.bearer("PATCH", skillPath(global), readWrite, obj{"status": "draft"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, a.bearer("PATCH", skillPath(global), a.pat(admin, "write", "skills:write"), obj{"status": "draft"}).status, 200)

	edToken := a.pat(ed, "read", "write")
	eq(t, a.bearer("PATCH", skillPath(local), edToken, obj{"status": "published"}).status, 200)
	eq(t, a.bearer("GET", skillsPath, a.pat(ed, "mcp"), nil).status, 403)
	eq(t, a.bearer("PATCH", skillPath(local), a.pat(ed, "read"), obj{"status": "draft"}).status, 403)

	group := idOf(createGroup(t, a, admin, "Skills"))
	bob := a.user("bob@example.com", false)
	eq(t, a.call("PUT", "/api/v1/groups/"+group+"/members/"+bob.ID.String(), admin).status, 204)
	a.send("PUT", "/api/v1/groups/"+group+"/permissions", admin, obj{"permissions": obj{"skills": []string{"create", "update"}}})
	bobSession := a.session("bob@example.com", password)
	created := a.uploadSkill(bobSession, map[string]string{"SKILL.md": skillMD("bobs", "")})
	eq(t, created.status, 201, created.text())
	r = a.call("DELETE", skillPath(idOf(created.json(t))), bobSession)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
}

func TestSkillsCanBeTurnedOff(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	id := idOf(a.skill(admin, "go-testing", "status", "published"))
	eq(t, at(a.get("/api/v1/auth/me", admin).json(t), "features", "skills"), any(true))
	a.restart("SKILLS_ENABLED", "false", "SKILLS_STORAGE", "s3")
	admin = a.session("root@example.com", password)
	r := a.get(skillsPath, admin)
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, at(a.get("/api/v1/auth/me", admin).json(t), "features", "skills"), any(false))
	a.restart("SKILLS_ENABLED", "true", "SKILLS_STORAGE", "local")
	admin = a.session("root@example.com", password)
	eq(t, a.get(skillPath(id), admin).json(t)["name"], any("go-testing"))
	eq(t, a.get(skillPath(id)+"/archive", admin).status, 200)
}
