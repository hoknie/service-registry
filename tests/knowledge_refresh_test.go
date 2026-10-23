package tests

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"svc-registry/pkg/testing/forgefake"
)

func (a *testApp) tick() int {
	a.t.Helper()
	ctx := context.Background()
	claimed, err := a.services.Knowledge.Collect.ClaimKnowledge(ctx, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, id := range claimed {
		if err := a.services.Knowledge.Collect.RunKnowledgeCollect(ctx, id); err != nil {
			a.t.Fatal(err)
		}
	}
	return len(claimed)
}

func (a *testApp) lastScanTrigger(project string) string {
	return scalar[string](a.t, a.db, "SELECT trigger FROM knowledge_scans WHERE project_id = $1 AND kind = 'collect' "+
		"ORDER BY last_started_at DESC, id DESC LIMIT 1", project)
}

func deployedOn(key, branch, commit string) obj {
	e := deployed(key, "api", "prod", "1.0."+key, time.Now().UTC().Format(time.RFC3339Nano))
	p := e["payload"].(obj)
	p["branch"], p["commit_sha"] = branch, commit
	return e
}

func eventDocs(t *testing.T) (*docsProject, string) {
	t.Helper()
	d := startDocs(t, map[string]string{"README.md": "v1"})
	eq(t, d.app.send("PUT", settingsPath(d.org), d.admin, obj{"refresh": "event"}).status, 200)
	eq(t, d.app.tick(), 1)
	eq(t, d.last("main", "commit_sha"), sha(1))
	eq(t, d.app.tick(), 0, "no timer in event mode")
	key := at(d.app.call("POST", nodePath(d.id)+"/keys", d.admin).json(t), "secret").(string)
	return d, key
}

func (d *docsProject) putHead(n int) {
	d.repo.Branches = []forgefake.Branch{{Name: "main", SHA: sha(n)}}
	d.fake.Put(d.repo)
}

func TestDeployEventQueuesCollectionInEventMode(t *testing.T) {
	t.Parallel()
	d, key := eventDocs(t)
	d.putHead(2)
	event := deployedOn("e1", "main", sha(2))
	r := d.app.ingest(d.id, key, event)
	eq(t, r.status, 201, r.text())
	eq(t, d.app.tick(), 1)
	eq(t, d.last("main", "commit_sha"), sha(2))
	eq(t, d.app.lastScanTrigger(d.id), "event")

	eq(t, d.app.ingest(d.id, key, event).status, 200)
	eq(t, d.app.tick(), 0, "a replay queues nothing")

	d.app.collect()
	eq(t, d.app.tick(), 0, "the timer stays off")
	eq(t, d.app.call("POST", knowledgePath(d.id)+"/collect", d.admin).status, 202)
	eq(t, d.app.tick(), 1)
	eq(t, d.app.lastScanTrigger(d.id), "manual")

	r = d.app.get("/api/v1/knowledge/scans?kind=collect&trigger=event", d.admin)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1)
	eq(t, items[0].(obj)["trigger"], any("event"))
	eq(t, d.app.get(nodePath(d.id)+"/knowledge/scans?trigger=event", d.admin).status, 200)
}

func TestDeployEventInIntervalModeQueuesNothing(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "v1"})
	d.app.collect()
	key := at(d.app.call("POST", nodePath(d.id)+"/keys", d.admin).json(t), "secret").(string)
	d.putHead(2)
	eq(t, d.app.ingest(d.id, key, deployedOn("e1", "main", sha(2))).status, 201)
	eq(t, d.app.tick(), 0)
	eq(t, scalar[bool](t, d.app.db, "SELECT triggered_at IS NULL AND next_run_at > now() FROM knowledge_settings WHERE project_id = $1", d.id), true)
	d.app.collect()
	eq(t, d.last("main", "commit_sha"), sha(2))
	eq(t, d.app.lastScanTrigger(d.id), "schedule")
}

func TestForgeRunsQueueCollectionOnlyWhenNotScheduled(t *testing.T) {
	t.Parallel()
	d, _ := eventDocs(t)
	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(3)})
	eq(t, d.app.tick(), 1, "a manual sync that moved the head")
	eq(t, d.last("main", "commit_sha"), sha(3))
	eq(t, d.app.lastScanTrigger(d.id), "event")

	d.repo.Pushed = d.repo.Pushed.Add(time.Hour)
	d.putHead(4)
	execSQL(t, d.app.db, "UPDATE forge_connections SET next_run_at = now(), next_trigger = 'schedule' WHERE id = $1", d.conn)
	eq(t, d.app.runDue(), 1)
	eq(t, scalar[string](t, d.app.db, "SELECT head_sha FROM branches WHERE project_id = $1 AND name = 'main'", d.id), sha(4))
	eq(t, d.app.tick(), 0, "a scheduled sync does not trigger")
	eq(t, d.last("main", "commit_sha"), sha(3))

	d.push(nil, forgefake.Branch{Name: "main", SHA: sha(4)})
	eq(t, d.app.tick(), 0, "the head did not move")
}

