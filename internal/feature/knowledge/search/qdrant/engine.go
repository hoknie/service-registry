package qdrant

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	qd "github.com/qdrant/go-client/qdrant"

	"svc-registry/internal/feature/knowledge"
)

var pointSpace = uuid.MustParse("6f1f6a52-0c55-4b1e-9a0f-3b0e7d4c2a11")

type Engine struct {
	addr       string
	apiKey     string
	collection string
	dims       int
	mu         sync.Mutex
	client     *qd.Client
	ready      bool
}

func New(addr, apiKey, collection string, dims int) *Engine {
	return &Engine{addr: addr, apiKey: apiKey, collection: collection, dims: dims}
}

func (e *Engine) Name() string { return "qdrant" }
func (e *Engine) Modes() []knowledge.Mode {
	return []knowledge.Mode{knowledge.ModeText, knowledge.ModeSemantic, knowledge.ModeHybrid}
}
func (e *Engine) DefaultMode() knowledge.Mode   { return knowledge.ModeHybrid }
func (e *Engine) Handles(m knowledge.Mode) bool { return m == knowledge.ModeSemantic }

func unavailable(op string, err error) error {
	return fmt.Errorf("%w: qdrant %s: %v", knowledge.ErrEngineUnavailable, op, err)
}

func (e *Engine) conn(ctx context.Context) (*qd.Client, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil {
		host, port, err := net.SplitHostPort(e.addr)
		if err != nil {
			return nil, unavailable("address", err)
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, unavailable("address", err)
		}
		c, err := qd.NewClient(&qd.Config{Host: host, Port: p, APIKey: e.apiKey, SkipCompatibilityCheck: true})
		if err != nil {
			return nil, unavailable("connect", err)
		}
		e.client = c
	}
	if !e.ready {
		exists, err := e.client.CollectionExists(ctx, e.collection)
		if err != nil {
			return nil, unavailable("collection", err)
		}
		if !exists {
			err := e.client.CreateCollection(ctx, &qd.CreateCollection{CollectionName: e.collection,
				VectorsConfig: qd.NewVectorsConfig(&qd.VectorParams{Size: uint64(e.dims), Distance: qd.Distance_Cosine})})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				return nil, unavailable("create collection", err)
			}
		}
		if err := e.checkDimensions(ctx); err != nil {
			return nil, err
		}
		for _, field := range []string{"project_id", "pb", "file"} {
			if _, err := e.client.CreateFieldIndex(ctx, &qd.CreateFieldIndexCollection{CollectionName: e.collection, Wait: qd.PtrOf(true),
				FieldName: field, FieldType: qd.FieldType_FieldTypeKeyword.Enum()}); err != nil {
				return nil, unavailable("index", err)
			}
		}
		e.ready = true
	}
	return e.client, nil
}

func pointID(d knowledge.IndexDoc) string {
	return uuid.NewSHA1(pointSpace, []byte(fmt.Sprintf("%s|%s|%s|%d", d.ProjectID, d.Branch, d.Path, d.Ord))).String()
}

func (e *Engine) checkDimensions(ctx context.Context) error {
	info, err := e.client.GetCollectionInfo(ctx, e.collection)
	if err != nil {
		return unavailable("collection info", err)
	}
	vectors := info.GetConfig().GetParams().GetVectorsConfig()
	params := vectors.GetParams()
	if params == nil {
		return fmt.Errorf("%w: qdrant collection %q has named vectors, not one vector of EMBEDDINGS_DIMENSIONS=%d; set another QDRANT_COLLECTION",
			knowledge.ErrEngineDimensions, e.collection, e.dims)
	}
	if size := params.GetSize(); size != uint64(e.dims) {
		return fmt.Errorf("%w: qdrant collection %q stores %d-dimensional vectors, EMBEDDINGS_DIMENSIONS=%d; set another QDRANT_COLLECTION or recreate the collection",
			knowledge.ErrEngineDimensions, e.collection, size, e.dims)
	}
	return nil
}

func (e *Engine) Check(ctx context.Context) error {
	_, err := e.conn(ctx)
	return err
}

