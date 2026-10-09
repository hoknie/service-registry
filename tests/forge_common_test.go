package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"svc-registry/internal/service"
	"svc-registry/internal/testsupport/forgefake"
)

const secretsKey = "k1:MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

const forgeToken = "forge-token-1"

func startForgeApp(t testing.TB, extra ...string) *testApp {
	t.Helper()
	return startApp(t, append([]string{"SECRETS_KEYS", secretsKey, "PUBLIC_URL", "https://registry.example"}, extra...)...)
}

func connectionsPath(node string) string { return nodePath(node) + "/forge/connections" }

func connectionPath(node, conn string) string { return connectionsPath(node) + "/" + conn }

func (a *testApp) connect(cookie, node string, f *forgefake.Fake, extra obj) obj {
	a.t.Helper()
	body := obj{"kind": f.Kind, "api_url": f.APIURL(), "owner_path": f.Owner, "credentials": obj{"token": f.Token}}
	for k, v := range extra {
		body[k] = v
	}
	r := a.send("POST", connectionsPath(node), cookie, body)
	if r.status != 201 {
		a.t.Fatalf("connect: %d %s", r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) runDue() int {
	a.t.Helper()
	ctx := context.Background()
	claimed, err := service.ClaimForgeRuns(ctx, a.state, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, c := range claimed {
		service.RunClaimedSync(ctx, a.state, c)
	}
	return len(claimed)
}

func (a *testApp) syncNow(cookie, node, conn string) obj {
	a.t.Helper()
	if r := a.call("POST", connectionPath(node, conn)+"/sync", cookie); r.status != 202 {
		a.t.Fatalf("sync: %d %s", r.status, r.body)
	}
	a.runDue()
	return a.lastRun(cookie, node, conn)
}

func (a *testApp) lastRun(cookie, node, conn string) obj {
	a.t.Helper()
	r := a.get(connectionPath(node, conn)+"/runs", cookie)
	if r.status != 200 {
		a.t.Fatalf("runs: %d %s", r.status, r.body)
	}
	items := list(a.t, r.json(a.t), "items")
	if len(items) == 0 {
		a.t.Fatal("no runs")
	}
	return items[0].(obj)
}

func (a *testApp) childBySlug(cookie, parent, slug string) obj {
	a.t.Helper()
	r := a.get("/api/v1/catalog/nodes?parent="+parent, cookie)
	for _, it := range list(a.t, r.json(a.t), "items") {
		if it.(obj)["slug"] == slug {
			return it.(obj)
		}
	}
	return nil
}

func writeTemp(t testing.TB, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
