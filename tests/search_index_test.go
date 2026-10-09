package tests

import (
	"net/http"
	"strconv"
	"testing"

	"svc-registry/internal/embeddings"
)

func TestIndexEmbedsContentOnceAndFollowsTheModel(t *testing.T) {
	t.Parallel()
	s := startSearch(t, "pgvector")
	same := "# Rollback\nHow to revert a release safely."
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry.", "docs/rollback.md": same, "docs/copy.md": same})
	s.collect()
	eq(t, s.index() >= 1, true)
	eq(t, s.embed.Calls(same), 1, "the same content is embedded once")
	eq(t, scalar[int64](t, s.db, "SELECT count(DISTINCT sha256) FROM knowledge_embeddings WHERE model = 'fake-1'"), int64(2))
	before := s.embed.Total()
	s.index()
	eq(t, s.embed.Total(), before, "nothing new, nothing embedded")

	s.state.Embedder = embeddings.New(s.embed.URL, "", "fake-2", embedDims, 8, http.DefaultClient)
	s.index()
	eq(t, scalar[int64](t, s.db, "SELECT count(DISTINCT sha256) FROM knowledge_embeddings WHERE model = 'fake-2'"), int64(2), "a new model reindexes")
}

func TestIndexFailureLeavesFilesPending(t *testing.T) {
	t.Parallel()
	s := startSearch(t, "pgvector")
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry."})
	s.collect()
	s.embed.Break(true)
	s.index()
	eq(t, scalar[int64](t, s.db, "SELECT count(*) FROM knowledge_embeddings"), int64(0))
	eq(t, scalar[string](t, s.db, "SELECT COALESCE(failure, '') FROM knowledge_index_state"), "search.embeddings_unavailable")
	idx := s.scans(s.project, "index")
	eq(t, idx[0].Status, "failed")
	eq(t, idx[0].Code, "search.embeddings_unavailable")
	s.embed.Break(false)
	s.index()
	eq(t, scalar[int64](t, s.db, "SELECT count(*) FROM knowledge_embeddings") > 0, true)
	eq(t, scalar[string](t, s.db, "SELECT COALESCE(failure, '') FROM knowledge_index_state"), "")
}

func TestIndexScansNameDimensionMismatchesAndCollapseIdlePasses(t *testing.T) {
	t.Parallel()
	s := startSearch(t, "pgvector")
	s.docs(map[string]string{"README.md": "# Registry\nThe service registry."})
	s.collect()
	s.state.Embedder = embeddings.New(s.embed.URL, "", "fake-1", embedDims*2, 8, http.DefaultClient)
	s.index()
	idx := s.scans(s.project, "index")
	eq(t, idx[0].Status, "failed")
	eq(t, idx[0].Code, "search.embeddings_dimensions")
	detail := scalar[string](t, s.db, "SELECT error_detail FROM knowledge_scans WHERE kind = 'index' ORDER BY started_at DESC LIMIT 1")
	contains(t, detail, strconv.Itoa(embedDims))
	contains(t, detail, strconv.Itoa(embedDims*2))

	s.state.Embedder = embeddings.New(s.embed.URL, "", "fake-1", embedDims, 8, http.DefaultClient)
	s.index()
	eq(t, s.scans(s.project, "index")[0].Status, "ok")
	s.index()
	s.index()
	s.index()
	idx = s.scans(s.project, "index")
	eq(t, len(idx), 3, idx)
	eq(t, idx[0].Status, "unchanged")
	eq(t, idx[0].Repeats, 2)
	contains(t, scalar[string](t, s.db, "SELECT index::text FROM knowledge_scans WHERE kind = 'index' ORDER BY started_at DESC LIMIT 1 OFFSET 1"), `"engine": "pgvector"`)
}
