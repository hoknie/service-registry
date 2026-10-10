package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	t1 = "2025-06-01T11:00:00Z"
	t2 = "2025-06-01T12:00:00Z"
	t3 = "2025-06-01T13:00:00Z"
)

func invalidKey(t *testing.T, r reply) {
	t.Helper()
	eq(t, r.status, 401, r.text())
	eq(t, r.json(t)["code"], any("auth.invalid_project_key"))
	eq(t, r.header("WWW-Authenticate"), "Bearer")
}

func (a *testApp) environments(cookie, project string) []any {
	a.t.Helper()
	r := a.get(recordsPath(project, "environments"), cookie)
	if r.status != 200 {
		a.t.Fatalf("environments: %d %s", r.status, r.body)
	}
	return list(a.t, r.json(a.t), "items")
}

func TestAnAcceptedEventIsAppliedAndListedAtOnce(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")

	r := app.ingest(pid, key, obj{
		"type": "service.deployed", "version": 1, "occurred_at": "2025-06-01T12:00:00Z",
		"idempotency_key": "run-1",
		"payload":         obj{"service": "API", "version": "1.4.2", "environment": "Production"},
	})
	eq(t, r.status, 201, r.text())
	v := r.json(t)
	eq(t, v["type"], any("service.deployed"))
	eq(t, v["version"], any(float64(1)))
	eq(t, v["replayed"], any(false))
	eq(t, v["occurred_at"], any("2025-06-01T12:00:00Z"))
	eq(t, v["idempotency_key"], any("run-1"))
	eq(t, strings.HasSuffix(v["received_at"].(string), "Z"), true)
	d := at(v, "result", "deployment").(obj)
	eq(t, d["service"], any("api"))
	eq(t, d["environment"], any("production"))
	eq(t, d["version"], any("1.4.2"))
	eq(t, d["became_current"], any(true))

	events := app.get(recordsPath(pid, "events"), admin).json(t)
	eq(t, events["total"], any(float64(1)))
	eq(t, at(events, "items", 0, "id"), v["id"])
	eq(t, at(events, "items", 0, "payload", "service"), any("API"), "stored as received")

	row := at(app.get(recordsPath(pid, "deployments"), admin).json(t), "items", 0).(obj)
	eq(t, row["service"], any("api"))
	eq(t, row["environment"], any("production"))
	for _, unset := range []string{"commit_sha", "branch", "cluster", "namespace", "url", "deployed_by"} {
		eq(t, has(row, unset) && row[unset] == nil, true, unset)
	}
	eq(t, jsonText(row["metadata"]), "{}")

	eq(t, at(app.get(keysPath(pid), admin).json(t), "items", 0, "last_used_at") != nil, true)
}

func TestOnlyAValidKeyOfThisProjectIsAccepted(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	a, keyA := app.projectWithKey(admin, "a")
	b, keyB := app.projectWithKey(admin, "b")
	body := jsonText(deployed("k", "api", "prod", "1.0.0", t1))

	invalidKey(t, app.ingestWith(a, body))
	invalidKey(t, app.ingestWith(a, body, "authorization", "Basic dXNlcjpwYXNz"))
	invalidKey(t, app.ingest(a, "svcr_bad", obj{}))
	distorted := []byte(keyA)
	if distorted[10] == 'a' {
		distorted[10] = 'b'
	} else {
		distorted[10] = 'a'
	}
	invalidKey(t, app.ingestWith(a, body, "authorization", "Bearer "+string(distorted)))

	invalidKey(t, app.ingest(a, keyB, deployed("k", "api", "prod", "1", t1)))
	invalidKey(t, app.ingest(newID(), keyA, deployed("k", "api", "prod", "1", t1)))
	eq(t, app.ingest("nope", keyA, deployed("k", "api", "prod", "1", t1)).status, 404)

	invalidKey(t, app.ingestWith(a, body, "cookie", admin))

	r := app.ingestWith(b, body, "authorization", "bearer "+keyB)
	eq(t, r.status, 201, r.text())

	keyID := at(app.get(keysPath(a), admin).json(t), "items", 0, "id").(string)
	eq(t, app.call("DELETE", keysPath(a)+"/"+keyID, admin).status, 204)
	invalidKey(t, app.ingest(a, keyA, deployed("k2", "api", "prod", "1", t1)))

	rotated := app.send("POST", keysPath(b), admin, obj{"grace_secs": 0})
	eq(t, rotated.status, 201)
	newKey := rotated.json(t)["secret"].(string)
	invalidKey(t, app.ingest(b, keyB, deployed("k3", "api", "prod", "1", t1)))
	r = app.ingest(b, newKey, deployed("k3", "api", "prod", "1", t1))
	eq(t, r.status, 201, r.text())

	eq(t, app.count("project_events"), int64(2), "nothing stored on 401")
}

