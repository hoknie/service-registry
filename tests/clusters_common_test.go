package tests

import (
	"testing"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/testsupport/k8sfake"
)

const clustersPath = "/api/v1/clusters"

const saToken = "sa-token-1"

func clusterPath(id string) string { return clustersPath + "/" + id }

func startClusterApp(t testing.TB, extra ...string) (*testApp, *k8sfake.Factory) {
	t.Helper()
	f := k8sfake.New(saToken)
	app := startAppWith(t, []svcapp.Option{svcapp.WithK8s(f)}, append([]string{"SECRETS_KEYS", secretsKey}, extra...)...)
	return app, f
}

func (a *testApp) cluster(cookie, name string, extra obj) obj {
	a.t.Helper()
	body := obj{"name": name, "environment": "production", "api_url": "https://k8s.example:6443",
		"credentials": obj{"token": saToken}}
	for k, v := range extra {
		body[k] = v
	}
	r := a.send("POST", clustersPath, cookie, body)
	if r.status != 201 {
		a.t.Fatalf("cluster: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}
