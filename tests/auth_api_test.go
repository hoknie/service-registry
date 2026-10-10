package tests

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

type obj = map[string]any

func TestLoginNormalizesTheEmailAndReturnsTheUser(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	r := app.login("  Ann@Example.COM ", password)
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	eq(t, body["email"], any("ann@example.com"))
	eq(t, body["status"], any("active"))
	eq(t, body["is_superadmin"], any(false))
	for _, f := range []string{"id", "display_name", "created_at", "updated_at"} {
		_, ok := body[f]
		eq(t, ok, true, f)
	}
	_, hasHash := body["password_hash"]
	eq(t, hasHash, false)
	lacks(t, r.text(), "argon2")
	eq(t, strings.HasPrefix(r.header("Set-Cookie"), "registry_session="), true)
	created := body["created_at"].(string)
	eq(t, len(created) == 20 && strings.HasSuffix(created, "Z") && created[10:11] == "T", true, "RFC 3339 UTC: "+created)
	eq(t, strings.HasPrefix(r.text(), `{"id":"`), true, r.text())
	contains(t, r.text(), `","email":"ann@example.com","display_name":"ann","status":"active","is_superadmin":false,"is_service":false,"has_password":true,"created_at":"`)
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	wrong := app.login("ann@example.com", "not the password")
	eq(t, wrong.status, 401)
	eq(t, wrong.json(t)["code"], any("auth.invalid_credentials"))
	eq(t, wrong.hasHeader("Set-Cookie"), false)

	before := app.services.Hasher.Verifications()
	unknown := app.login("nobody@example.com", "not the password")
	eq(t, app.services.Hasher.Verifications(), before+1, "an unknown email still costs one full argon2id verification")
	eq(t, unknown.status, 401)
	eq(t, bytes.Equal(unknown.body, wrong.body), true, "identical response bodies")
	eq(t, unknown.hasHeader("Set-Cookie"), false)
}

func TestDisabledUserWithTheRightPasswordIsRefused(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	user := app.user("ann@example.com", false)
	execSQL(t, app.db, "UPDATE users SET status = 'disabled' WHERE id = $1", user.ID)
	r := app.login("ann@example.com", password)
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.account_disabled"))
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM sessions"), int64(0))
}

func TestMalformedLoginBodyIsAValidationError(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	r := request(t, app.addr, "POST", "/api/v1/auth/login", "not json", "content-type", "application/json")
	eq(t, r.status, 400)
	eq(t, r.json(t)["code"], any("validation.invalid_body"))

	r = request(t, app.addr, "POST", "/api/v1/auth/login", `{"email":"a@b","password":"x"}`, "content-type", "text/plain")
	eq(t, r.status, 400)
	body := r.json(t)
	eq(t, body["code"], any("validation.invalid_body"))
	eq(t, len(body), 2, "only code + message")

	for _, bad := range []string{`null`, `[]`, `{"email":null,"password":"x"}`, `{"email":"a@b"}`, `{"email":1,"password":"x"}`, `{"email":"a@b","password":"x"} trailing`} {
		r = request(t, app.addr, "POST", "/api/v1/auth/login", bad, "content-type", "application/json")
		eq(t, r.status, 400, bad)
		eq(t, r.json(t)["code"], any("validation.invalid_body"), bad)
	}
	r = request(t, app.addr, "POST", "/api/v1/auth/login", `{"email":"x@example.com","password":"wrong one!","extra":1}`,
		"content-type", "application/merge-patch+json; charset=utf-8")
	eq(t, r.status, 401)
}

func TestMeWithAndWithoutASession(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	r := app.get("/api/v1/auth/me", cookie)
	eq(t, r.status, 200)
	eq(t, r.json(t)["email"], any("ann@example.com"))
	r = app.get("/api/v1/auth/me", "")
	eq(t, r.status, 401)
	eq(t, r.json(t)["code"], any("auth.unauthenticated"))
	eq(t, app.get("/api/v1/auth/me", "registry_session=forged").status, 401)
}