func TestOriginChecksDoNotApplyToIngest(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	body := jsonText(deployed("k", "api", "prod", "1.0.0", t1))
	r := app.ingestWith(pid, body, "authorization", "Bearer "+key, "origin", "https://ci.example", "sec-fetch-site", "cross-site")
	eq(t, r.status, 201, r.text())
	eq(t, r.hasHeader("Access-Control-Allow-Origin"), false)

	invalidKey(t, app.ingestWith(pid, body, "sec-fetch-site", "cross-site"))
}

func TestBodiesThatAreNotJSONObjectsOrTooLargeAreRefused(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	auth := "Bearer " + key

	for _, body := range []string{"hello", "[]", `"x"`, "", "{} x"} {
		r := app.ingestWith(pid, body, "authorization", auth)
		eq(t, r.status, 400, body)
		eq(t, r.json(t)["code"], any("validation.invalid_body"))
	}
	jsonBody := jsonText(deployed("k", "api", "prod", "1", t1))
	for _, ct := range []string{"text/plain", "application/problem+json"} {
		r := request(t, app.addr, "POST", ingestPath(pid), jsonBody, "authorization", auth, "content-type", ct)
		eq(t, r.status, 400, ct)
		eq(t, r.json(t)["code"], any("validation.invalid_body"))
	}
	r := request(t, app.addr, "POST", ingestPath(pid), jsonBody, "authorization", auth, "content-type", "Application/JSON; charset=utf-8")
	eq(t, r.status, 201, "parameters and case of the media type do not matter: ", r.text())

	big := deployed("big", "api", "prod", "1", t1)
	big["payload"].(obj)["padding"] = strings.Repeat("x", 70_000)
	r = app.ingest(pid, key, big)
	eq(t, r.status, 413)
	eq(t, r.json(t)["code"], any("ingest.payload_too_large"))
	invalidKey(t, app.ingestWith(pid, jsonText(big)))
	eq(t, app.count("project_events"), int64(1))
}

func announceHuge(t *testing.T, addr, project, authorization string) (int, string, http.Header) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	must(t, err)
	defer conn.Close()
	auth := ""
	if authorization != "" {
		auth = "authorization: " + authorization + "\r\n"
	}
	fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nhost: x\r\ncontent-type: application/json\r\n%scontent-length: %d\r\n\r\n",
		ingestPath(project), auth, 3<<20)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	must(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), resp.Header
}

func TestBodiesOverTheServerLimitStillCheckTheKeyFirst(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	status, body, header := announceHuge(t, app.addr, pid, "")
	eq(t, status, 401, body)
	contains(t, body, `"code":"auth.invalid_project_key"`)
	eq(t, header.Get("WWW-Authenticate"), "Bearer")
	status, body, _ = announceHuge(t, app.addr, pid, "Bearer "+key)
	eq(t, status, 413, body)
	contains(t, body, `"code":"ingest.payload_too_large"`)
	status, _, _ = announceHuge(t, app.addr, "nope", "Bearer "+key)
	eq(t, status, 404)
}

func TestEnvelopeErrorsAre422WithFieldPaths(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")

	unknown := deployed("k", "api", "prod", "1", t1)
	unknown["type"] = "build.finished"
	r := app.ingest(pid, key, unknown)
	eq(t, r.status, 422)
	eq(t, r.text(), `{"code":"ingest.unknown_event_type","message":"unknown event type"}`)

	v2 := deployed("k", "api", "prod", "1", t1)
	v2["version"] = 2
	r = app.ingest(pid, key, v2)
	eq(t, r.status, 422)
	eq(t, r.json(t)["code"], any("ingest.unsupported_version"))

	r = app.ingest(pid, key, obj{"type": "service.deployed", "version": 1, "occurred_at": t1,
		"payload": obj{"service": "API!", "version": 12}})
	eq(t, r.status, 422)
	body := r.json(t)
	var keys []string
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	eq(t, strings.Join(keys, ","), "code,fields,message")
	eq(t, body["code"], any("ingest.invalid_event"))
	eq(t, jsonText(body["fields"]), `[{"code":"required","path":"idempotency_key"},`+
		`{"code":"invalid_value","path":"payload.service"},{"code":"invalid_type","path":"payload.version"},`+
		`{"code":"required","path":"payload.environment"}]`)
	contains(t, r.text(), `"fields":[{"path":"idempotency_key","code":"required"}`)

	badDate := deployed("k", "api", "prod", "1", "2026-02-30T10:00:00Z")
	badDate["payload"].(obj)["metadata"] = obj{"run_id": 42}
	eq(t, jsonText(app.ingest(pid, key, badDate).json(t)["fields"]),
		`[{"code":"invalid_value","path":"occurred_at"},{"code":"invalid_type","path":"payload.metadata.run_id"}]`)

	r = app.ingest(pid, key, deployed("k", "api", "prod", "1", "2999-01-01T00:00:00Z"))
	eq(t, r.status, 422)
	eq(t, jsonText(r.json(t)["fields"]), `[{"code":"in_future","path":"occurred_at"}]`)
	eq(t, app.count("project_events"), int64(0))
	eq(t, app.count("service_deployments"), int64(0))
}

