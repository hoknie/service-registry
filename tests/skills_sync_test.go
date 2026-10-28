package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"svc-registry/internal/presentation/jobs"
)

type skillSync struct {
	*testApp
	root   string
	admin  string
	folder string
}

func startSkillSync(t *testing.T) *skillSync {
	t.Helper()
	return startSkillSyncWith(t)
}

func startSkillSyncWith(t *testing.T, extra ...string) *skillSync {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	a := startApp(t, append([]string{"KNOWLEDGE_LOCAL_ROOTS", root}, extra...)...)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	folder := idOf(a.skillFolder(admin, obj{"organization_id": acme, "slug": "agents", "name": "Agents"}))
	return &skillSync{testApp: a, root: root, admin: admin, folder: folder}
}

func (s *skillSync) source(body obj) obj {
	s.t.Helper()
	r := s.send("POST", skillsPath+"/folders/"+s.folder+"/sources", s.admin, body)
	if r.status != 201 {
		s.t.Fatalf("source: %d %s", r.status, r.body)
	}
	return r.json(s.t)
}

func (s *skillSync) runSource(id string) obj {
	s.t.Helper()
	sid := parseUUID(s.t, id)
	must(s.t, s.services.Skills.Sources.RunSource(context.Background(), sid))
	return s.get(skillsPath+"/sources/"+id, s.admin).json(s.t)
}

func (s *skillSync) skills() obj {
	return s.get(skillsPath+"?folder_id="+s.folder, s.admin).json(s.t)
}

