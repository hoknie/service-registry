package tests

import (
	"testing"

	"svc-registry/pkg/testing/oidcfake"
)

func editorsPath(group, user string) string {
	return "/api/v1/groups/" + group + "/contact-editors/" + user
}

func ownContactsPath(group string) string { return "/api/v1/account/groups/" + group + "/contacts" }

func contactEditor(t *testing.T, app *testApp, admin, group, email string) any {
	t.Helper()
	for _, m := range list(t, app.get("/api/v1/groups/"+group, admin).json(t), "members") {
		if m.(obj)["email"] == email {
			return m.(obj)["contact_editor"]
		}
	}
	return nil
}

func TestContactEditorsAreGrantedToMembers(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob, carol := app.user("bob@example.com", false), app.user("carol@example.com", false)
	platform := idOf(createGroup(t, app, admin, "Platform"))
	eq(t, app.call("PUT", "/api/v1/groups/"+platform+"/members/"+bob.ID.String(), admin).status, 204)
	eq(t, contactEditor(t, app, admin, platform, "bob@example.com"), any(false))

	eq(t, app.call("PUT", editorsPath(platform, bob.ID.String()), admin).status, 204)
	eq(t, app.call("PUT", editorsPath(platform, bob.ID.String()), admin).status, 204)
	eq(t, contactEditor(t, app, admin, platform, "bob@example.com"), any(true))

	r := app.call("PUT", editorsPath(platform, carol.ID.String()), admin)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.not_group_member"))
	eq(t, app.call("PUT", editorsPath(platform, "00000000-0000-7000-8000-000000000000"), admin).status, 404)
	eq(t, app.call("PUT", editorsPath("00000000-0000-7000-8000-000000000000", bob.ID.String()), admin).status, 404)
	eq(t, app.call("DELETE", editorsPath(platform, carol.ID.String()), admin).status, 204)

	bobSession := app.session("bob@example.com", password)
	r = app.call("PUT", editorsPath(platform, bob.ID.String()), bobSession)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	r = app.send("PUT", "/api/v1/groups/"+platform+"/contacts", bobSession, obj{"contacts": []obj{}, "on_call_url": nil})
	eq(t, code(t, r), any("auth.forbidden"))

	eq(t, app.call("DELETE", editorsPath(platform, bob.ID.String()), admin).status, 204)
	eq(t, contactEditor(t, app, admin, platform, "bob@example.com"), any(false))
	eq(t, app.call("PUT", editorsPath(platform, bob.ID.String()), admin).status, 204)
	eq(t, app.call("DELETE", "/api/v1/groups/"+platform+"/members/"+bob.ID.String(), admin).status, 204)
	eq(t, app.call("PUT", "/api/v1/groups/"+platform+"/members/"+bob.ID.String(), admin).status, 204)
	eq(t, contactEditor(t, app, admin, platform, "bob@example.com"), any(false))
}

func TestMembersEditContactsOfTheirGroups(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	platform := idOf(createGroup(t, app, admin, "Platform"))
	sre := idOf(createGroup(t, app, admin, "SRE"))
	other := idOf(createGroup(t, app, admin, "Other"))
	for _, g := range []string{platform, sre} {
		eq(t, app.call("PUT", "/api/v1/groups/"+g+"/members/"+bob.ID.String(), admin).status, 204)
	}
	eq(t, app.call("PUT", editorsPath(platform, bob.ID.String()), admin).status, 204)
	bobSession := app.session("bob@example.com", password)

	r := app.get("/api/v1/account/groups", bobSession)
	eq(t, r.status, 200, r.text())
	eq(t, ownerNames(t, r), "Platform,SRE")
	eq(t, at(r.json(t), "items", 0, "can_edit_contacts"), any(true))
	eq(t, at(r.json(t), "items", 1, "can_edit_contacts"), any(false))
	eq(t, jsonText(at(r.json(t), "items", 0, "contacts")), `[]`)

	body := obj{"contacts": []obj{{"kind": "chat", "value": "https://chat.example/platform", "label": "#platform"}},
		"on_call_url": "https://oncall.example/platform"}
	r = app.send("PUT", ownContactsPath(platform), bobSession, body)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["can_edit_contacts"], any(true))
	eq(t, app.get("/api/v1/groups/"+platform, admin).json(t)["on_call_url"], any("https://oncall.example/platform"))
	eq(t, at(app.get("/api/v1/account/groups", bobSession).json(t), "items", 0, "contacts", 0, "label"), any("#platform"))

	r = app.send("PUT", ownContactsPath(platform), bobSession, obj{"contacts": []obj{}, "on_call_url": "oncall"})
	eq(t, code(t, r), any("validation.invalid_on_call_url"))
	r = app.send("PUT", ownContactsPath(sre), bobSession, body)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, jsonText(app.get("/api/v1/groups/"+sre, admin).json(t)["contacts"]), `[]`)
	r = app.send("PUT", ownContactsPath(other), bobSession, body)
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, app.send("PUT", ownContactsPath(other), admin, body).status, 404)

	reader := app.pat(bobSession, "read")
	r = app.bearer("GET", "/api/v1/account/groups", reader, nil)
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "items", 0, "can_edit_contacts"), any(true))
	r = app.bearer("PUT", ownContactsPath(platform), reader, body)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	writer := app.pat(bobSession, "write")
	eq(t, app.bearer("GET", "/api/v1/account/groups", writer, nil).status, 403)
	r = app.bearer("PUT", ownContactsPath(platform), writer, body)
	eq(t, r.status, 200, r.text())
	eq(t, app.bearer("PUT", ownContactsPath(sre), writer, body).status, 403)
	acme := app.nodeID(admin, "organization", "", "acme")
	app.grant(admin, acme, "user", "bob@example.com", "editor")
	scoped := app.secretOn(bobSession, []string{acme}, "read", "write")
	eq(t, app.bearer("GET", "/api/v1/account/groups", scoped, nil).status, 200)
	r = app.bearer("PUT", ownContactsPath(platform), scoped, body)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.get("/api/v1/account/groups", "").status, 401)
}

func TestContactEditorSurvivesProviderSync(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_GROUP_MAP", "platform-team=>Platform")
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	platform := idOf(createGroup(t, app, admin, "Platform"))
	person := oidcfake.Person{Subject: "s-bob", Email: "bob@example.com", EmailVerified: true, Groups: []string{"platform-team"}}
	login := func() string {
		idp.SignIn(person)
		return cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	}
	login()
	eq(t, app.call("PUT", editorsPath(platform, bob.ID.String()), admin).status, 204)
	body := obj{"contacts": []obj{}, "on_call_url": "https://oncall.example/p"}

	person.Groups = nil
	eq(t, app.send("PUT", ownContactsPath(platform), login(), body).status, 404)
	person.Groups = []string{"platform-team"}
	eq(t, app.send("PUT", ownContactsPath(platform), login(), body).status, 200)
	eq(t, contactEditor(t, app, admin, platform, "bob@example.com"), any(true))
}