func TestEventsDuringCollectionQueueOneMoreRun(t *testing.T) {
	t.Parallel()
	d, key := eventDocs(t)
	d.putHead(5)
	eq(t, d.app.ingest(d.id, key, deployedOn("e1", "main", sha(5))).status, 201)
	d.fake.Delay(400 * time.Millisecond)
	var wg sync.WaitGroup
	var first int
	wg.Go(func() { first = d.app.tick() })
	time.Sleep(150 * time.Millisecond)
	for _, k := range []string{"e2", "e3", "e4"} {
		eq(t, d.app.ingest(d.id, key, deployedOn(k, "main", sha(5))).status, 201)
	}
	eq(t, scalar[bool](t, d.app.db, "SELECT lease_until > now() FROM knowledge_settings WHERE project_id = $1", d.id), true, "events came during the run")
	wg.Wait()
	eq(t, first, 1)
	d.fake.Delay(0)
	eq(t, d.last("main", "commit_sha"), sha(5))
	eq(t, d.app.tick(), 1, "one more run for the events that came meanwhile")
	eq(t, d.app.tick(), 0)
}

func TestLocalDirInEventModeWaitsForAnEvent(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# v1")
	a.useSource(a.project, "local_dir", dir, "*.md")
	eq(t, a.send("PUT", settingsPath(a.project), a.admin, obj{"include": []string{"*.md"}, "refresh": "event"}).status, 200)
	eq(t, a.tick(), 1)
	eq(t, a.processes(a.project)["collect"]["state"], any("idle"), "waiting for an event is not queued")
	first := scalar[string](t, a.db, "SELECT commit_sha FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", a.project)
	writeFile(t, filepath.Join(dir, "README.md"), "# v2")
	a.collect()
	eq(t, a.tick(), 0)
	eq(t, scalar[int64](t, a.db, "SELECT count(*) FROM knowledge_snapshots WHERE project_id = $1", a.project), int64(1))
	eq(t, a.call("POST", knowledgePath(a.project)+"/collect", a.admin).status, 202)
	eq(t, a.tick(), 1)
	eq(t, scalar[string](t, a.db, "SELECT commit_sha FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", a.project) != first, true)

	eq(t, a.send("PUT", settingsPath(a.project), a.admin, obj{"include": []string{"*.md"}, "refresh": "interval"}).status, 200)
	eq(t, a.tick(), 1, "switching back queues a collection")
	eq(t, scalar[bool](t, a.db, "SELECT next_run_at > now() FROM knowledge_settings WHERE project_id = $1", a.project), true)
}

func TestDeployEventCollectsARemoteSource(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	f := forgefake.Start(t, "gitlab", forgeToken, "platform")
	repo := forgefake.Repo{ID: 7, Path: "platform/api", Files: map[string]string{"README.md": "# API"},
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(7)}}}
	f.Put(repo)
	r := a.send("POST", "/api/v1/catalog/nodes", a.admin, obj{"kind": "project", "parent_id": a.org, "slug": "remote", "name": "Remote",
		"source": obj{"kind": "remote", "forge": "gitlab", "url": "https://gitlab.example.com/platform/api",
			"api_url": f.APIURL() + "/api/v4", "credentials": a.tokenOn(a.admin, a.org, forgeToken)}})
	eq(t, r.status, 201, r.text())
	id, key := idOf(r.json(t)), at(r.json(t), "key", "secret").(string)
	eq(t, a.send("PUT", settingsPath(id), a.admin, obj{"refresh": "event"}).status, 200)
	a.tick()
	eq(t, scalar[string](t, a.db, "SELECT commit_sha FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", id), sha(7))

	repo.Branches = []forgefake.Branch{{Name: "main", SHA: sha(8)}}
	f.Put(repo)
	a.collect()
	eq(t, a.tick(), 0)
	eq(t, a.ingest(id, key, deployedOn("d1", "main", sha(8))).status, 201)
	eq(t, a.tick(), 1)
	eq(t, scalar[string](t, a.db, "SELECT commit_sha FROM knowledge_snapshots WHERE project_id = $1 ORDER BY collected_at DESC LIMIT 1", id), sha(8))
	eq(t, a.lastScanTrigger(id), "event")
}
