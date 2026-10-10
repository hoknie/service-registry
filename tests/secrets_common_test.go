package tests

import (
	"strconv"
	"sync/atomic"
)

const globalSecretsPath = "/api/v1/secrets"

func secretsPath(node string) string { return nodePath(node) + "/secrets" }

var secretSeq atomic.Int64

func (a *testApp) secret(cookie, node string, body obj) string {
	a.t.Helper()
	if _, ok := body["name"]; !ok {
		body["name"] = "secret-" + strconv.FormatInt(secretSeq.Add(1), 10)
	}
	path := globalSecretsPath
	if node != "" {
		path = secretsPath(node)
	}
	r := a.send("POST", path, cookie, body)
	if r.status != 201 {
		a.t.Fatalf("secret: %d %s", r.status, r.body)
	}
	return idOf(r.json(a.t))
}

func (a *testApp) tokenOn(cookie, node, token string) obj {
	a.t.Helper()
	return obj{"secret_id": a.secret(cookie, node, obj{"value": token})}
}

func (a *testApp) refOn(cookie, node, ref string) obj {
	a.t.Helper()
	return obj{"secret_id": a.secret(cookie, node, obj{"ref": ref})}
}