func (e *Engine) Sync(ctx context.Context, project uuid.UUID, docs []knowledge.IndexDoc) error {
	c, err := e.conn(ctx)
	if err != nil {
		return err
	}
	ids := make([]*qd.PointId, 0, len(docs))
	const batch = 256
	for start := 0; start < len(docs); start += batch {
		var points []*qd.PointStruct
		for _, d := range docs[start:min(len(docs), start+batch)] {
			id := qd.NewID(pointID(d))
			ids = append(ids, id)
			points = append(points, &qd.PointStruct{Id: id, Vectors: qd.NewVectors(d.Vector...), Payload: qd.NewValueMap(map[string]any{
				"project_id": d.ProjectID.String(), "pb": d.ProjectID.String() + "|" + d.Branch,
				"file": d.ProjectID.String() + "|" + d.Branch + "|" + d.Path, "branch": d.Branch, "commit": d.Commit,
				"path": d.Path, "kind": string(d.Kind), "project_path": d.ProjectPath, "project_name": d.ProjectName, "text": d.Text,
			})})
		}
		if _, err := c.Upsert(ctx, &qd.UpsertPoints{CollectionName: e.collection, Wait: qd.PtrOf(true), Points: points}); err != nil {
			return unavailable("upsert", err)
		}
	}
	stale := &qd.Filter{Must: []*qd.Condition{qd.NewMatchKeyword("project_id", project.String())}}
	if len(ids) > 0 {
		stale.MustNot = []*qd.Condition{qd.NewHasID(ids...)}
	}
	if _, err := c.Delete(ctx, &qd.DeletePoints{CollectionName: e.collection, Wait: qd.PtrOf(true), Points: qd.NewPointsSelectorFilter(stale)}); err != nil {
		return unavailable("delete", err)
	}
	return nil
}

func (e *Engine) Target() string { return "qdrant://" + e.addr + "/" + e.collection }

func (e *Engine) Retain(ctx context.Context, projects []uuid.UUID) error {
	c, err := e.conn(ctx)
	if err != nil {
		return err
	}
	keep := make([]string, len(projects))
	for i, p := range projects {
		keep[i] = p.String()
	}
	filter := &qd.Filter{}
	if len(keep) > 0 {
		filter.MustNot = []*qd.Condition{qd.NewMatchKeywords("project_id", keep...)}
	}
	if _, err := c.Delete(ctx, &qd.DeletePoints{CollectionName: e.collection, Wait: qd.PtrOf(true), Points: qd.NewPointsSelectorFilter(filter)}); err != nil {
		return unavailable("delete", err)
	}
	return nil
}

func (e *Engine) Search(ctx context.Context, q knowledge.EngineQuery) ([]knowledge.Hit, bool, error) {
	var pairs []string
	for _, p := range q.Projects {
		branch := q.Branches[p]
		if q.Branch != nil {
			branch = *q.Branch
		}
		if branch != "" {
			pairs = append(pairs, p.String()+"|"+branch)
		}
	}
	if len(pairs) == 0 {
		return nil, false, nil
	}
	c, err := e.conn(ctx)
	if err != nil {
		return nil, false, err
	}
	want := q.Offset + q.Limit + 1
	fetch := want
	if q.Path != "" {
		fetch = want * 5
	}
	groups, err := c.QueryGroups(ctx, &qd.QueryPointGroups{CollectionName: e.collection, Query: qd.NewQueryDense(q.Vector),
		Filter: &qd.Filter{Must: []*qd.Condition{qd.NewMatchKeywords("pb", pairs...)}}, GroupBy: "file", GroupSize: qd.PtrOf(uint64(1)),
		Limit: qd.PtrOf(uint64(fetch)), WithPayload: qd.NewWithPayload(true)})
	if err != nil {
		return nil, false, unavailable("query", err)
	}
	var hits []knowledge.Hit
	for _, g := range groups {
		if len(g.GetHits()) == 0 {
			continue
		}
		pl := g.GetHits()[0].GetPayload()
		get := func(k string) string { return pl[k].GetStringValue() }
		if !strings.HasPrefix(get("path"), q.Path) {
			continue
		}
		id, err := uuid.Parse(get("project_id"))
		if err != nil {
			continue
		}
		text := []rune(get("text"))
		hits = append(hits, knowledge.Hit{ProjectID: id, ProjectPath: get("project_path"), ProjectName: get("project_name"),
			Branch: get("branch"), Commit: get("commit"), Path: get("path"), Kind: knowledge.Kind(get("kind")),
			Snippet: []knowledge.Segment{{Text: string(text[:min(len(text), 300)])}}})
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
