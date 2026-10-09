package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/knowledge"
)

func ClaimIndex(ctx context.Context, state *State, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := state.DocIndex.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func ExtendIndex(ctx context.Context, state *State, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(state.DocIndex.Extend(ctx, id, leaseSecs))
}

func RunKnowledgeIndex(ctx context.Context, state *State, id uuid.UUID) error {
	next := state.Config.Search.IndexIntervalSecs
	scan := &knowledge.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledge.IndexScan,
		Trigger: knowledge.TriggerSchedule, StartedAt: time.Now()}
	var st indexStats
	err := indexProject(ctx, state, id, &st)
	model := ""
	if state.Embedder != nil {
		model = state.Embedder.Model()
	}
	scan.Index = &knowledge.ScanIndex{EmbeddedFiles: st.embedded, EmbeddedChunks: st.chunks, Documents: st.documents,
		Synced: st.synced, Engine: engine(state).Name(), Model: model}
	switch {
	case err != nil:
	case st.embedded > 0 || st.synced:
		slog.Info("documentation indexed", "project", id, "engine", engine(state).Name(), "embedded_files", st.embedded,
			"embedded_chunks", st.chunks, "synced", st.synced, "documents", st.documents)
	default:
		slog.Debug("documentation index: nothing to do", "project", id)
	}
	failure := ""
	if err != nil {
		failure = knowledge.IndexFailureCode(err)
		scan.Fail(failure, err)
	}
	if !errors.Is(err, context.Canceled) {
		recordScan(ctx, state, scan)
	}
	if ctx.Err() == nil {
		if rerr := state.DocIndex.Release(context.WithoutCancel(ctx), id, next, failure); rerr != nil && err == nil {
			err = rerr
		}
	}
	return apperr.Wrap(err)
}

func RetainExternalIndex(ctx context.Context, state *State) error {
	if state.ExternalIndex == nil {
		return nil
	}
	keep, err := state.DocIndex.Indexed(ctx)
	if err != nil {
		return apperr.Wrap(err)
	}
	if err := state.ExternalIndex.Retain(ctx, keep); err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(state.DocIndex.Forget(ctx, keep))
}

type indexStats struct {
	embedded, chunks, documents int
	synced                      bool
}

func indexProject(ctx context.Context, state *State, id uuid.UUID, st *indexStats) error {
	model := ""
	if state.Embedder != nil {
		model = state.Embedder.Model()
		if err := embedMissing(ctx, state, id, model, st); err != nil {
			return err
		}
	}
	if state.ExternalIndex == nil {
		return nil
	}
	files, err := state.DocIndex.Files(ctx, id)
	if err != nil {
		return err
	}
	want := knowledge.IndexState{Engine: engine(state).Name() + " " + state.ExternalIndex.Target(), Model: model, Fingerprint: fingerprint(files)}
	if have, err := state.DocIndex.State(ctx, id); err != nil {
		return err
	} else if have == want {
		return nil
	}
	docs, err := indexDocs(ctx, state, id, files, model)
	if err != nil {
		return err
	}
	if err := state.ExternalIndex.Sync(ctx, id, docs); err != nil {
		return err
	}
	st.synced, st.documents = true, len(docs)
	return state.DocIndex.Synced(ctx, id, want)
}

func embedMissing(ctx context.Context, state *State, id uuid.UUID, model string, st *indexStats) error {
	missing, err := state.DocIndex.Missing(ctx, id, model)
	if err != nil {
		return err
	}
	for _, blob := range missing {
		spans := knowledge.Chunk(blob.Content, state.Config.Search.ChunkChars)
		texts := make([]string, len(spans))
		for i, s := range spans {
			texts[i] = knowledge.Slice(blob.Content, s)
		}
		var vectors [][]float32
		if len(texts) > 0 {
			if vectors, err = state.Embedder.Embed(ctx, texts); err != nil {
				return err
			}
		}
		chunks := make([]knowledge.StoredChunk, len(spans))
		for i, s := range spans {
			chunks[i] = knowledge.StoredChunk{Ord: i, Span: s, Vector: vectors[i]}
		}
		if err := state.DocIndex.Save(ctx, blob.SHA256, model, chunks); err != nil {
			return err
		}
		st.embedded++
		st.chunks += len(chunks)
	}
	return nil
}

func fingerprint(files []knowledge.IndexFile) string {
	keys := make([]string, len(files))
	for i, f := range files {
		keys[i] = fmt.Sprintf("%s\x00%s\x00%s\x00%x", f.Branch, f.Commit, f.Path, f.SHA256)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func indexDocs(ctx context.Context, state *State, id uuid.UUID, files []knowledge.IndexFile, model string) ([]knowledge.IndexDoc, error) {
	seen := map[string]bool{}
	var shas [][]byte
	for _, f := range files {
		if !seen[string(f.SHA256)] {
			seen[string(f.SHA256)] = true
			shas = append(shas, f.SHA256)
		}
	}
	contents, err := state.DocIndex.Contents(ctx, shas)
	if err != nil {
		return nil, err
	}
	chunks := map[string][]knowledge.StoredChunk{}
	if model != "" {
		if chunks, err = state.DocIndex.Chunks(ctx, shas, model); err != nil {
			return nil, err
		}
	} else {
		for sha, content := range contents {
			for i, s := range knowledge.Chunk(content, state.Config.Search.ChunkChars) {
				chunks[sha] = append(chunks[sha], knowledge.StoredChunk{Ord: i, Span: s})
			}
		}
	}
	names, err := state.DocSearch.Projects(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	name := names[id]
	var docs []knowledge.IndexDoc
	for _, f := range files {
		content := contents[string(f.SHA256)]
		for _, c := range chunks[string(f.SHA256)] {
			docs = append(docs, knowledge.IndexDoc{ProjectID: id, ProjectPath: name.Path, ProjectName: name.Name, Branch: f.Branch,
				Commit: f.Commit, Path: f.Path, Kind: f.Kind, Ord: c.Ord, Text: knowledge.Slice(content, c.Span), Vector: c.Vector})
		}
	}
	return docs, nil
}
