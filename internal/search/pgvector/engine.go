package pgvector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/knowledge"
)

type Engine struct {
	pool   *pgxpool.Pool
	model  string
	schema string
}

func New(pool *pgxpool.Pool, model string) *Engine { return &Engine{pool: pool, model: model} }

func (e *Engine) Name() string { return "pgvector" }
func (e *Engine) Modes() []knowledge.Mode {
	return []knowledge.Mode{knowledge.ModeText, knowledge.ModeSemantic, knowledge.ModeHybrid}
}
func (e *Engine) DefaultMode() knowledge.Mode   { return knowledge.ModeHybrid }
func (e *Engine) Handles(m knowledge.Mode) bool { return m == knowledge.ModeSemantic }

var ErrNoExtension = errors.New("KNOWLEDGE_SEARCH_ENGINE=pgvector needs the PostgreSQL extension \"vector\" (pgvector); install it and run db:migrate")

func (e *Engine) Check(ctx context.Context) error {
	var schema string
	err := e.pool.QueryRow(ctx, "SELECT n.nspname FROM pg_extension x JOIN pg_namespace n ON n.oid = x.extnamespace WHERE x.extname = 'vector'").Scan(&schema)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoExtension
	}
	if err != nil {
		return fmt.Errorf("%w: %v", knowledge.ErrSearchUnavailable, err)
	}
	e.schema = schema
	return nil
}

func quote(id string) string { return `"` + strings.ReplaceAll(id, `"`, `""`) + `"` }

func (e *Engine) query() string {
	s := quote(e.schema)
	return "WITH want AS (SELECT * FROM unnest($1::uuid[], $2::text[]) AS w(project_id, branch)), " +
		"latest AS (SELECT DISTINCT ON (s.project_id, s.branch) s.id, s.project_id, s.branch, s.commit_sha FROM knowledge_snapshots s " +
		"JOIN want w ON w.project_id = s.project_id AND w.branch = s.branch WHERE s.status <> 'failed' " +
		"ORDER BY s.project_id, s.branch, s.collected_at DESC, s.id DESC), " +
		"scored AS (SELECT DISTINCT ON (l.project_id, l.branch, f.path) l.project_id, l.branch, l.commit_sha, f.path, f.kind, f.sha256, " +
		"e.chunk_start, e.chunk_end, (e.vector::" + s + ".vector OPERATOR(" + s + ".<=>) $3::real[]::" + s + ".vector) AS d " +
		"FROM latest l JOIN knowledge_files f ON f.snapshot_id = l.id " +
		"JOIN knowledge_embeddings e ON e.sha256 = f.sha256 AND e.model = $4 WHERE f.path LIKE $5 ESCAPE '\\' " +
		"ORDER BY l.project_id, l.branch, f.path, d) " +
		"SELECT sc.project_id, n.name, (WITH RECURSIVE up AS (SELECT id, parent_id, slug, 0 AS lvl FROM nodes WHERE id = sc.project_id " +
		"UNION ALL SELECT x.id, x.parent_id, x.slug, up.lvl + 1 FROM nodes x JOIN up ON x.id = up.parent_id) " +
		"SELECT string_agg(slug, '/' ORDER BY lvl DESC) FROM up), sc.branch, sc.commit_sha, sc.path, sc.kind, " +
		"left(substring(b.content FROM sc.chunk_start + 1 FOR sc.chunk_end - sc.chunk_start), 300) " +
		"FROM scored sc JOIN nodes n ON n.id = sc.project_id JOIN knowledge_blobs b ON b.sha256 = sc.sha256 " +
		"ORDER BY sc.d, sc.project_id, sc.branch, sc.path LIMIT $6 OFFSET $7"
}

func (e *Engine) Search(ctx context.Context, q knowledge.EngineQuery) ([]knowledge.Hit, bool, error) {
	if e.schema == "" {
		if err := e.Check(ctx); err != nil {
			return nil, false, err
		}
	}
	projects, branches := Targets(q)
	if len(projects) == 0 {
		return nil, false, nil
	}
	rows, err := e.pool.Query(ctx, e.query(), projects, branches, q.Vector, e.model, Like(q.Path)+"%", q.Limit+1, q.Offset)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", knowledge.ErrSearchUnavailable, err)
	}
	hits, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (knowledge.Hit, error) {
		var h knowledge.Hit
		var kind, text string
		err := r.Scan(&h.ProjectID, &h.ProjectName, &h.ProjectPath, &h.Branch, &h.Commit, &h.Path, &kind, &text)
		h.Kind = knowledge.Kind(kind)
		h.Snippet = []knowledge.Segment{{Text: text}}
		return h, err
	})
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", knowledge.ErrSearchUnavailable, err)
	}
	more := len(hits) > q.Limit
	if more {
		hits = hits[:q.Limit]
	}
	return hits, more, nil
}

func Targets(q knowledge.EngineQuery) ([]uuid.UUID, []string) {
	var projects []uuid.UUID
	var branches []string
	for _, p := range q.Projects {
		branch := ""
		if q.Branch != nil {
			branch = *q.Branch
		} else {
			branch = q.Branches[p]
		}
		if branch == "" {
			continue
		}
		projects = append(projects, p)
		branches = append(branches, branch)
	}
	return projects, branches
}

func Like(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
