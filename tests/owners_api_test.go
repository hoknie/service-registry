package tests

import (
	"strings"
	"testing"
)

type ownersTree struct {
	*testApp
	admin, acme, backend, api, web string
	platform, sre, frontend        string
}

func ownersPath(id string) string { return nodePath(id) + "/owners" }

func startOwners(t *testing.T) *ownersTree {
	t.Helper()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	backend := a.nodeID(admin, "folder", acme, "backend")
	api := a.nodeID(admin, "project", backend, "api")
	web := a.nodeID(admin, "project", acme, "web")
	o := &ownersTree{testApp: a, admin: admin, acme: acme, backend: backend, api: api, web: web}
	o.platform = idOf(createGroup(t, a, admin, "Platform"))
	o.sre = idOf(createGroup(t, a, admin, "SRE"))
	o.frontend = idOf(createGroup(t, a, admin, "Frontend"))
	return o
}

func (o *ownersTree) setOwners(cookie, node string, owners ...obj) reply {
	o.t.Helper()
	if owners == nil {
		owners = []obj{}
	}
	return o.send("PUT", ownersPath(node), cookie, obj{"owners": owners})
}

func group(name string) obj { return obj{"subject_kind": "group", "subject": name} }

func person(email string) obj { return obj{"subject_kind": "user", "subject": email} }

func ownerNames(t *testing.T, r reply) string {
	t.Helper()
	var out []string
	for _, it := range list(t, r.json(t), "items") {
		out = append(out, it.(obj)["name"].(string))
	}
	return strings.Join(out, ",")
}

func TestOwnersAreAssignedAndValidated(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("ed@example.com", false)
	o.user("vi@example.com", false)
	o.grant(o.admin, o.backend, "user", "ed@example.com", "editor")
	o.grant(o.admin, o.backend, "user", "vi@example.com", "viewer")
	ed, vi := o.session("ed@example.com", password), o.session("vi@example.com", password)

	r := o.setOwners(ed, o.backend, group("platform"))
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "items", 0, "subject_kind"), any("group"))
	eq(t, at(r.json(t), "items", 0, "name"), any("Platform"))
	eq(t, at(r.json(t), "items", 0, "inherited"), any(false))
	eq(t, at(r.json(t), "items", 0, "email"), nil)
	eq(t, jsonText(at(r.json(t), "items", 0, "contacts")), `[]`)
	eq(t, r.json(t)["own"], any(true))
	eq(t, r.json(t)["from"], any(o.backend))

	r = o.setOwners(ed, o.backend, group("Platform"), group("platform"))
	eq(t, code(t, r), any("validation.invalid_owners"))
	eq(t, code(t, o.setOwners(ed, o.backend, obj{"subject_kind": "team", "subject": "x"})), any("validation.invalid_subject_kind"))
	eq(t, code(t, o.setOwners(ed, o.backend, group("Nobody"))), any("validation.unknown_subject"))
	many := make([]obj, 11)
	for i := range many {
		many[i] = group("Platform")
	}
	eq(t, code(t, o.setOwners(ed, o.backend, many...)), any("validation.invalid_owners"))
	eq(t, ownerNames(t, o.get(ownersPath(o.backend), ed)), "Platform")

	r = o.setOwners(vi, o.backend, group("SRE"))
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, o.get(ownersPath(o.backend), vi).status, 200)
	eq(t, o.get(ownersPath(o.web), vi).status, 404)
	eq(t, code(t, o.send("PUT", ownersPath(o.backend), ed, obj{})), any("validation.invalid_body"))
}

func TestOwnersAreInheritedAsAWholeSet(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("bob@example.com", false)
	r := o.get(ownersPath(o.api), o.admin)
	eq(t, r.text(), `{"items":[],"from":null,"own":false,"none":false}`)

	eq(t, o.setOwners(o.admin, o.acme, group("Platform"), group("SRE")).status, 200)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "Platform,SRE")
	eq(t, at(r.json(t), "items", 0, "inherited"), any(true))
	eq(t, at(r.json(t), "items", 0, "node_id"), any(o.acme))
	eq(t, r.json(t)["from"], any(o.acme))
	eq(t, r.json(t)["own"], any(false))

	eq(t, o.setOwners(o.admin, o.backend, person("BOB@example.com")).status, 200)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "bob")
	eq(t, at(r.json(t), "items", 0, "subject_kind"), any("user"))
	eq(t, at(r.json(t), "items", 0, "email"), any("bob@example.com"))
	eq(t, at(r.json(t), "items", 0, "status"), any("active"))
	eq(t, at(r.json(t), "items", 0, "on_call_url"), nil)
	eq(t, jsonText(at(r.json(t), "items", 0, "contacts")), `[]`)

	eq(t, o.setOwners(o.admin, o.api, group("Frontend")).status, 200)
	r = o.setOwners(o.admin, o.api)
	eq(t, r.status, 200)
	eq(t, ownerNames(t, r), "bob")
	eq(t, r.json(t)["own"], any(false))
}

