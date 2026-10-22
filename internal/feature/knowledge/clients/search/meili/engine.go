package meili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/meilisearch/meilisearch-go"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

var docSpace = uuid.MustParse("0b8f9d6e-5a3c-4f47-8e21-7c4d2b9a1e53")

const (
	embedder  = "default"
	markStart = ""
	markStop  = ""
)

type EngineClient struct {
	url    string
	client meilisearch.ServiceManager
	index  string
	dims   int
	mu     sync.Mutex
	ready  bool
}

func New(url, apiKey, index string, dims int, hc *http.Client) *EngineClient {
	opts := []meilisearch.Option{meilisearch.WithCustomClient(hc)}
	if apiKey != "" {
		opts = append(opts, meilisearch.WithAPIKey(apiKey))
	}
	return &EngineClient{url: url, client: meilisearch.New(url, opts...), index: index, dims: dims}
}

func (e *EngineClient) Name() string { return "meilisearch" }

func (e *EngineClient) Modes() []knowledgetypes.Mode {
	if e.dims == 0 {
		return []knowledgetypes.Mode{knowledgetypes.ModeText}
	}
	return []knowledgetypes.Mode{knowledgetypes.ModeText, knowledgetypes.ModeSemantic, knowledgetypes.ModeHybrid}
}

func (e *EngineClient) DefaultMode() knowledgetypes.Mode {
	if e.dims == 0 {
		return knowledgetypes.ModeText
	}
	return knowledgetypes.ModeHybrid
}

func (e *EngineClient) Handles(knowledgetypes.Mode) bool { return true }

func unavailable(op string, err error) error {
	return fmt.Errorf("%w: meilisearch %s: %v", knowledgetypes.ErrEngineUnavailable, op, err)
}

func (e *EngineClient) wait(ctx context.Context, op string, info *meilisearch.TaskInfo, err error) error {
	if err != nil {
		return unavailable(op, err)
	}
	task, err := e.client.WaitForTaskWithContext(ctx, info.TaskUID, 50*time.Millisecond)
	if err != nil {
		return unavailable(op, err)
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		return unavailable(op, fmt.Errorf("task %d %s: %s", task.UID, task.Status, task.Error.Message))
	}
	return nil
}

func (e *EngineClient) prepare(ctx context.Context) (meilisearch.IndexManager, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	idx := e.client.Index(e.index)
	if e.ready {
		return idx, nil
	}
	if _, err := e.client.GetIndexWithContext(ctx, e.index); err != nil {
		info, err := e.client.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{Uid: e.index, PrimaryKey: "id"})
		if err := e.wait(ctx, "create index", info, err); err != nil && !strings.Contains(err.Error(), "index_already_exists") {
			return nil, err
		}
	}
	settings := &meilisearch.Settings{SearchableAttributes: []string{"text", "path"}, DistinctAttribute: new("file"),
		FilterableAttributes: []string{"project_id", "pb", "file"}}
	if e.dims > 0 {
		settings.Embedders = map[string]meilisearch.Embedder{embedder: {Source: meilisearch.UserProvidedEmbedderSource, Dimensions: e.dims}}
	}
	info, err := idx.UpdateSettingsWithContext(ctx, settings)
	if err := e.wait(ctx, "settings", info, err); err != nil {
		return nil, err
	}
	e.ready = true
	return idx, nil
}

type document struct {
	ID          string               `json:"id"`
	ProjectID   string               `json:"project_id"`
	PB          string               `json:"pb"`
	File        string               `json:"file"`
	ProjectPath string               `json:"project_path"`
	ProjectName string               `json:"project_name"`
	Branch      string               `json:"branch"`
	Commit      string               `json:"commit"`
	Path        string               `json:"path"`
	Kind        string               `json:"kind"`
	Ord         int                  `json:"ord"`
	Text        string               `json:"text"`
	Vectors     map[string][]float32 `json:"_vectors,omitempty"`
}

func quote(s string) string { return strconv.Quote(s) }

func (e *EngineClient) Sync(ctx context.Context, project uuid.UUID, docs []knowledgetypes.IndexDoc) error {
	idx, err := e.prepare(ctx)
	if err != nil {
		return err
	}
	info, err := idx.DeleteDocumentsByFilterWithContext(ctx, "project_id = "+quote(project.String()), nil)
	if err := e.wait(ctx, "delete", info, err); err != nil {
		return err
	}
	const batch = 500
	for start := 0; start < len(docs); start += batch {
		part := docs[start:min(len(docs), start+batch)]
		out := make([]document, len(part))
		for i, d := range part {
			p := d.ProjectID.String()
			out[i] = document{ID: uuid.NewSHA1(docSpace, []byte(fmt.Sprintf("%s|%s|%s|%d", p, d.Branch, d.Path, d.Ord))).String(),
				ProjectID: p, PB: p + "|" + d.Branch, File: p + "|" + d.Branch + "|" + d.Path, ProjectPath: d.ProjectPath,
				ProjectName: d.ProjectName, Branch: d.Branch, Commit: d.Commit, Path: d.Path, Kind: string(d.Kind), Ord: d.Ord, Text: d.Text}
			if e.dims > 0 && len(d.Vector) > 0 {
				out[i].Vectors = map[string][]float32{embedder: d.Vector}
			}
		}
		info, err := idx.AddDocumentsWithContext(ctx, out, &meilisearch.DocumentOptions{PrimaryKey: new("id")})
		if err := e.wait(ctx, "add documents", info, err); err != nil {
			return err
		}
	}
	return nil
}

