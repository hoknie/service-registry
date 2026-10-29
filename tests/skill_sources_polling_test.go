package tests

import (
	"path/filepath"
	"testing"
)

func (s *skillSync) pollingSource(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := filepath.Join(s.root, "repo")
	writeFile(t, filepath.Join(dir, "skills/go-testing/SKILL.md"), skillMD("go-testing", ""))
	writeFile(t, filepath.Join(dir, "skills/sql-review/SKILL.md"), skillMD("sql-review", ""))
	id := idOf(s.source(obj{"kind": "local_dir", "path": dir, "root": "skills"}))
	eq(s.t, at(s.runSource(id), "last_run", "created"), any(float64(2)))
	skills := map[string]string{}
	for _, it := range list(s.t, s.skills(), "items") {
		skills[it.(obj)["slug"].(string)] = idOf(it.(obj))
	}
	return id, skills
}

func (s *skillSync) makeDue(id string) {
	s.t.Helper()
	execSQL(s.t, s.db, "UPDATE skill_sources SET next_run_at = now() WHERE id = $1", id)
}

func (s *skillSync) sourceSecondsUntilRun(id string) float64 {
	s.t.Helper()
	return scalar[float64](s.t, s.db, "SELECT extract(epoch FROM next_run_at - now())::float8 FROM skill_sources WHERE id = $1", id)
}

func TestDisabledFolderPausesItsSkillSource(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	id, _ := s.pollingSource(t)
	started := at(s.get(skillsPath+"/sources/"+id, s.admin).json(t), "last_run", "started_at")

	eq(t, s.send("PATCH", skillsPath+"/folders/"+s.folder, s.admin, obj{"disabled": true}).status, 200)
	src := s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["state"], any("paused"))
	eq(t, src["paused_reason"], any("folder_disabled"))
	s.makeDue(id)
	src = s.runSource(id)
	eq(t, at(src, "last_run", "started_at"), started)
	if secs := s.sourceSecondsUntilRun(id); secs < 60 {
		t.Fatalf("a paused source must be postponed, next run in %v s", secs)
	}

	r := s.call("POST", skillsPath+"/sources/"+id+"/sync", s.admin)
	eq(t, r.status, 202, r.text())
	eq(t, r.json(t)["state"], any("queued"))
	src = s.runSource(id)
	eq(t, at(src, "last_run", "status"), any("unchanged"))
	eq(t, src["state"], any("paused"))

	eq(t, s.send("PATCH", skillsPath+"/folders/"+s.folder, s.admin, obj{"disabled": false}).status, 200)
	src = s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["paused_reason"], any(nil))
	eq(t, src["state"], any("queued"))
}

func TestSourceWithEverySkillDisabledIsPaused(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	id, skills := s.pollingSource(t)
	for _, skill := range skills {
		eq(t, s.send("PATCH", skillPath(skill), s.admin, obj{"status": "disabled"}).status, 200)
	}
	src := s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["state"], any("paused"))
	eq(t, src["paused_reason"], any("skills_disabled"))

	eq(t, s.send("PATCH", skillPath(skills["go-testing"]), s.admin, obj{"status": "published"}).status, 200)
	src = s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["paused_reason"], any(nil))
	eq(t, src["state"], any("queued"))
}

func TestOrganizationPollingSettingsApplyToSkillSources(t *testing.T) {
	t.Parallel()
	s := startSkillSyncWith(t, "SKILLS_SYNC_INTERVAL_SECS", "3600")
	id, _ := s.pollingSource(t)
	acme := scalar[string](t, s.db, "SELECT id::text FROM nodes WHERE slug = 'acme'")
	src := s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["interval_secs"], any(nil))
	eq(t, src["effective_interval_secs"], any(float64(3600)))

	eq(t, s.send("PUT", pollingPath(acme), s.admin, obj{"interval_secs": 7200}).status, 200)
	src = s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["effective_interval_secs"], any(float64(7200)))
	eq(t, src["state"], any("queued"), "a changed setting queues the source")
	s.runSource(id)
	if secs := s.sourceSecondsUntilRun(id); secs < 7100 || secs > 7200 {
		t.Fatalf("next run in %v s, want about 7200", secs)
	}

	eq(t, s.send("PUT", pollingPath(acme), s.admin, obj{"mode": "off"}).status, 200)
	src = s.get(skillsPath+"/sources/"+id, s.admin).json(t)
	eq(t, src["state"], any("paused"))
	eq(t, src["paused_reason"], any("polling_off"))
}
