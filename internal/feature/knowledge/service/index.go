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

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ClaimIndex(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.docIndex.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *Service) ExtendIndex(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.docIndex.Extend(ctx, id, leaseSecs))
}

func (s *Service) RunKnowledgeIndex(ctx context.Context, id uuid.UUID) error {
	next := s.cfg.Search.IndexIntervalSecs
	scan := &knowledge.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledge.IndexScan,
		Trigger: knowledge.TriggerSchedule, StartedAt: time.Now()}
	var st indexStats
	err := s.indexProject(ctx, id, &st)
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
	}
	scan.Index = &knowledge.ScanIndex{EmbeddedFiles: st.embedded, EmbeddedChunks: st.chunks, Documents: st.documents,
		Synced: st.synced, Engine: s.engine().Name(), Model: model}
	switch {
	case err != nil:
	case st.embedded > 0 || st.synced:
		slog.Info("documentation indexed", "project", id, "engine", s.engine().Name(), "embedded_files", st.embedded,
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
		s.recordScan(ctx, scan)
	}
	if ctx.Err() == nil {
		if rerr := s.docIndex.Release(context.WithoutCancel(ctx), id, next, failure); rerr != nil && err == nil {
			err = rerr
		}
	}
	if err == nil {
		return nil
	}
	return knowledge.NewIndexFailure(err)
}

func (s *Service) RetainExternalIndex(ctx context.Context) error {
	if s.external == nil {
		return nil
	}
	keep, err := s.docIndex.Indexed(ctx)
	if err != nil {
		return apperr.Wrap(err)
	}
	if err := s.external.Retain(ctx, keep); err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.docIndex.Forget(ctx, keep))
}

type indexStats struct {
	embedded, chunks, documents int
	synced                      bool
}

func (s *Service) indexProject(ctx context.Context, id uuid.UUID, st *indexStats) error {
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
		if err := s.embedMissing(ctx, id, model, st); err != nil {
			return err
		}
	}
	if s.external == nil {
		return nil
	}
	files, err := s.docIndex.Files(ctx, id)
	if err != nil {
		return err
	}
	want := knowledge.IndexState{Engine: s.engine().Name() + " " + s.external.Target(), Model: model, Fingerprint: fingerprint(files)}
	if have, err := s.docIndex.State(ctx, id); err != nil {
		return err
	} else if have == want {
		return nil
	}
	docs, err := s.indexDocs(ctx, id, files, model)
	if err != nil {
		return err
	}
	if err := s.external.Sync(ctx, id, docs); err != nil {
		return err
	}
	st.synced, st.documents = true, len(docs)
	return s.docIndex.Synced(ctx, id, want)
}

func (s *Service) embedMissing(ctx context.Context, id uuid.UUID, model string, st *indexStats) error {
	missing, err := s.docIndex.Missing(ctx, id, model)
	if err != nil {
		return err
	}
	for _, blob := range missing {
		spans := knowledge.Chunk(blob.Content, s.cfg.Search.ChunkChars)
		texts := make([]string, len(spans))
		for i, sc := range spans {
			texts[i] = knowledge.Slice(blob.Content, sc)
		}
		var vectors [][]float32
		if len(texts) > 0 {
			if vectors, err = s.embedder.Embed(ctx, texts); err != nil {
				return err
			}
		}
		chunks := make([]knowledge.StoredChunk, len(spans))
		for i, sc := range spans {
			chunks[i] = knowledge.StoredChunk{Ord: i, Span: sc, Vector: vectors[i]}
		}
		if err := s.docIndex.Save(ctx, blob.SHA256, model, chunks); err != nil {
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

func (s *Service) indexDocs(ctx context.Context, id uuid.UUID, files []knowledge.IndexFile, model string) ([]knowledge.IndexDoc, error) {
	seen := map[string]bool{}
	var shas [][]byte
	for _, f := range files {
		if !seen[string(f.SHA256)] {
			seen[string(f.SHA256)] = true
			shas = append(shas, f.SHA256)
		}
	}
	contents, err := s.docIndex.Contents(ctx, shas)
	if err != nil {
		return nil, err
	}
	chunks := map[string][]knowledge.StoredChunk{}
	if model != "" {
		if chunks, err = s.docIndex.Chunks(ctx, shas, model); err != nil {
			return nil, err
		}
	} else {
		for sha, content := range contents {
			for i, sc := range knowledge.Chunk(content, s.cfg.Search.ChunkChars) {
				chunks[sha] = append(chunks[sha], knowledge.StoredChunk{Ord: i, Span: sc})
			}
		}
	}
	names, err := s.docSearch.Projects(ctx, []uuid.UUID{id})
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
