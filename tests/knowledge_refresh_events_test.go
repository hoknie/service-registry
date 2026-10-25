package tests

import (
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	deploytypes "svc-registry/internal/feature/deploy/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	ingestports "svc-registry/internal/feature/ingest/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/testsupport/k8sfake"
	"svc-registry/pkg/testing/forgefake"
)

func TestProducerEventsAreRefreshEvents(t *testing.T) {
	t.Parallel()
	events := []string{ingestports.EventServiceDeployed, ingestports.EventClusterDeployed, forgeports.EventForgeWebhook, forgeports.EventForgeManual}
	for i, e := range events {
		eq(t, knowledgetypes.CollectEvent(e).Valid(), true, e)
		eq(t, knowledgetypes.CollectEvent(e), knowledgetypes.DefaultRefreshEvents[i])
	}
}

func TestAPIPathsAndRefreshEventsAreInheritedAndChecked(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	folder := a.nodeID(a.admin, "folder", a.org, "backend")
	other := a.nodeID(a.admin, "project", folder, "web")

	s := a.get(settingsPath(other), a.admin).json(t)
	eq(t, jsonText(s["api_paths"]), `["**/openapi*.{yaml,yml,json}","**/api-docs/**"]`)
	eq(t, jsonText(s["refresh_events"]), `["service.deployed","cluster.deployed","forge.webhook","forge.manual"]`)
	eq(t, jsonText(s["defaults"]), `{"api_paths":["**/openapi*.{yaml,yml,json}","**/api-docs/**"],`+
		`"refresh_events":["service.deployed","cluster.deployed","forge.webhook","forge.manual"]}`)
	eq(t, at(s, "from", "api_paths"), nil)
	eq(t, at(s, "from", "refresh_events"), nil)

	r := a.send("PUT", settingsPath(a.org), a.admin, obj{"api_paths": []string{"**/*.proto", "contracts/**"},
		"refresh": "event", "refresh_events": []string{"service.deployed", "forge.webhook"}})
	eq(t, r.status, 200, r.text())
	contains(t, r.text(), `"own":{"api_paths":["**/*.proto","contracts/**"],"refresh":"event","refresh_events":["service.deployed","forge.webhook"]}`)
	s = a.get(settingsPath(other), a.admin).json(t)
	eq(t, jsonText(s["api_paths"]), `["**/*.proto","contracts/**"]`)
	eq(t, jsonText(s["refresh_events"]), `["service.deployed","forge.webhook"]`)
	eq(t, at(s, "from", "api_paths", "id"), any(a.org))
	eq(t, at(s, "from", "refresh_events", "id"), any(a.org))

	r = a.send("PUT", settingsPath(a.project), a.admin, obj{"api_paths": []string{"**/*.proto", "spec/**"}, "refresh_events": []string{}})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(at(r.json(t), "api_paths")), `["**/*.proto","spec/**"]`)
	eq(t, jsonText(at(r.json(t), "refresh_events")), `[]`)
	eq(t, at(r.json(t), "refresh"), any("event"))
	eq(t, at(r.json(t), "from", "api_paths"), nil)
	eq(t, at(r.json(t), "from", "refresh", "id"), any(a.org))
	eq(t, jsonText(a.get(settingsPath(other), a.admin).json(t)["api_paths"]), `["**/*.proto","contracts/**"]`)

	r = a.send("PUT", settingsPath(a.project), a.admin, obj{"api_paths": []string{}})
	eq(t, jsonText(at(r.json(t), "api_paths")), `[]`)
	contains(t, r.text(), `"own":{"api_paths":[]}`)

	for _, body := range []obj{{"api_paths": nil}, {"api_paths": []string{"api/[a-"}}, {"api_paths": []string{""}},
		{"refresh_events": nil}, {"refresh_events": []string{"push"}}, {"refresh_events": []string{"forge.webhook", "forge.webhook"}}} {
		r = a.send("PUT", settingsPath(a.project), a.admin, body)
		eq(t, r.status, 400, body)
		eq(t, code(t, r), any("validation.invalid_knowledge_settings"))
	}
	contains(t, a.get(settingsPath(a.project), a.admin).text(), `"own":{"api_paths":[]}`)
}

func eventDocsWith(t *testing.T, d *docsProject, events ...string) string {
	t.Helper()
	eq(t, d.app.send("PUT", settingsPath(d.org), d.admin, obj{"refresh": "event", "refresh_events": events}).status, 200)
	eq(t, d.app.tick(), 1)
	eq(t, d.app.tick(), 0)
	return at(d.app.call("POST", nodePath(d.id)+"/keys", d.admin).json(t), "secret").(string)
}

func TestUnselectedEventsQueueNothing(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "v1"})
	key := eventDocsWith(t, d, "forge.webhook")

	d.putHead(2)
	eq(t, d.app.ingest(d.id, key, deployedOn("e1", "main", sha(2))).status, 201)
	eq(t, d.app.tick(), 0, "service.deployed is not selected")

	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(3)})
	eq(t, d.app.tick(), 0, "a manual sync is not selected")
	eq(t, d.last("main", "commit_sha"), sha(1))

	d.repo.Pushed = d.repo.Pushed.Add(time.Hour)
	d.putHead(4)
	execSQL(t, d.app.db, "UPDATE forge_connections SET next_run_at = now(), next_trigger = 'webhook' WHERE id = $1", d.conn)
	eq(t, d.app.runDue(), 1)
	eq(t, d.app.tick(), 1, "a webhook sync that moved the head")
	eq(t, d.last("main", "commit_sha"), sha(4))
	eq(t, d.app.lastScanTrigger(d.id), "event")

	eq(t, d.app.send("PUT", settingsPath(d.org), d.admin, obj{"refresh": "event", "refresh_events": []string{}}).status, 200)
	eq(t, d.app.tick(), 1, "a settings change queues a collection")
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(5)})
	eq(t, d.app.tick(), 0, "no events selected")
}

func TestClusterDeploymentQueuesCollection(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		events []string
		runs   int
	}{{"selected", []string{"cluster.deployed"}, 1}, {"not selected", []string{"service.deployed"}, 0}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			k8s := k8sfake.New(saToken)
			app := startAppWith(t, []svcapp.Option{svcapp.WithK8s(k8s)}, "SECRETS_KEYS", secretsKey, "PUBLIC_URL", "https://registry.example")
			d := startDocsOn(t, app, map[string]string{"README.md": "v1"})
			eventDocsWith(t, d, c.events...)
			app.cluster(d.admin, "prod", nil)
			k8s.Cluster().Put(workload("u1", "api", "1.0.0", map[string]string{deploytypes.AnnotationProject: "acme/api",
				deploytypes.AnnotationService: "api"}))
			app.pollNow()
			execSQL(t, app.db, "UPDATE cluster_workloads SET pending_since = pending_since - interval '11 minutes'")
			app.pollNow()
			eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM service_deployments WHERE source = 'cluster' AND project_id = $1", d.id), int64(1))
			eq(t, app.tick(), c.runs)
			if c.runs > 0 {
				eq(t, app.lastScanTrigger(d.id), "event")
			}
		})
	}
}