func TestChangingThePasswordRevokesOtherSessionsOnly(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	a := app.session("ann@example.com", password)
	b := app.session("ann@example.com", password)
	r := app.send("POST", "/api/v1/auth/password", a, obj{"current_password": password, "new_password": "a brand new secret"})
	eq(t, r.status, 204, r.text())
	eq(t, app.get("/api/v1/auth/me", a).status, 200)
	eq(t, app.get("/api/v1/auth/me", b).status, 401)
	eq(t, app.login("ann@example.com", password).status, 401)
	eq(t, app.login("ann@example.com", "a brand new secret").status, 200)
}

func TestChangingThePasswordChecksTheCurrentOneAndTheRules(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	a := app.session("ann@example.com", password)
	r := app.send("POST", "/api/v1/auth/password", a, obj{"current_password": "wrong one!", "new_password": "a brand new secret"})
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.wrong_password"))
	r = app.send("POST", "/api/v1/auth/password", a, obj{"current_password": password, "new_password": "short"})
	eq(t, r.status, 400)
	eq(t, r.json(t)["code"], any("validation.password_too_short"))
	r = app.send("POST", "/api/v1/auth/password", "", obj{"current_password": password, "new_password": "a brand new secret"})
	eq(t, r.status, 401)
	eq(t, app.login("ann@example.com", password).status, 200, "password unchanged")
}

func TestFailedLoginsAreThrottledPerIPAndEmail(t *testing.T) {
	t.Parallel()
	app := startApp(t, "LOGIN_MAX_FAILURES", "3")
	app.user("ann@example.com", false)
	app.user("bob@example.com", false)
	for range 3 {
		eq(t, app.login("ann@example.com", "wrong password").status, 401)
	}
	before := app.services.Hasher.Verifications()
	r := app.login("ANN@example.com", password)
	eq(t, r.status, 429)
	eq(t, r.json(t)["code"], any("auth.rate_limited"))
	retry, err := strconv.Atoi(r.header("Retry-After"))
	must(t, err)
	eq(t, retry >= 1 && retry <= 900, true, retry)
	eq(t, app.services.Hasher.Verifications(), before, "a blocked attempt does not hash")
	eq(t, app.login("bob@example.com", password).status, 200, "another email is not affected")
}

func TestASuccessfulLoginResetsTheFailureCount(t *testing.T) {
	t.Parallel()
	app := startApp(t, "LOGIN_MAX_FAILURES", "2")
	app.user("ann@example.com", false)
	app.login("ann@example.com", "wrong password")
	eq(t, app.login("ann@example.com", password).status, 200)
	app.login("ann@example.com", "wrong password")
	eq(t, app.login("ann@example.com", password).status, 200, "one failure after the reset is below the limit")
}

func TestLoginWithAnExistingSessionReplacesIt(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	old := app.session("ann@example.com", password)
	r := app.send("POST", "/api/v1/auth/login", old, obj{"email": "ann@example.com", "password": password})
	eq(t, r.status, 200)
	fresh := cookieOf(t, r)
	eq(t, fresh != old, true)
	eq(t, app.get("/api/v1/auth/me", old).status, 401)
	eq(t, app.get("/api/v1/auth/me", fresh).status, 200)
}

func TestMeTellsHowTheRequestSignedIn(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	session := app.session("ann@example.com", password)
	me := app.get("/api/v1/auth/me", session).json(t)
	eq(t, me["login_method"], any("password"))
	eq(t, me["has_password"], any(true))
	secret := app.pat(session, "read")
	eq(t, app.bearer("GET", "/api/v1/auth/me", secret, nil).json(t)["login_method"], any("token"))
	r := app.send("POST", "/api/v1/auth/password", session, obj{"new_password": "another long password"})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_body"))
}

func TestUsersShowWhetherTheyHaveAPassword(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	ann := app.user("ann@example.com", false)
	eq(t, app.get("/api/v1/users/"+ann.ID.String(), admin).json(t)["has_password"], any(true))
	execSQL(t, app.db, "UPDATE users SET password_hash = NULL WHERE id = $1", ann.ID)
	eq(t, app.get("/api/v1/users/"+ann.ID.String(), admin).json(t)["has_password"], any(false))
	contains(t, app.get("/api/v1/users", admin).text(), `"has_password":false`)
}
