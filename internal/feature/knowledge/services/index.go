package services

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

	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

func (s *IndexService) ClaimIndex(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.indexRepository.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *IndexService) ExtendIndex(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.indexRepository.Extend(ctx, id, leaseSecs))
}

func (s *IndexService) RunKnowledgeIndex(ctx context.Context, id uuid.UUID) error {
	next := s.cfg.Search.IndexIntervalSecs
	scan := &knowledgetypes.Scan{ID: uuid.Must(uuid.NewV7()), ProjectID: id, Kind: knowledgetypes.IndexScan,
		Trigger: knowledgetypes.TriggerSchedule, StartedAt: time.Now()}
	var st indexStats
	err := s.indexProject(ctx, id, &st)
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
	}
	scan.Index = &knowledgetypes.ScanIndex{EmbeddedFiles: st.embedded, EmbeddedChunks: st.chunks, Documents: st.documents,
		Synced: st.synced, Engine: s.search.engine().Name(), Model: model}
	switch {
	case err != nil:
	case st.embedded > 0 || st.synced:
		slog.Info("documentation indexed", "project", id, "engine", s.search.engine().Name(), "embedded_files", st.embedded,
			"embedded_chunks", st.chunks, "synced", st.synced, "documents", st.documents)
	default:
		slog.Debug("documentation index: nothing to do", "project", id)
	}
	failure := ""
	if err != nil {
		failure = knowledgetypes.IndexFailureCode(err)
		scan.Fail(failure, err)
	}
	if !errors.Is(err, context.Canceled) {
		s.collect.recordScan(ctx, scan)
	}
	if ctx.Err() == nil {
		if rerr := s.indexRepository.Release(context.WithoutCancel(ctx), id, next, failure); rerr != nil && err == nil {
			err = rerr
		}
	}
	if err == nil {
		return nil
	}
	return knowledgetypes.NewIndexFailure(err)
}

func (s *IndexService) RetainExternalIndex(ctx context.Context) error {
	if s.external == nil {
		return nil
	}
	keep, err := s.indexRepository.Indexed(ctx)
	if err != nil {
		return apperr.Wrap(err)
	}
	if err := s.external.Retain(ctx, keep); err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.indexRepository.Forget(ctx, keep))
}

type indexStats struct {
	embedded, chunks, documents int
	synced                      bool
}

func (s *IndexService) indexProject(ctx context.Context, id uuid.UUID, st *indexStats) error {
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
	files, err := s.indexRepository.Files(ctx, id)
	if err != nil {
		return err
	}
	want := knowledgetypes.IndexState{Engine: s.search.engine().Name() + " " + s.external.Target(), Model: model, Fingerprint: fingerprint(files)}
	if have, err := s.indexRepository.State(ctx, id); err != nil {
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
	return s.indexRepository.Synced(ctx, id, want)
}

func (s *IndexService) embedMissing(ctx context.Context, id uuid.UUID, model string, st *indexStats) error {
	missing, err := s.indexRepository.Missing(ctx, id, model)
	if err != nil {
		return err
	}
	for _, blob := range missing {
		spans := knowledgetypes.Chunk(blob.Content, s.cfg.Search.ChunkChars)
		texts := make([]string, len(spans))
		for i, sc := range spans {
			texts[i] = knowledgetypes.Slice(blob.Content, sc)
		}
		var vectors [][]float32
		if len(texts) > 0 {
			if vectors, err = s.embedder.Embed(ctx, texts); err != nil {
				return err
			}
		}
		chunks := make([]knowledgetypes.StoredChunk, len(spans))
		for i, sc := range spans {
			chunks[i] = knowledgetypes.StoredChunk{Ord: i, Span: sc, Vector: vectors[i]}
		}
		if err := repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
			return s.indexRepository.ReplaceEmbeddings(ctx, blob.SHA256, model, chunks)
		})); err != nil {
			return err
		}
		st.embedded++
		st.chunks += len(chunks)
	}
	return nil
}

func fingerprint(files []knowledgetypes.IndexFile) string {
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

func (s *IndexService) indexDocs(ctx context.Context, id uuid.UUID, files []knowledgetypes.IndexFile, model string) ([]knowledgetypes.IndexDoc, error) {
	seen := map[string]bool{}
	var shas [][]byte
	for _, f := range files {
		if !seen[string(f.SHA256)] {
			seen[string(f.SHA256)] = true
			shas = append(shas, f.SHA256)
		}
	}
	contents, err := s.indexRepository.Contents(ctx, shas)
	if err != nil {
		return nil, err
	}
	chunks := map[string][]knowledgetypes.StoredChunk{}
	if model != "" {
		if chunks, err = s.indexRepository.Chunks(ctx, shas, model); err != nil {
			return nil, err
		}
	} else {
		for sha, content := range contents {
			for i, sc := range knowledgetypes.Chunk(content, s.cfg.Search.ChunkChars) {
				chunks[sha] = append(chunks[sha], knowledgetypes.StoredChunk{Ord: i, Span: sc})
			}
		}
	}
	names, err := s.searchRepository.Projects(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	name := names[id]
	var docs []knowledgetypes.IndexDoc
	for _, f := range files {
		content := contents[string(f.SHA256)]
		for _, c := range chunks[string(f.SHA256)] {
			docs = append(docs, knowledgetypes.IndexDoc{ProjectID: id, ProjectPath: name.Path, ProjectName: name.Name, Branch: f.Branch,
				Commit: f.Commit, Path: f.Path, Kind: f.Kind, Ord: c.Ord, Text: knowledgetypes.Slice(content, c.Span), Vector: c.Vector})
		}
	}
	return docs, nil
}

type IndexService struct {
	cfg              *config.Config
	db               *pgtx.DB
	indexRepository  *repositories.IndexRepository
	searchRepository *repositories.SearchRepository
	embedder         knowledgeports.EmbedderPort
	external         knowledgeports.ExternalIndexPort
	collect          *CollectService
	search           *SearchService
}