func TestOwnerContactsAndVanishedSubjects(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	r := o.send("PUT", "/api/v1/groups/"+o.platform+"/contacts", o.admin, obj{
		"contacts":    []obj{{"kind": "chat", "value": "https://chat.example/platform", "label": "#platform"}},
		"on_call_url": "https://oncall.example/platform"})
	eq(t, r.status, 200, r.text())
	eq(t, o.setOwners(o.admin, o.acme, group("SRE")).status, 200)
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)

	r = o.get(ownersPath(o.api), o.admin)
	eq(t, jsonText(at(r.json(t), "items", 0, "contacts")), `[{"kind":"chat","label":"#platform","value":"https://chat.example/platform"}]`)
	eq(t, at(r.json(t), "items", 0, "on_call_url"), any("https://oncall.example/platform"))
	eq(t, has(at(r.json(t), "items", 0).(obj), "members"), false)

	eq(t, o.call("DELETE", "/api/v1/groups/"+o.platform, o.admin).status, 204)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "SRE")
	eq(t, r.json(t)["from"], any(o.acme))

	bob := o.user("bob@example.com", false)
	eq(t, o.setOwners(o.admin, o.web, person("bob@example.com")).status, 200)
	eq(t, o.send("PATCH", "/api/v1/users/"+bob.ID.String(), o.admin, obj{"status": "disabled"}).status, 200)
	eq(t, at(o.get(ownersPath(o.web), o.admin).json(t), "items", 0, "status"), any("disabled"))
}

func TestOwnerCandidatesAndSuggestions(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("plato@example.com", false)
	o.user("ed@example.com", false)
	o.user("vi@example.com", false)
	o.grant(o.admin, o.backend, "user", "ed@example.com", "editor")
	o.grant(o.admin, o.backend, "user", "vi@example.com", "viewer")
	ed, vi := o.session("ed@example.com", password), o.session("vi@example.com", password)

	r := o.get(ownersPath(o.backend)+"/candidates?q=pla", ed)
	eq(t, r.status, 200, r.text())
	eq(t, ownerNames(t, r), "Platform,plato")
	eq(t, at(r.json(t), "items", 1, "email"), any("plato@example.com"))
	eq(t, code(t, o.get(ownersPath(o.backend)+"/candidates?q=p", ed)), any("validation.invalid_filter"))
	r = o.get(ownersPath(o.backend)+"/candidates?q=pla", vi)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))

	more := o.nodeID(o.admin, "project", o.backend, "more")
	o.nodeID(o.admin, "project", o.backend, "third")
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)
	eq(t, o.setOwners(o.admin, more, group("Platform"), group("SRE")).status, 200)
	secret := idOf(createGroup(t, o.testApp, o.admin, "Secret"))
	eq(t, o.setOwners(o.admin, o.web, group("Secret")).status, 200)

	r = o.get("/api/v1/catalog/owners?q=pl", vi)
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["items"]), `[{"count":3,"name":"Platform","subject_id":"`+o.platform+`","subject_kind":"group"}]`)
	r = o.get("/api/v1/catalog/owners", vi)
	eq(t, ownerNames(t, r), "Platform,SRE")
	eq(t, strings.Contains(r.text(), secret), false)
	eq(t, ownerNames(t, o.get("/api/v1/catalog/owners", o.admin)), "Platform,Secret,SRE")
}

