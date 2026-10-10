package tests

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport/embedfake"
)

const embedDims = 64

type searchApp struct {
	*sourcesApp
	tt    *testing.T
	embed *embedfake.Fake
	dir   string
}

func startSearch(t *testing.T, engine string, extra ...string) *searchApp {
	t.Helper()
	f := embedfake.Start(t, embedDims)
	env := append([]string{"KNOWLEDGE_SEARCH_ENGINE", engine, "EMBEDDINGS_URL", f.URL, "EMBEDDINGS_MODEL", "fake-1",
		"EMBEDDINGS_DIMENSIONS", strconv.Itoa(embedDims)}, extra...)
	a := startSources(t, append(engineEnv(t, engine), env...)...)
	return &searchApp{sourcesApp: a, tt: t, embed: f, dir: filepath.Join(a.root, "docs")}
}

func startTextSearch(t *testing.T, engine string) *searchApp {
	t.Helper()
	a := startSources(t, append([]string{"KNOWLEDGE_SEARCH_ENGINE", engine}, engineEnv(t, engine)...)...)
	return &searchApp{sourcesApp: a, tt: t, dir: filepath.Join(a.root, "docs")}
}

func engineEnv(t *testing.T, engine string) []string {
	t.Helper()
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	switch engine {
	case "qdrant":
		addr := os.Getenv("TEST_QDRANT_URL")
		if addr == "" {
			t.Skip("TEST_QDRANT_URL is not set; `just test` sets it")
		}
		return []string{"QDRANT_URL", addr, "QDRANT_COLLECTION", name}
	case "meilisearch":
		addr := os.Getenv("TEST_MEILISEARCH_URL")
		if addr == "" {
			t.Skip("TEST_MEILISEARCH_URL is not set; `just test` sets it")
		}
		return []string{"MEILISEARCH_URL", addr, "MEILISEARCH_INDEX", name}
	}
	return nil
}

func (s *searchApp) docs(files map[string]string) {
	s.tt.Helper()
	s.docsOf(s.project, s.dir, files)
}

func (s *searchApp) docsOf(project, dir string, files map[string]string) {
	s.tt.Helper()
	for p, c := range files {
		writeFile(s.tt, filepath.Join(dir, p), c)
	}
	eq(s.tt, s.send("PUT", sourcePath(project), s.admin, obj{"kind": "local_dir", "path": dir}).status, 200)
	eq(s.tt, s.send("PUT", settingsPath(project), s.admin, obj{"include": []string{"**/*.md"}}).status, 200)
}

func (a *testApp) index() int {
	a.t.Helper()
	ctx := context.Background()
	execSQL(a.t, a.db, "UPDATE knowledge_index_state SET next_run_at = now()")
	claimed, err := a.services.Knowledge.ClaimIndex(ctx, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, id := range claimed {
		_ = a.services.Knowledge.RunKnowledgeIndex(ctx, id)
	}
	return len(claimed)
}
