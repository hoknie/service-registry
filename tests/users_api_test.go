package tests

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestEveryUsersRouteNeedsASessionAndSuperadmin(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	ann := app.user("ann@example.com", false)
	plain := app.session("ann@example.com", password)
	id := ann.ID.String()
	for _, rt := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/users", nil},
		{"POST", "/api/v1/users", obj{"email": "x@example.com", "display_name": "X", "password": "long enough pw"}},
		{"GET", "/api/v1/users/" + id, nil},
		{"PATCH", "/api/v1/users/" + id, obj{"display_name": "Y"}},
		{"POST", "/api/v1/users/" + id + "/password", obj{"password": "long enough pw"}},
	} {
		r := app.send(rt.method, rt.path, "", rt.body)
		eq(t, r.status, 401, rt.method, " ", rt.path)
		eq(t, r.json(t)["code"], any("auth.unauthenticated"))
		r = app.send(rt.method, rt.path, plain, rt.body)
		eq(t, r.status, 403, rt.method, " ", rt.path)
		eq(t, r.json(t)["code"], any("auth.forbidden"))
	}
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM users"), int64(1), "nothing was created")
}

func TestSuperadminCreatesAUserWhoCanSignIn(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	r := app.send("POST", "/api/v1/users", admin, obj{"email": "Bob@Example.com", "display_name": " Bob ", "password": "long enough pw"})
	eq(t, r.status, 201, r.text())
	body := r.json(t)
	eq(t, body["email"], any("bob@example.com"))
	eq(t, body["display_name"], any("Bob"))
	eq(t, body["status"], any("active"))
	eq(t, body["is_superadmin"], any(false))
	id := uuid.MustParse(body["id"].(string))
	eq(t, id.Version(), uuid.Version(7))
	hash := scalar[string](t, app.db, "SELECT password_hash FROM users WHERE id = $1", id)
	eq(t, strings.HasPrefix(hash, "$argon2id$v=19$"), true, hash)
	lacks(t, hash, "long enough pw")
	eq(t, app.login("bob@example.com", "long enough pw").status, 200)
}

func TestSamePasswordGivesDifferentHashes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	app.user("bob@example.com", false)
	eq(t, scalar[int64](t, app.db, "SELECT count(DISTINCT password_hash) FROM users"), int64(2))
}

func TestCreateValidatesAndDetectsConflicts(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	app.user("ann@example.com", false)
	for _, c := range []struct {
		body   obj
		status int
		code   string
	}{
		{obj{"email": "ANN@example.com", "display_name": "A", "password": "long enough pw"}, 409, "conflict.email_taken"},
		{obj{"email": "ann.example.com", "display_name": "A", "password": "long enough pw"}, 400, "validation.invalid_email"},
		{obj{"email": "c@example.com", "display_name": "  ", "password": "long enough pw"}, 400, "validation.invalid_display_name"},
		{obj{"email": "c@example.com", "display_name": "C", "password": "short"}, 400, "validation.password_too_short"},
		{obj{"email": "c@example.com", "display_name": "C", "password": strings.Repeat("x", 257)}, 400, "validation.password_too_long"},
		{obj{"email": "c@example.com"}, 400, "validation.invalid_body"},
	} {
		r := app.send("POST", "/api/v1/users", admin, c.body)
		eq(t, r.status, c.status, jsonText(c.body))
		eq(t, r.json(t)["code"], any(c.code), jsonText(c.body))
	}
}

