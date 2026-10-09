package tests

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func createGroup(t *testing.T, app *testApp, admin, name string) obj {
	t.Helper()
	r := app.send("POST", "/api/v1/groups", admin, obj{"name": name})
	eq(t, r.status, 201, r.text())
	return r.json(t)
}

func TestGroupsAreSuperadminOnly(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	plain := app.session("ann@example.com", password)
	id := uuid.Must(uuid.NewV7()).String()
	for _, rt := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/groups", nil},
		{"POST", "/api/v1/groups", obj{"name": "Platform"}},
		{"GET", "/api/v1/groups/" + id, nil},
		{"PATCH", "/api/v1/groups/" + id, obj{"name": "X"}},
		{"DELETE", "/api/v1/groups/" + id, nil},
		{"PUT", "/api/v1/groups/" + id + "/members/" + id, nil},
		{"DELETE", "/api/v1/groups/" + id + "/members/" + id, nil},
	} {
		eq(t, app.send(rt.method, rt.path, "", rt.body).status, 401, rt.method, " ", rt.path)
		r := app.send(rt.method, rt.path, plain, rt.body)
		eq(t, r.status, 403, rt.method, " ", rt.path)
		eq(t, r.json(t)["code"], any("auth.forbidden"))
	}
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM groups"), int64(0))
}

func TestCreateRenameGetAndConflicts(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	g := createGroup(t, app, admin, " Platform ")
	eq(t, g["name"], any("Platform"))
	eq(t, g["member_count"], any(float64(0)))
	id := g["id"].(string)
	eq(t, uuid.MustParse(id).Version(), uuid.Version(7))

	r := app.send("POST", "/api/v1/groups", admin, obj{"name": "platform"})
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.group_name_taken"))
	eq(t, app.send("POST", "/api/v1/groups", admin, obj{"name": ""}).json(t)["code"], any("validation.invalid_group_name"))

	r = app.send("PATCH", "/api/v1/groups/"+id, admin, obj{"name": "Platform team"})
	eq(t, r.status, 200)
	eq(t, r.json(t)["name"], any("Platform team"))

	createGroup(t, app, admin, "Data")
	eq(t, app.send("PATCH", "/api/v1/groups/"+id, admin, obj{"name": "DATA"}).json(t)["code"], any("conflict.group_name_taken"))

	r = app.get("/api/v1/groups/"+id, admin)
	eq(t, r.status, 200)
	eq(t, r.json(t)["name"], any("Platform team"))
	contains(t, r.text(), `"members":[]`)

	r = app.get("/api/v1/groups", admin)
	var names []string
	for _, g := range r.json(t)["items"].([]any) {
		names = append(names, g.(obj)["name"].(string))
	}
	eq(t, strings.Join(names, ","), "Data,Platform team", "ordered by name")
	eq(t, r.json(t)["total"], any(float64(2)))

	for _, missing := range []string{uuid.Must(uuid.NewV7()).String(), "nope"} {
		eq(t, app.get("/api/v1/groups/"+missing, admin).status, 404, missing)
	}
}

func TestMembershipIsIdempotent(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	id := createGroup(t, app, admin, "Platform")["id"].(string)
	member := "/api/v1/groups/" + id + "/members/" + bob.ID.String()
	for range 2 {
		r := app.call("PUT", member, admin)
		eq(t, r.status, 204, r.text())
	}
	g := app.get("/api/v1/groups/"+id, admin).json(t)
	eq(t, g["member_count"], any(float64(1)))
	members := g["members"].([]any)
	eq(t, len(members), 1)
	eq(t, members[0].(obj)["email"], any("bob@example.com"))
	eq(t, members[0].(obj)["status"], any("active"))
	for range 2 {
		eq(t, app.call("DELETE", member, admin).status, 204)
	}
	eq(t, len(app.get("/api/v1/groups/"+id, admin).json(t)["members"].([]any)), 0)
}

func TestUnknownMemberOrGroupIs404(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	id := createGroup(t, app, admin, "Platform")["id"].(string)
	r := app.call("PUT", "/api/v1/groups/"+id+"/members/"+uuid.Must(uuid.NewV7()).String(), admin)
	eq(t, r.status, 404)
	eq(t, r.json(t)["code"], any("not_found"))
	missing := uuid.Must(uuid.NewV7()).String()
	for _, m := range []string{"PUT", "DELETE"} {
		eq(t, app.call(m, "/api/v1/groups/"+missing+"/members/"+bob.ID.String(), admin).status, 404, m)
	}
}

func TestDeleteRemovesMembershipButNotUsers(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	id := createGroup(t, app, admin, "Platform")["id"].(string)
	app.call("PUT", "/api/v1/groups/"+id+"/members/"+bob.ID.String(), admin)
	eq(t, app.call("DELETE", "/api/v1/groups/"+id, admin).status, 204)
	eq(t, app.get("/api/v1/groups/"+id, admin).status, 404)
	eq(t, app.call("DELETE", "/api/v1/groups/"+id, admin).status, 404)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM group_members"), int64(0))
	eq(t, app.get("/api/v1/users/"+bob.ID.String(), admin).status, 200, "the member still exists")
}