func TestOwnersInTheCatalogTable(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	bob := o.user("bob@example.com", false)
	eq(t, o.call("PUT", "/api/v1/groups/"+o.platform+"/members/"+bob.ID.String(), o.admin).status, 204)
	o.grant(o.admin, o.acme, "user", "bob@example.com", "viewer")
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)
	eq(t, o.setOwners(o.admin, o.web, group("Frontend")).status, 200)

	r := o.get(tablePath("parent="+o.backend), o.admin)
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(at(r.json(t), "items", 0, "owners")), `[{"name":"Platform","subject_id":"`+o.platform+`","subject_kind":"group"}]`)

	session := o.session("bob@example.com", password)
	eq(t, names(t, o.get(tablePath("owner=me"), session)), "(acme),(backend),api")
	eq(t, names(t, o.get(tablePath("owner=group:"+o.frontend), session)), "(acme),web")
	eq(t, names(t, o.get(tablePath("owner=user:"+bob.ID.String()), session)), "")
	eq(t, names(t, o.get(tablePath("owner=me&activity=failed"), session)), "")
	eq(t, code(t, o.get(tablePath("owner=team:42"), session)), any("validation.invalid_filter"))

	o.user("nav@example.com", false)
	o.grant(o.admin, o.api, "user", "nav@example.com", "viewer")
	r = o.get(tablePath(""), o.session("nav@example.com", password))
	eq(t, at(r.json(t), "items", 0, "access"), any("navigate"))
	eq(t, jsonText(at(r.json(t), "items", 0, "owners")), `[]`)
}

func TestOwnerRoutesAndTokenScopes(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("ed@example.com", false)
	o.grant(o.admin, o.acme, "user", "ed@example.com", "editor")
	ed := o.session("ed@example.com", password)
	read, write := o.pat(ed, "read"), o.pat(ed, "write")
	eq(t, o.bearer("GET", ownersPath(o.api), read, nil).status, 200)
	eq(t, o.bearer("GET", ownersPath(o.api)+"/candidates?q=pla", read, nil).status, 200)
	eq(t, o.bearer("GET", "/api/v1/catalog/owners", read, nil).status, 200)
	r := o.bearer("PUT", ownersPath(o.api), read, obj{"owners": []obj{group("SRE")}})
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, o.bearer("PUT", ownersPath(o.api), write, obj{"owners": []obj{group("SRE")}}).status, 200)
	eq(t, code(t, o.bearer("GET", ownersPath(o.api), write, nil)), any("auth.insufficient_scope"))
	r = o.call("DELETE", ownersPath(o.api), o.admin)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "GET,HEAD,PUT")
}

func TestMCPProjectCarriesOwners(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	eq(t, o.send("PUT", "/api/v1/groups/"+o.platform+"/contacts", o.admin, obj{
		"contacts": []obj{}, "on_call_url": "https://oncall.example/platform"}).status, 200)
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)
	o.user("ed@example.com", false)
	o.grant(o.admin, o.acme, "user", "ed@example.com", "viewer")
	s := o.mcpSession(o.pat(o.session("ed@example.com", password), "mcp"))
	out, isErr := call(t, s, "get_project", map[string]any{"project": "acme/backend/api"})
	eq(t, isErr, false, out)
	contains(t, out, `"name": "Platform"`)
	contains(t, out, `"on_call_url": "https://oncall.example/platform"`)
	contains(t, out, `"from": "`+o.backend+`"`)
}

func TestOwnerTablesRefuseBadRows(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	contains(t, execErr(t, o.db, "INSERT INTO node_owners (id, node_id, position) VALUES (gen_random_uuid(), $1, 0)", o.api),
		"node_owners_subject_check")
	contains(t, execErr(t, o.db, "INSERT INTO node_owners (id, node_id, group_id, user_id, position) VALUES (gen_random_uuid(), $1, $2, (SELECT id FROM users LIMIT 1), 0)",
		o.api, o.platform), "node_owners_subject_check")
	contains(t, execErr(t, o.db, "UPDATE groups SET on_call_url = '' WHERE id = $1", o.platform), "groups_on_call_url_check")
	contains(t, execErr(t, o.db, "INSERT INTO group_contacts (id, group_id, kind, value, position) VALUES (gen_random_uuid(), $1, 'fax', 'x', 0)",
		o.platform), "group_contacts_kind_check")
	eq(t, o.setOwners(o.admin, o.api, group("Platform")).status, 200)
	eq(t, o.send("PUT", "/api/v1/groups/"+o.platform+"/contacts", o.admin, obj{"contacts": []obj{{"kind": "url", "value": "https://a.example"}}}).status, 200)
	execSQL(t, o.db, "DELETE FROM groups WHERE id = $1", o.platform)
	eq(t, o.count("node_owners"), int64(0))
	eq(t, o.count("group_contacts"), int64(0))
}
