package tests

import (
	"path/filepath"
	"strings"
	"testing"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/pkg/testing/forgefake"
)

func edgesOf(t *testing.T, a *testApp, project, source string) string {
	t.Helper()
	return scalar[string](t, a.db, `
		SELECT COALESCE(string_agg(COALESCE(target_external, target_ref), ',' ORDER BY target_ref), '')
		FROM service_dependencies
		WHERE project_id = $1
			AND source = $2`, project, source)
}

func TestDependenciesFromClusterAnnotations(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	folder := app.nodeID(root, "folder", org, "backend")
	api := app.nodeID(root, "project", folder, "api")
	app.nodeID(root, "project", folder, "billing")
	c := app.cluster(root, "prod", nil)
	eq(t, app.send("POST", depsPath(api), root, obj{"target": "external:manual"}).status, 201)

	w := workload("u1", "api", "1", map[string]string{deploytypes.AnnotationProject: "acme/backend/api",
		deploytypes.AnnotationDependsOn: " acme/backend/billing , external:kafka,, external: "})
	k8s.Cluster().Put(w)
	app.pollNow()
	eq(t, at(app.get(clusterPath(idOf(c)), root).json(t), "status"), any("ok"))
	eq(t, edgesOf(t, app, api, "cluster"), "acme/backend/billing,kafka")
	contains(t, scalar[string](t, app.db, "SELECT origin FROM service_dependencies WHERE source = 'cluster' LIMIT 1"), idOf(c)+"/backend/Deployment/api")

	w.Annotations = map[string]string{deploytypes.AnnotationProject: "acme/backend/api"}
	w.TemplateAnnotations = map[string]string{deploytypes.AnnotationDependsOn: "external:redis"}
	k8s.Cluster().Put(w)
	app.pollNow()
	eq(t, edgesOf(t, app, api, "cluster"), "redis")

	delete(w.TemplateAnnotations, deploytypes.AnnotationDependsOn)
	k8s.Cluster().Put(w)
	app.pollNow()
	eq(t, edgesOf(t, app, api, "cluster"), "")
	eq(t, edgesOf(t, app, api, "manual"), "manual")

	w.TemplateAnnotations[deploytypes.AnnotationDependsOn] = "external:redis"
	k8s.Cluster().Put(w)
	app.pollNow()
	eq(t, edgesOf(t, app, api, "cluster"), "redis")
	k8s.Cluster().Put()
	app.pollNow()
	eq(t, edgesOf(t, app, api, "cluster"), "", "gone workload")

	k8s.Cluster().Put(w)
	app.pollNow()
	eq(t, app.send("DELETE", clusterPath(idOf(c)), root, nil).status, 204)
	eq(t, edgesOf(t, app, api, "cluster"), "", "cluster deleted")
}

func TestDependenciesFromAgentReports(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	api := app.nodeID(root, "project", org, "api")
	c, token := app.agentCluster(root, "edge", nil)
	r := app.agentReport(idOf(c), token, agentReportOf(reportWorkload("u1", "api", "1",
		map[string]string{deploytypes.AnnotationProject: "acme/api", deploytypes.AnnotationDependsOn: "external:kafka"})))
	eq(t, r.status < 300, true, r.text())
	eq(t, edgesOf(t, app, api, "cluster"), "kafka")
}

func TestDependenciesFromTheProjectManifest(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# API",
		".svc-registry.yaml": "owners: [x]\ndependencies:\n  - acme/billing\n  - \"external:s3\"\n"})
	d.app.nodeID(d.admin, "project", d.org, "billing")
	d.app.collect()
	eq(t, d.files("main"), "README.md:")
	eq(t, edgesOf(t, d.app, d.id, "repository"), "acme/billing,s3")
	eq(t, scalar[string](t, d.app.db, "SELECT DISTINCT origin FROM service_dependencies WHERE source = 'repository'"), "main")

	d.push(map[string]string{"README.md": "# API", ".svc-registry.yaml": "dependencies: 42\n"},
		forgefake.Branch{Name: "main", SHA: sha(2)})
	d.app.collect()
	eq(t, edgesOf(t, d.app, d.id, "repository"), "acme/billing,s3", "broken file keeps edges")
	eq(t, scalar[string](t, d.app.db, "SELECT array_to_string(warnings, ',') FROM knowledge_scans WHERE project_id = $1 AND kind = 'collect' ORDER BY last_started_at DESC LIMIT 1", d.id), "collect.manifest_invalid")

	d.push(map[string]string{"README.md": "# API", ".svc-registry.yml": "dependencies: [\"external:s3\", 7]\n"},
		forgefake.Branch{Name: "main", SHA: sha(3)})
	d.app.collect()
	eq(t, edgesOf(t, d.app, d.id, "repository"), "s3")

	d.push(map[string]string{"README.md": "# API", ".svc-registry.yaml": strings.Repeat("#", 70<<10)},
		forgefake.Branch{Name: "main", SHA: sha(4)})
	d.app.collect()
	eq(t, edgesOf(t, d.app, d.id, "repository"), "s3", "too large")

	d.push(map[string]string{"README.md": "# API"}, forgefake.Branch{Name: "main", SHA: sha(5)})
	d.app.collect()
	eq(t, edgesOf(t, d.app, d.id, "repository"), "")
}

func TestDependenciesFromALocalDirectoryManifest(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	writeFile(t, filepath.Join(dir, ".svc-registry.yaml"), "dependencies: [\"external:kafka\"]\n")
	a.useSource(a.project, "local_dir", dir, "**/*.md")
	a.collect()
	eq(t, edgesOf(t, a.testApp, a.project, "repository"), "kafka")
	eq(t, scalar[string](t, a.db, "SELECT origin FROM service_dependencies WHERE source = 'repository'"), "local")
}