func (e *EngineClient) Target() string { return strings.TrimRight(e.url, "/") + "/indexes/" + e.index }

func (e *EngineClient) Retain(ctx context.Context, projects []uuid.UUID) error {
	idx, err := e.prepare(ctx)
	if err != nil {
		return err
	}
	var info *meilisearch.TaskInfo
	if len(projects) == 0 {
		info, err = idx.DeleteAllDocumentsWithContext(ctx, nil)
	} else {
		keep := make([]string, len(projects))
		for i, p := range projects {
			keep[i] = quote(p.String())
		}
		info, err = idx.DeleteDocumentsByFilterWithContext(ctx, "NOT project_id IN ["+strings.Join(keep, ", ")+"]", nil)
	}
	return e.wait(ctx, "delete", info, err)
}

type hit struct {
	ProjectID   string `json:"project_id"`
	ProjectPath string `json:"project_path"`
	ProjectName string `json:"project_name"`
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Text        string `json:"text"`
	Formatted   struct {
		Text string `json:"text"`
	} `json:"_formatted"`
}

func (e *EngineClient) Search(ctx context.Context, q knowledgetypes.EngineQuery) ([]knowledgetypes.Hit, bool, error) {
	var pairs []string
	for _, p := range q.Projects {
		branch := q.Branches[p]
		if q.Branch != nil {
			branch = *q.Branch
		}
		if branch != "" {
			pairs = append(pairs, quote(p.String()+"|"+branch))
		}
	}
	if len(pairs) == 0 {
		return nil, false, nil
	}
	idx, err := e.prepare(ctx)
	if err != nil {
		return nil, false, err
	}
	want := q.Offset + q.Limit + 1
	fetch := want
	if q.Path != "" {
		fetch = min(want*5, 1000)
	}
	req := &meilisearch.SearchRequest{Filter: "pb IN [" + strings.Join(pairs, ", ") + "]", Limit: int64(fetch)}
	query := q.Q
	switch q.Mode {
	case knowledgetypes.ModeText:
		req.AttributesToCrop, req.CropLength = []string{"text"}, 40
		req.AttributesToHighlight, req.HighlightPreTag, req.HighlightPostTag = []string{"text"}, markStart, markStop
	case knowledgetypes.ModeSemantic:
		req.Vector, req.Hybrid, query = q.Vector, &meilisearch.SearchRequestHybrid{Embedder: embedder, SemanticRatio: 1}, ""
	default:
		req.Vector, req.Hybrid = q.Vector, &meilisearch.SearchRequestHybrid{Embedder: embedder, SemanticRatio: 0.5}
	}
	raw, err := idx.SearchRawWithContext(ctx, query, req)
	if err != nil {
		return nil, false, unavailable("search", err)
	}
	var res struct {
		Hits []hit `json:"hits"`
	}
	if err := json.Unmarshal(*raw, &res); err != nil {
		return nil, false, unavailable("search", err)
	}
	var hits []knowledgetypes.Hit
	for _, h := range res.Hits {
		if !strings.HasPrefix(h.Path, q.Path) {
			continue
		}
		id, err := uuid.Parse(h.ProjectID)
		if err != nil {
			continue
		}
		var snippet []knowledgetypes.Segment
		if q.Mode == knowledgetypes.ModeText {
			snippet = segments(h.Formatted.Text)
		} else {
			text := []rune(h.Text)
			snippet = []knowledgetypes.Segment{{Text: string(text[:min(len(text), 300)])}}
		}
		hits = append(hits, knowledgetypes.Hit{ProjectID: id, ProjectPath: h.ProjectPath, ProjectName: h.ProjectName, Branch: h.Branch,
			Commit: h.Commit, Path: h.Path, Kind: knowledgetypes.Kind(h.Kind), Snippet: snippet})
	}
	if q.Offset >= len(hits) {
		return nil, false, nil
	}
	hits = hits[q.Offset:]
	more := len(hits) > q.Limit
	if more {
		hits = hits[:q.Limit]
	}
	return hits, more, nil
}

func segments(formatted string) []knowledgetypes.Segment {
	var out []knowledgetypes.Segment
	for formatted != "" {
		before, rest, found := strings.Cut(formatted, markStart)
		if before != "" {
			out = append(out, knowledgetypes.Segment{Text: before})
		}
		if !found {
			break
		}
		match, after, _ := strings.Cut(rest, markStop)
		if match != "" {
			out = append(out, knowledgetypes.Segment{Text: match, Match: true})
		}
		formatted = after
	}
	return out
}
