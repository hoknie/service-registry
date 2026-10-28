package tests

import (
	"strings"
	"testing"
)

func TestSkillsVisibilityFollowsOrganizations(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	api := a.nodeID(admin, "project", a.nodeID(admin, "folder", acme, "backend"), "api")
	a.nodeID(admin, "organization", "", "other")
	a.user("bob@example.com", false)
	a.user("ed@example.com", false)
	a.grant(admin, api, "user", "bob@example.com", "viewer")
	a.grant(admin, acme, "user", "ed@example.com", "editor")
	bob, ed := a.session("bob@example.com", password), a.session("ed@example.com", password)

	published := a.skill(ed, "go-testing", "organization_id", acme, "status", "published")
	draft := a.skill(ed, "sql-review", "organization_id", acme)
	global := a.skill(admin, "writing", "status", "published")

	eq(t, fieldsOf(t, a.get(skillsPath+"?organization_id="+acme, bob).json(t), "name"), "go-testing")
	eq(t, fieldsOf(t, a.get(skillsPath, bob).json(t), "name"), "writing,go-testing")
	eq(t, a.get(skillPath(idOf(draft)), bob).status, 404)
	eq(t, a.get(skillPath(idOf(draft)), ed).json(t)["status"], any("draft"))
	eq(t, fieldsOf(t, a.get(skillsPath+"?scope=global", ed).json(t), "name"), "writing")

	a.user("eve@example.com", false)
	eve := a.session("eve@example.com", password)
	eq(t, a.get(skillPath(idOf(published)), eve).status, 404)
	eq(t, a.get(skillPath(idOf(global)), eve).status, 200)
	r := a.send("PATCH", skillPath(idOf(global)), eve, obj{"tags": []string{"x"}})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))

	r = a.send("PATCH", skillPath(idOf(draft)), ed, obj{"status": "published", "tags": []string{"SQL", "postgres"}})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["tags"]), `["sql","postgres"]`)
	eq(t, at(r.json(t), "version", "available"), any(true))
	eq(t, a.get(skillPath(idOf(draft)), bob).status, 200)

	eq(t, fieldsOf(t, a.get(skillsPath+"?tag=sql&tag=postgres", bob).json(t), "name"), "sql-review")
	eq(t, fieldsOf(t, a.get(skillsPath+"?q=GO-TEST", bob).json(t), "name"), "go-testing")
	eq(t, code(t, a.get(skillsPath+"?tag=Bad%20Tag", bob)), any("validation.invalid_filter"))
	tags := a.get(skillsPath+"/tags", bob).json(t)
	eq(t, fieldsOf(t, tags, "tag"), "postgres,sql")

	page := a.get(skillsPath+"?limit=2", ed).json(t)
	eq(t, len(list(t, page, "items")), 2)
	next := a.get(skillsPath+"?limit=2&cursor="+page["next_cursor"].(string), ed).json(t)
	eq(t, len(list(t, next, "items")), 1)
	eq(t, next["next_cursor"], nil)

	eq(t, a.call("DELETE", skillPath(idOf(published)), ed).status, 204)
	eq(t, a.get(skillPath(idOf(published)), ed).status, 404)
}

func TestSkillImageFromUploadOrFrontmatter(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	png := string(tinyPNG)
	r := a.uploadSkill(admin, map[string]string{"SKILL.md": skillMD("pic", "  image: assets/cover.png\n"), "assets/cover.png": png},
		"status", "published")
	eq(t, r.status, 201, r.text())
	id := idOf(r.json(t))
	eq(t, r.json(t)["image_url"], any("/api/v1/skills/"+id+"/image"))
	img := a.get(skillPath(id)+"/image", admin)
	eq(t, img.status, 200)
	eq(t, img.header("Content-Type"), "image/png")
	eq(t, img.header("Content-Security-Policy"), "sandbox")

	body, ct := multipartFile(t, "x.txt", []byte("not an image"))
	r = request(t, a.addr, "PUT", skillPath(id)+"/image", strings.ReplaceAll(body, `name="file"`, `name="image"`), "content-type", ct, "cookie", admin)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_skill_image"))
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	body, ct = multipartFile(t, "logo.svg", svg)
	r = request(t, a.addr, "PUT", skillPath(id)+"/image", strings.ReplaceAll(body, `name="file"`, `name="image"`), "content-type", ct, "cookie", admin)
	eq(t, r.status, 200, r.text())
	eq(t, a.get(skillPath(id)+"/image", admin).header("Content-Type"), "image/svg+xml")
	eq(t, a.call("DELETE", skillPath(id)+"/image", admin).status, 204)
	eq(t, a.get(skillPath(id)+"/image", admin).header("Content-Type"), "image/png")
}
