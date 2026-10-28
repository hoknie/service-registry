package tests

import (
	"strings"
	"testing"
)

func setPath(org string) string { return skillsPath + "/organizations/" + org + "/set" }

func TestOrganizationSkillSetNarrowsTokensWithNodes(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	a.user("ed@example.com", false)
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	ed := a.session("ed@example.com", password)
	a.user("vi@example.com", false)
	a.grant(admin, acme, "user", "vi@example.com", "viewer")
	vi := a.session("vi@example.com", password)
	examples := idOf(a.skillFolder(admin, obj{"slug": "examples", "name": "Examples"}))
	apollo := idOf(a.skillFolder(admin, obj{"parent_id": examples, "slug": "apollo", "name": "Apollo"}))
	cyber := idOf(a.skillFolder(admin, obj{"parent_id": examples, "slug": "cyber", "name": "Cyber"}))
	upload := func(cookie, name string, fields ...string) string {
		r := a.uploadSkill(cookie, map[string]string{"SKILL.md": skillMD(name, "")}, append(fields, "status", "published")...)
		eq(t, r.status, 201, r.text())
		return idOf(r.json(t))
	}
	upload(admin, "rover", "folder_id", apollo)
	nmap := upload(admin, "nmap", "folder_id", cyber)
	upload(ed, "own-skill", "organization_id", acme)
	agents := idOf(a.skillFolder(ed, obj{"organization_id": acme, "slug": "agents", "name": "Agents"}))

	set := a.get(setPath(acme), vi).json(t)
	eq(t, set["mode"], any("all"))
	eq(t, set["can_manage"], any(false))
	eq(t, a.send("PUT", setPath(acme), vi, obj{"mode": "selected", "folders": []string{apollo}}).status, 403)
	eq(t, code(t, a.send("PUT", setPath(acme), ed, obj{"mode": "selected", "folders": []string{agents}})), any("validation.invalid_skill_set"))
	eq(t, code(t, a.send("PUT", setPath(acme), ed, obj{"mode": "some"})), any("validation.invalid_skill_set"))

	wide := a.mcpSession(a.pat(ed, "mcp"))
	r := a.issue(ed, obj{"name": "narrow", "scopes": []string{"mcp"}, "expires_in_days": 30, "nodes": []string{acme}})
	eq(t, r.status, 201, r.text())
	narrow := a.mcpSession(r.json(t)["secret"].(string))
	out, _ := call(t, narrow, "get_skill", map[string]any{"skill": nmap})
	contains(t, out, "name: nmap")

	r = a.send("PUT", setPath(acme), ed, obj{"mode": "selected", "folders": []string{apollo}, "skills": []string{}})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "folders", 0, "path"), any("examples/apollo"))
	eq(t, at(r.json(t), "folders", 0, "skills"), any(float64(1)))

	out, isErr := call(t, narrow, "list_skills", nil)
	eq(t, isErr, false, out)
	contains(t, out, "rover")
	contains(t, out, "own-skill")
	eq(t, strings.Contains(out, "nmap"), false)
	out, isErr = call(t, narrow, "get_skill", map[string]any{"skill": nmap})
	eq(t, isErr, true)
	contains(t, out, "not_found")
	out, _ = call(t, wide, "list_skills", nil)
	contains(t, out, "nmap")

	out, isErr = call(t, narrow, "get_skill_folder", map[string]any{"folder": "examples/apollo"})
	eq(t, isErr, false, out)
	contains(t, out, "1 skills will be installed")
	contains(t, out, "/api/v1/skills/folders/"+apollo+"/archive")

	eq(t, a.call("DELETE", skillsPath+"/folders/"+cyber, admin).status, 409)
	eq(t, a.call("DELETE", skillPath(nmap), admin).status, 204)
	eq(t, a.call("DELETE", skillsPath+"/folders/"+cyber, admin).status, 204)
	r = a.send("PUT", setPath(acme), admin, obj{"mode": "selected", "skills": []string{}, "folders": []string{apollo, examples}})
	eq(t, r.status, 200, r.text())
	eq(t, a.call("DELETE", skillsPath+"/folders/"+apollo, admin).status, 409)
	eq(t, scalar[int64](t, a.db, "SELECT cardinality(folder_ids) FROM skill_organization_sets"), int64(2))
}