func TestSkillSourceSyncsALocalDirectory(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	dir := filepath.Join(s.root, "repo")
	writeFile(t, filepath.Join(dir, "skills/go-testing/SKILL.md"), skillMD("go-testing", ""))
	writeFile(t, filepath.Join(dir, "skills/go-testing/reference.md"), "# Ref\n")
	writeFile(t, filepath.Join(dir, "skills/go-testing/nested/SKILL.md"), skillMD("nested", ""))
	writeFile(t, filepath.Join(dir, "skills/sql-review/SKILL.md"), skillMD("sql-review", "  tags: [sql]\n"))
	writeFile(t, filepath.Join(dir, "skills/broken/SKILL.md"), "---\nname: broken\n---\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# Repo\n")
	src := s.source(obj{"kind": "local_dir", "path": dir, "root": "skills"})
	eq(t, src["state"], any("queued"))
	id := idOf(src)

	got := s.runSource(id)
	eq(t, at(got, "last_run", "status"), any("succeeded"), jsonText(got))
	eq(t, at(got, "last_run", "created"), any(float64(2)))
	eq(t, at(got, "last_run", "skipped"), any(float64(1)))
	eq(t, at(got, "last_run", "problems", 0, "code"), any("skills.manifest_invalid"))
	eq(t, at(got, "last_run", "problems", 0, "path"), any("skills/broken"))
	eq(t, got["state"], any("idle"))
	list := s.skills()
	eq(t, fieldsOf(t, list, "name"), "sql-review,go-testing")
	eq(t, at(list, "items", 1, "managed"), any(true))
	eq(t, at(list, "items", 1, "status"), any("published"))
	eq(t, at(list, "items", 1, "author"), any("repo"))
	eq(t, jsonText(at(list, "items", 0, "tags")), `["sql"]`)
	goTesting := idOf(at(list, "items", 1).(map[string]any))
	files := s.get(skillPath(goTesting)+"/files", s.admin).json(t)
	eq(t, fieldsOf(t, files, "path"), "SKILL.md,nested/SKILL.md,reference.md")

	got = s.runSource(id)
	eq(t, at(got, "last_run", "status"), any("unchanged"))

	writeFile(t, filepath.Join(dir, "skills/go-testing/reference.md"), "# Ref 2\n")
	removeAll(filepath.Join(dir, "skills/sql-review"))
	got = s.runSource(id)
	eq(t, at(got, "last_run", "updated"), any(float64(1)), jsonText(got))
	eq(t, at(got, "last_run", "removed"), any(float64(1)))
	eq(t, fieldsOf(t, s.skills(), "name"), "go-testing")
	eq(t, at(s.get(skillPath(goTesting), s.admin).json(t), "version", "version"), any("0.1.1"))

	r := s.sendForm("PUT", skillPath(goTesting)+"/content", s.admin, form{archive: skillZip(t, map[string]string{"SKILL.md": skillMD("go-testing", "")})})
	eq(t, code(t, r), any("conflict.skill_managed_by_source"))
	eq(t, code(t, s.call("DELETE", skillPath(goTesting), s.admin)), any("conflict.skill_managed_by_source"))
	eq(t, code(t, s.send("PATCH", skillPath(goTesting), s.admin, obj{"tags": []string{"x"}})), any("conflict.skill_managed_by_source"))
	r = s.send("PATCH", skillPath(goTesting), s.admin, obj{"about": "Наш **разбор** навыка"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["about"], any("Наш **разбор** навыка"))
	eq(t, code(t, s.send("PATCH", skillPath(goTesting), s.admin, obj{"about": strings.Repeat("я", 4001)})), any("validation.invalid_skill"))
	eq(t, s.send("PATCH", skillPath(goTesting), s.admin, obj{"status": "draft"}).status, 200)

	r = s.send("POST", skillsPath+"/sources/"+id+"/sync", s.admin, obj{})
	eq(t, r.status, 202)
	eq(t, r.json(t)["state"], any("queued"))
	eq(t, s.call("DELETE", skillsPath+"/sources/"+id, s.admin).status, 204)
	eq(t, s.count("skills"), int64(0))
}

func TestSkillSourceVersionsFromGitTags(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	dir := filepath.Join(s.root, "git")
	g := newGitRepo(t, dir)
	first := g.commit(map[string]string{"skills/go-testing/SKILL.md": skillMD("go-testing", ""), "skills/go-testing/a.md": "1"})
	g.tag("go-testing/v1.0.0", first)
	second := g.commit(map[string]string{"skills/go-testing/a.md": "2"})
	g.tag("go-testing/v1.1.0", second)
	g.tag("unrelated", second)
	id := idOf(s.source(obj{"kind": "local_git", "path": dir, "root": "skills", "versions": "tags", "tag_pattern": "{name}/v{version}",
		"working_tree": false}))
	got := s.runSource(id)
	eq(t, at(got, "last_run", "status"), any("succeeded"), jsonText(got))
	skill := idOf(at(s.skills(), "items", 0).(map[string]any))
	versions := s.get(skillPath(skill)+"/versions", s.admin).json(t)
	eq(t, fieldsOf(t, versions, "version"), "1.1.0,1.0.0")
	eq(t, at(versions, "items", 0, "tag"), any("go-testing/v1.1.0"))
	eq(t, at(versions, "items", 0, "commit"), any(second))

	third := g.commit(map[string]string{"skills/go-testing/a.md": "3"})
	g.tag("go-testing/v1.2.0", third)
	got = s.runSource(id)
	eq(t, at(got, "last_run", "updated"), any(float64(1)), jsonText(got))
	eq(t, fieldsOf(t, s.get(skillPath(skill)+"/versions", s.admin).json(t), "version"), "1.2.0,1.1.0,1.0.0")

	r := s.send("POST", skillsPath+"/folders/"+s.folder+"/sources", s.admin, obj{"kind": "local_git", "path": dir, "versions": "tags",
		"tag_pattern": "release"})
	eq(t, code(t, r), any("validation.invalid_tag_pattern"))
	r = s.send("POST", skillsPath+"/folders/"+s.folder+"/sources", s.admin, obj{"kind": "local_dir", "path": dir, "versions": "tags"})
	eq(t, code(t, r), any("validation.invalid_skill_source"))
}

func TestSkillSourceSecretsAndJob(t *testing.T) {
	t.Parallel()
	s := startSkillSync(t)
	global := idOf(s.skillFolder(s.admin, obj{"slug": "global", "name": "Global"}))
	acme := scalar[string](t, s.db, "SELECT id::text FROM nodes WHERE slug = 'acme'")
	secret := refSecret(t, s.db, acme)
	r := s.send("POST", skillsPath+"/folders/"+global+"/sources", s.admin, obj{"kind": "remote", "hosting": "gitlab",
		"url": "https://gitlab.example.com/platform/agent-skills", "credentials": obj{"secret_id": secret}})
	eq(t, r.status, 400, r.text())
	eq(t, code(t, r), any("validation.secret_not_available"))
	src := s.source(obj{"kind": "remote", "hosting": "gitlab", "url": "https://gitlab.example.com/platform/agent-skills", "root": "skills",
		"credentials": obj{"secret_id": secret}})
	eq(t, src["api_url"], any("https://gitlab.example.com/api/v4"))
	eq(t, at(src, "credentials", "secret_id"), any(secret))
	r = s.call("DELETE", "/api/v1/catalog/nodes/"+acme+"/secrets/"+secret, s.admin)
	eq(t, code(t, r), any("conflict.secret_in_use"))

	dir := filepath.Join(s.root, "job")
	writeFile(t, filepath.Join(dir, "a/SKILL.md"), skillMD("job-skill", ""))
	s.source(obj{"kind": "local_dir", "path": dir})
	jobs.SkillSourcesTick = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnSkillSources(ctx, s.services.Jobs())
	deadline := time.Now().Add(20 * time.Second)
	for s.count("skills") == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	<-done
	eq(t, s.count("skills"), int64(1))
}

func (g *gitRepo) tag(name, commit string) {
	g.t.Helper()
	if _, err := g.repo.CreateTag(name, plumbing.NewHash(commit), nil); err != nil {
		g.t.Fatal(err)
	}
}
