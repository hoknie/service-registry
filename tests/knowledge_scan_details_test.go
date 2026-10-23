package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func lastScan(t *testing.T, a *sourcesApp, project string) map[string]any {
	t.Helper()
	r := a.get(projectScansPath(project)+"?kind=collect", a.admin)
	eq(t, r.status, 200, r.text())
	items := r.json(t)["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no scans")
	}
	return items[0].(map[string]any)
}

func TestScanDetailsNameSkippedFiles(t *testing.T) {
	t.Parallel()
	a := startSources(t, "KNOWLEDGE_MAX_FILE_BYTES", "1024")
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	writeFile(t, filepath.Join(dir, "big.md"), strings.Repeat("x", 2048))
	writeFile(t, filepath.Join(dir, "bin.md"), "a\x00b")
	a.useSource(a.project, "local_dir", dir, "**/*.md")
	a.collect()
	s := lastScan(t, a, a.project)
	eq(t, fmt.Sprint(s["warnings"]), "[collect.files_skipped]")
	eq(t, at(s, "warning_counts", "collect.files_skipped"), any(float64(2)))
	details := s["warning_details"].([]any)
	eq(t, len(details), 2)
	reasons := map[string]any{}
	for _, d := range details {
		m := d.(map[string]any)
		eq(t, m["code"], any("collect.files_skipped"))
		eq(t, m["branch"], any("local"))
		eq(t, m["detail"], nil)
		reasons[m["path"].(string)] = m["reason"]
	}
	eq(t, reasons["big.md"], any("too_large"))
	eq(t, reasons["bin.md"], any("binary"))

	all := a.get(scansPath+"?project="+a.project, a.admin)
	eq(t, all.status, 200)
	eq(t, len(at(all.json(t), "items", 0, "warning_details").([]any)), 2)
}

func TestScanDetailsAreCappedButCounted(t *testing.T) {
	t.Parallel()
	a := startSources(t, "KNOWLEDGE_MAX_FILES", "3")
	dir := filepath.Join(a.root, "docs")
	for i := range 40 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("f%02d.md", i)), "# Doc")
	}
	a.useSource(a.project, "local_dir", dir, "**/*.md")
	a.collect()
	s := lastScan(t, a, a.project)
	eq(t, at(s, "warning_counts", "collect.files_skipped"), any(float64(37)))
	details := s["warning_details"].([]any)
	eq(t, len(details), 20)
	eq(t, at(details[0], "reason"), any("limit"))
}

func TestScanDetailsExplainAnInvalidManifest(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	writeFile(t, filepath.Join(dir, ".svc-registry.yaml"), "dependencies: 42\n")
	a.useSource(a.project, "local_dir", dir, "**/*.md")
	a.collect()
	s := lastScan(t, a, a.project)
	details := s["warning_details"].([]any)
	eq(t, len(details), 1, fmt.Sprint(details))
	eq(t, at(details[0], "code"), any("collect.manifest_invalid"))
	eq(t, at(details[0], "path"), any(".svc-registry.yaml"))
	eq(t, at(details[0], "branch"), any("local"))
	eq(t, at(details[0], "detail"), any("dependencies is not a list"))
	eq(t, at(s, "warning_counts", "collect.manifest_invalid"), any(float64(1)))
}