func TestListIsPaginatedInCreationOrder(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	app.user("ann@example.com", false)
	app.user("bob@example.com", false)

	r := app.get("/api/v1/users?limit=2&offset=2", admin)
	eq(t, r.status, 200)
	body := r.json(t)
	eq(t, body["total"], any(float64(3)))
	eq(t, body["limit"], any(float64(2)))
	eq(t, body["offset"], any(float64(2)))
	items := body["items"].([]any)
	eq(t, len(items), 1)
	eq(t, items[0].(obj)["email"], any("bob@example.com"))

	body = app.get("/api/v1/users", admin).json(t)
	eq(t, body["limit"], any(float64(50)))
	var emails []string
	for _, u := range body["items"].([]any) {
		emails = append(emails, u.(obj)["email"].(string))
	}
	eq(t, strings.Join(emails, ","), "root@example.com,ann@example.com,bob@example.com")

	for _, bad := range []string{"limit=0", "limit=201", "offset=-1", "limit=abc"} {
		r := app.get("/api/v1/users?"+bad, admin)
		eq(t, r.status, 400, bad)
		eq(t, r.json(t)["code"], any("validation.invalid_pagination"))
	}
	contains(t, app.get("/api/v1/users?offset=10", admin).text(), `"items":[]`)
}

func TestGetByIDOr404(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	ann := app.user("ann@example.com", false)
	r := app.get("/api/v1/users/"+ann.ID.String(), admin)
	eq(t, r.status, 200)
	eq(t, r.json(t)["email"], any("ann@example.com"))
	for _, missing := range []string{uuid.Must(uuid.NewV7()).String(), "not-a-uuid"} {
		r := app.get("/api/v1/users/"+missing, admin)
		eq(t, r.status, 404, missing)
		eq(t, r.json(t)["code"], any("not_found"))
	}
}

func TestUpdateNameStatusAndFlag(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	path := "/api/v1/users/" + bob.ID.String()

	r := app.send("PATCH", path, admin, obj{"display_name": "Robert", "is_superadmin": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["display_name"], any("Robert"))
	eq(t, r.json(t)["is_superadmin"], any(true))

	r = app.send("PATCH", path, admin, obj{"status": "disabled"})
	eq(t, r.status, 200)
	eq(t, r.json(t)["status"], any("disabled"))
	r = app.login("bob@example.com", password)
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.account_disabled"))

	r = app.send("PATCH", path, admin, obj{"status": "banned"})
	eq(t, r.status, 400)
	eq(t, r.json(t)["code"], any("validation.invalid_status"))

	r = app.send("PATCH", path, admin, obj{"display_name": nil})
	eq(t, r.status, 200)
	eq(t, r.json(t)["display_name"], any("Robert"))

	eq(t, app.send("PATCH", "/api/v1/users/"+uuid.Must(uuid.NewV7()).String(), admin, obj{"display_name": "Nobody"}).status, 404)
}

func TestTheLastActiveSuperadminCannotBeRemoved(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	me := app.get("/api/v1/auth/me", admin).json(t)
	path := "/api/v1/users/" + me["id"].(string)
	for _, change := range []obj{{"is_superadmin": false}, {"status": "disabled"}} {
		r := app.send("PATCH", path, admin, change)
		eq(t, r.status, 409, jsonText(change))
		eq(t, r.json(t)["code"], any("conflict.last_superadmin"))
	}
	eq(t, app.get("/api/v1/auth/me", admin).json(t)["is_superadmin"], any(true), "flag kept")

	app.user("second@example.com", true)
	eq(t, app.send("PATCH", path, admin, obj{"is_superadmin": false}).status, 200)
}

func TestResetPasswordReplacesItAndRevokesSessions(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	bobSession := app.session("bob@example.com", password)
	path := "/api/v1/users/" + bob.ID.String() + "/password"

	eq(t, app.send("POST", path, admin, obj{"password": "short"}).json(t)["code"], any("validation.password_too_short"))
	eq(t, app.send("POST", path, admin, obj{"password": "reset by admin"}).status, 204)
	eq(t, app.get("/api/v1/auth/me", bobSession).status, 401)
	eq(t, app.login("bob@example.com", password).status, 401)
	eq(t, app.login("bob@example.com", "reset by admin").status, 200)
	eq(t, app.send("POST", "/api/v1/users/"+uuid.Must(uuid.NewV7()).String()+"/password", admin, obj{"password": "reset by admin"}).status, 404)
}
