package tests

import (
	"strings"
	"testing"

	"svc-registry/internal/deploy"
)

func TestClusterLifecycle(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	c := app.cluster(root, "prod-eu", obj{"environment": "Production", "namespaces": []string{"backend", " backend "}})
	eq(t, c["environment"], any("production"))
	eq(t, c["in_cluster"], any(false))
	eq(t, c["interval_secs"], any(float64(60)))
	eq(t, c["enabled"], any(true))
	eq(t, c["status"], any("never"))
	eq(t, at(c, "credentials", "kind"), any("token"))
	eq(t, has(c["credentials"].(obj), "fingerprint"), true)
	eq(t, jsonText(c["namespaces"]), `["backend"]`)
	id := idOf(c)
	lacks(t, scalar[string](t, app.db, "SELECT row_to_json(c)::text FROM clusters c"), saToken)
	lacks(t, app.get(clusterPath(id), root).text(), saToken)

	r := app.send("POST", clustersPath, root, obj{"name": "PROD-EU", "environment": "production", "in_cluster": true})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.cluster_name_taken"))
	for _, tt := range []struct {
		body obj
		code string
	}{
		{obj{"name": "x", "environment": "p", "in_cluster": true, "api_url": "https://x"}, "validation.invalid_api_url"},
		{obj{"name": "x", "environment": "p", "api_url": "http://x", "credentials": obj{"token": "t"}}, "validation.invalid_api_url"},
		{obj{"name": "x", "environment": "p", "api_url": "https://x"}, "validation.invalid_credentials"},
		{obj{"name": "x", "environment": "a b", "in_cluster": true}, "validation.invalid_environment_key"},
		{obj{"name": " ", "environment": "p", "in_cluster": true}, "validation.invalid_cluster_name"},
		{obj{"name": "x", "environment": "p", "in_cluster": true, "interval_secs": 5}, "validation.invalid_poll_interval"},
		{obj{"name": "x", "environment": "p", "in_cluster": true, "ca_pem": "nope"}, "validation.invalid_ca"},
		{obj{"name": "x", "environment": "p", "in_cluster": true, "namespaces": []string{"Bad"}}, "validation.invalid_namespaces"},
		{obj{"name": "x", "environment": "p", "in_cluster": true, "rules": []obj{{"label": "app", "project": "A/B"}}}, "validation.invalid_cluster_rules"},
	} {
		r := app.send("POST", clustersPath, root, tt.body)
		eq(t, r.status, 400, jsonText(tt.body))
		eq(t, code(t, r), any(tt.code), jsonText(tt.body))
	}

	ref := app.cluster(root, "stage", obj{"credentials": obj{"token_ref": "env:K8S_TOKEN_STAGE"}})
	eq(t, at(ref, "credentials", "ref"), any("env:K8S_TOKEN_STAGE"))
	self := app.send("POST", clustersPath, root, obj{"name": "self", "environment": "staging", "in_cluster": true})
	eq(t, self.status, 201, self.text())
	eq(t, self.json(t)["credentials"], nil)
	eq(t, self.json(t)["api_url"], nil)

	r = app.send("PATCH", clusterPath(id), root, obj{"interval_secs": 30, "enabled": false})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["interval_secs"], any(float64(30)))
	eq(t, at(r.json(t), "credentials", "fingerprint"), at(c, "credentials", "fingerprint"))
	eq(t, app.send("PATCH", clusterPath(newID()), root, obj{"enabled": true}).status, 404)

	list := app.get(clustersPath, root)
	eq(t, len(list.json(t)["items"].([]any)), 3)

	k8s.Cluster().Missing("pods:list")
	r = app.call("POST", clusterPath(id)+"/test", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["ok"], any(true))
	eq(t, r.json(t)["version"], any("v1.31.2"))
	eq(t, jsonText(r.json(t)["missing"]), `["pods:list"]`)
	eq(t, k8s.Accesses()[0].Token, saToken)
	eq(t, app.send("PATCH", clusterPath(id), root, obj{"credentials": obj{"token": "wrong"}}).status, 200)
	r = app.call("POST", clusterPath(id)+"/test", root)
	eq(t, r.json(t)["ok"], any(false))
	eq(t, at(r.json(t), "error", "code"), any("k8s.unauthorized"))
	k8s.Cluster().Fail(&deploy.Failure{Code: deploy.FailUnreachable, Message: "dial"})
	r = app.call("POST", idPath(self.json(t))+"/test", root)
	eq(t, at(r.json(t), "error", "code"), any("k8s.unreachable"))

	eq(t, app.call("POST", clusterPath(id)+"/poll", root).status, 202)
	eq(t, app.call("POST", clusterPath(newID())+"/poll", root).status, 404)
	eq(t, app.call("DELETE", clusterPath(id), root).status, 204)
	eq(t, app.call("DELETE", clusterPath(id), root).status, 404)
	r = app.call("PUT", clusterPath(idOf(ref)), root)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "GET,HEAD,PATCH,DELETE")
}

func idPath(c obj) string { return clusterPath(idOf(c)) }

func TestClustersAreForTheSuperadminOnly(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	app.user("ann@example.com", false)
	app.grant(root, org, "user", "ann@example.com", "admin")
	ann := app.session("ann@example.com", password)
	r := app.get(clustersPath, ann)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, code(t, app.send("POST", clustersPath, ann, obj{"name": "x"})), any("auth.forbidden"))
	r = app.bearer("GET", clustersPath, app.pat(root, "read", "write"), nil)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("GET", clustersPath, app.pat(root, "admin"), nil).status, 200)
}

func TestClusterTokenNeedsAnEncryptionKey(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	r := app.send("POST", clustersPath, root, obj{"name": "x", "environment": "p", "api_url": "https://x", "credentials": obj{"token": "t"}})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.secrets_key_missing"))
	eq(t, strings.Contains(r.text(), "\"t\""), false)
}
