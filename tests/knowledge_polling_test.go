package tests

import (
	"testing"

	"svc-registry/pkg/testing/hostingfake"
)

func (d *docsProject) secondsUntilCollect() float64 {
	d.app.t.Helper()
	return scalar[float64](d.app.t, d.app.db, "SELECT extract(epoch FROM next_run_at - now())::float8 FROM knowledge_settings WHERE project_id = $1", d.id)
}

func (d *docsProject) newHead(n int) {
	d.app.t.Helper()
	d.push(nil, hostingfake.Branch{Name: "main", SHA: sha(n)})
}

func (d *docsProject) collectScans() int64 {
	return scalar[int64](d.app.t, d.app.db, "SELECT count(*) FROM knowledge_scans WHERE project_id = $1 AND kind = 'collect'", d.id)
}

func TestDocsCollectionIntervalComesFromPollingSettings(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "v1"}, "KNOWLEDGE_INTERVAL_SECS", "300")
	eq(t, d.app.tick(), 1)
	if s := d.secondsUntilCollect(); s < 200 || s > 300 {
		t.Fatalf("next collection in %v s, want about 300", s)
	}
	eq(t, d.app.get(settingsPath(d.id), d.admin).json(t)["interval_secs"], any(float64(300)))

	eq(t, d.app.send("PUT", pollingPath(d.id), d.admin, obj{"interval_secs": 3600}).status, 200)
	eq(t, d.app.tick(), 1, "a changed setting queues the project")
	if s := d.secondsUntilCollect(); s < 3500 || s > 3600 {
		t.Fatalf("next collection in %v s, want about 3600", s)
	}
	eq(t, d.app.get(settingsPath(d.id), d.admin).json(t)["interval_secs"], any(float64(3600)))
}

func TestDocsCollectionStopsWhenPollingIsOff(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "v1"})
	eq(t, d.app.tick(), 1)
	eq(t, d.last("main", "commit_sha"), sha(1))
	scans := d.collectScans()

	eq(t, d.app.send("PUT", pollingPath(d.org), d.admin, obj{"mode": "off"}).status, 200)
	d.newHead(2)
	d.app.collect()
	eq(t, d.last("main", "commit_sha"), sha(1))
	eq(t, d.collectScans(), scans, "a paused collection records no scan")
	trees := d.fake.Calls("tree")
	key := at(d.app.call("POST", nodePath(d.id)+"/keys", d.admin).json(t), "secret").(string)
	eq(t, d.app.send("PUT", settingsPath(d.org), d.admin, obj{"refresh": "event"}).status, 200)
	eq(t, d.app.ingest(d.id, key, deployedOn("e1", "main", sha(2))).status, 201)
	d.app.tick()
	eq(t, d.last("main", "commit_sha"), sha(1))
	eq(t, d.fake.Calls("tree"), trees)

	eq(t, d.app.call("POST", knowledgePath(d.id)+"/collect", d.admin).status, 202)
	eq(t, d.app.tick(), 1)
	eq(t, d.last("main", "commit_sha"), sha(2))
	eq(t, d.app.lastScanTrigger(d.id), "manual")
}

func TestPublishedModeCollectsOnlyPublishedDocs(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "v1"})
	eq(t, d.app.send("PUT", pollingPath(d.org), d.admin, obj{"mode": "published"}).status, 200)
	d.app.collect()
	eq(t, d.snapshots("main"), int64(0))

	r := d.app.send("PUT", publicationPath(d.org), d.admin, obj{"publish": true})
	eq(t, r.status, 200, r.text())
	eq(t, d.app.tick(), 1, "publishing queues the collection")
	eq(t, d.last("main", "commit_sha"), sha(1))

	d.newHead(2)
	d.app.collect()
	eq(t, d.last("main", "commit_sha"), sha(2))

	eq(t, d.app.send("PUT", publicationPath(d.org), d.admin, obj{"publish": false}).status, 200)
	d.newHead(3)
	d.app.collect()
	eq(t, d.last("main", "commit_sha"), sha(2))

	eq(t, d.app.send("PUT", pollingPath(d.id), d.admin, obj{"mode": "on"}).status, 200)
	d.app.tick()
	eq(t, d.last("main", "commit_sha"), sha(3))
}