func TestOptionalFieldsAreStoredAndBlankMeansUnset(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	e := deployed("k", "api", "prod", "1.0.0", t1)
	p := e["payload"].(obj)
	p["commit_sha"] = "ABCDEF1234567"
	p["branch"] = ""
	p["url"] = nil
	p["cluster"] = "eu-1"
	p["namespace"] = "prod"
	p["deployed_by"] = "octocat"
	p["metadata"] = obj{"run_id": "42"}
	eq(t, app.ingest(pid, key, e).status, 201)
	row := at(app.get(recordsPath(pid, "deployments"), admin).json(t), "items", 0).(obj)
	eq(t, row["commit_sha"], any("abcdef1234567"))
	eq(t, row["branch"], any(nil))
	eq(t, row["url"], any(nil))
	eq(t, row["cluster"], any("eu-1"))
	eq(t, row["namespace"], any("prod"))
	eq(t, row["deployed_by"], any("octocat"))
	eq(t, jsonText(row["metadata"]), `{"run_id":"42"}`)
}

func TestAReplayedEventReturnsTheOriginalResultWithoutDuplicates(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	other, otherKey := app.projectWithKey(admin, "web")

	e := deployed("run-7", "api", "production", "2.0.0", t2)
	e["payload"].(obj)["metadata"] = obj{"a": "1", "b": "2"}
	first := app.ingest(pid, key, e)
	eq(t, first.status, 201)

	reordered := `{"payload": {"metadata": {"b": "2", "a": "1"}, "version": "2.0.0",
		"environment": "production", "service": "api"},
		"idempotency_key": "run-7", "occurred_at": "2025-06-01T15:00:00+03:00",
		"version": 1, "type": "service.deployed"}`
	again := app.ingestWith(pid, reordered, "authorization", "Bearer "+key)
	eq(t, again.status, 200, again.text())
	f, a := first.json(t), again.json(t)
	eq(t, a["replayed"], any(true))
	for _, field := range []string{"id", "received_at", "occurred_at", "result"} {
		eq(t, jsonText(a[field]), jsonText(f[field]), field)
	}
	eq(t, app.count("project_events"), int64(1))
	eq(t, app.count("service_deployments"), int64(1))

	r := app.ingest(pid, key, deployed("run-7", "api", "production", "9.9.9", t3))
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.idempotency_key_reused"))
	eq(t, at(app.environments(admin, pid)[0], "version"), any("2.0.0"))

	eq(t, app.ingest(other, otherKey, e).status, 201)

	burst := jsonText(deployed("run-8", "api", "production", "2.1.0", t3))
	statuses := make([]int, 8)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Go(func() {
			statuses[i] = request(t, app.addr, "POST", ingestPath(pid), burst,
				"authorization", "Bearer "+key, "content-type", "application/json").status
		})
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		if s == 201 {
			created++
		} else if s != 200 {
			t.Fatalf("statuses %v", statuses)
		}
	}
	eq(t, created, 1, statuses)
	eq(t, app.count("project_events"), int64(3))
	eq(t, app.count("service_deployments"), int64(3))
}

func TestTheLatestOccurredAtIsCurrentWhateverTheArrivalOrder(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	became := func(r reply) any { return at(r.json(t), "result", "deployment", "became_current") }

	eq(t, became(app.ingest(pid, key, deployed("1", "api", "production", "2.0.0", t2))), any(true))
	r := app.ingest(pid, key, deployed("2", "api", "production", "1.9.0", t1))
	eq(t, r.status, 201)
	eq(t, became(r), any(false))
	envs := app.environments(admin, pid)
	eq(t, len(envs), 1)
	eq(t, at(envs[0], "version"), any("2.0.0"))
	eq(t, became(app.ingest(pid, key, deployed("3", "api", "production", "2.1.0", t3))), any(true))
	eq(t, became(app.ingest(pid, key, deployed("4", "api", "production", "2.1.1", t3))), any(true))
	eq(t, at(app.environments(admin, pid)[0], "version"), any("2.1.1"))

	eq(t, became(app.ingest(pid, key, deployed("5", "api", "staging", "3.0.0", t1))), any(true))
	var pairs []string
	for _, e := range app.environments(admin, pid) {
		pairs = append(pairs, at(e, "environment").(string)+"="+at(e, "version").(string))
	}
	eq(t, strings.Join(pairs, ","), "production=2.1.1,staging=3.0.0")
	eq(t, app.get(recordsPath(pid, "deployments"), admin).json(t)["total"], any(float64(5)))
}

func TestTooManyEventsPerKeyAreThrottled(t *testing.T) {
	t.Parallel()
	app := startApp(t, "INGEST_RATE_LIMIT_PER_MINUTE", "2")
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	for _, k := range []string{"1", "2"} {
		eq(t, app.ingest(pid, key, deployed(k, "api", "prod", "1", t1)).status, 201)
	}
	r := app.ingest(pid, key, deployed("3", "api", "prod", "1", t1))
	eq(t, r.status, 429)
	eq(t, r.json(t)["code"], any("ingest.rate_limited"))
	secs, err := strconv.Atoi(r.header("Retry-After"))
	if err != nil || secs < 1 || secs > 60 {
		t.Fatalf("Retry-After %q", r.header("Retry-After"))
	}
	invalidKey(t, app.ingest(pid, "svcr_bad", obj{}))
	eq(t, app.count("project_events"), int64(2))
}

func TestRetentionDeletesOldEventsButKeepsTheHistory(t *testing.T) {
	t.Parallel()
	app := startApp(t, "INGEST_EVENT_RETENTION_DAYS", "30")
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	for _, k := range []string{"old", "new"} {
		eq(t, app.ingest(pid, key, deployed(k, "api", "prod", k, t1)).status, 201)
	}
	execSQL(t, app.db, "UPDATE project_events SET received_at = now() - interval '31 days' WHERE idempotency_key = 'old'")
	for _, want := range []uint64{1, 0} {
		n, err := app.services.Ingest.PruneEvents(context.Background())
		if err != nil || n != want {
			t.Fatalf("prune: %d %v, want %d", n, err, want)
		}
	}

	events := app.get(recordsPath(pid, "events"), admin).json(t)
	eq(t, events["total"], any(float64(1)))
	eq(t, at(events, "items", 0, "idempotency_key"), any("new"))
	history := app.get(recordsPath(pid, "deployments"), admin).json(t)
	eq(t, history["total"], any(float64(2)))
	var old obj
	for _, d := range list(t, history, "items") {
		if d.(obj)["version"] == "old" {
			old = d.(obj)
		}
	}
	eq(t, has(old, "event_id") && old["event_id"] == nil, true)

	r := app.call("DELETE", nodePath(pid), admin)
	eq(t, r.status, 204, r.text())
	for _, table := range []string{"project_events", "service_deployments", "service_environments"} {
		eq(t, app.count(table), int64(0), table)
	}
}

func TestPayloadsComeBackAsSent(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, key := app.projectWithKey(admin, "api")
	payload := `{"version":"1","service":"api","environment":"prod","metadata":{"z":"line\u2028sep <&>"},
		"extra":{"n":1.50,"e":1e5,"big":12345678901234567890,"neg":-0.0,"s":"\u0001\u001f\"\\/","list":[1,2.5e-7,true,null]}}`
	body := `{"type":"service.deployed","version":1,"occurred_at":"2025-06-01T11:00:00Z","idempotency_key":"k","payload":` + payload + `}`
	eq(t, app.ingestWith(pid, body, "authorization", "Bearer "+key).status, 201)
	var want any
	must(t, json.Unmarshal([]byte(payload), &want))
	events := app.get(recordsPath(pid, "events"), admin).json(t)
	if got := at(events, "items", 0, "payload"); !reflect.DeepEqual(got, want) {
		t.Fatalf("payload %v, want %v", got, want)
	}
	deployments := app.get(recordsPath(pid, "deployments"), admin).json(t)
	eq(t, at(deployments, "items", 0, "metadata", "z"), any("line\u2028sep <&>"))
}
