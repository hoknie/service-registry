package knowledge

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/knowledge"
)

type SearchStore struct{ pool *pgxpool.Pool }

func NewSearchStore(pool *pgxpool.Pool) *SearchStore { return &SearchStore{pool: pool} }

const headlineOptions = "'StartSel=' || chr(57344) || ', StopSel=' || chr(57345) || ', MaxWords=40, MinWords=15, MaxFragments=1'"

const searchQuery = "WITH RECURSIVE bound AS (SELECT node_id FROM role_bindings WHERE user_id = $1 " +
	"UNION ALL SELECT b.node_id FROM role_bindings b JOIN group_members gm ON gm.group_id = b.group_id WHERE gm.user_id = $1), " +
	"readable AS (SELECT n.id FROM nodes n WHERE $2 OR n.id IN (SELECT node_id FROM bound) " +
	"UNION SELECT c.id FROM nodes c JOIN readable r ON c.parent_id = r.id), " +
	"latest AS (SELECT DISTINCT ON (s.project_id, s.branch) s.id, s.project_id, s.branch, s.commit_sha FROM knowledge_snapshots s " +
	"JOIN nodes n ON n.id = s.project_id LEFT JOIN knowledge_sources src ON src.project_id = s.project_id " +
	"LEFT JOIN forge_repositories r ON r.project_id = s.project_id AND r.orphaned_at IS NULL " +
	"WHERE s.status <> 'failed' AND s.project_id IN (SELECT id FROM readable) " +
	"AND ($3::uuid IS NULL OR s.project_id = $3) " +
	"AND (($4::text IS NULL AND s.branch = CASE WHEN src.kind = 'local_dir' THEN 'local' " +
	"ELSE COALESCE(n.default_branch, r.default_branch, NULLIF(src.default_branch, '')) END) OR s.branch = $4) " +
	"ORDER BY s.project_id, s.branch, s.collected_at DESC, s.id DESC), " +
	"stems AS (SELECT websearch_to_tsquery('russian', $5) AS q), " +
	"q AS (SELECT CASE WHEN numnode(s.q) = 0 THEN websearch_to_tsquery('simple', $5) ELSE s.q END AS q, " +
	"(CASE WHEN numnode(s.q) = 0 THEN 'simple' ELSE 'russian' END)::regconfig AS cfg FROM stems s), " +
	"pref AS (SELECT CASE WHEN $10::text = '' THEN NULL ELSE to_tsquery('simple', $10) END AS pos, " +
	"CASE WHEN $11::text = '' THEN NULL ELSE to_tsquery('simple', $11) END AS neg, " +
	"CASE WHEN $12::text = '' THEN NULL ELSE websearch_to_tsquery('russian', $12) END AS stem_neg), " +
	"hits AS (SELECT l.project_id, l.branch, l.commit_sha, f.path, f.kind, f.sha256, " +
	"(cardinality($7::text[]) > 0 AND (SELECT bool_and(lower(f.path) LIKE w ESCAPE '\\') FROM unnest($7::text[]) w)) AS in_path, " +
	"COALESCE(b.tsv @@ q.q, false) AS in_stems, " +
	"COALESCE(b.tsv @@ q.q, false) OR COALESCE(b.tsv @@ pref.pos, false) AS in_text, " +
	"COALESCE(b.tsv @@ pref.neg, false) OR COALESCE(b.tsv @@ pref.stem_neg, false) AS excluded, " +
	"GREATEST(COALESCE(ts_rank_cd(b.tsv, q.q), 0), COALESCE(ts_rank_cd(b.tsv, pref.pos), 0)) AS rank " +
	"FROM latest l JOIN knowledge_files f ON f.snapshot_id = l.id LEFT JOIN knowledge_blobs b ON b.sha256 = f.sha256, q, pref " +
	"WHERE f.path LIKE $6 ESCAPE '\\'), " +
	"page AS (SELECT * FROM hits WHERE (in_path OR in_text) AND NOT excluded " +
	"ORDER BY in_path DESC, rank DESC, project_id, branch, path LIMIT $8 OFFSET $9) " +
	"SELECT p.project_id, n.name, (WITH RECURSIVE up AS (SELECT id, parent_id, slug, 0 AS lvl FROM nodes WHERE id = p.project_id " +
	"UNION ALL SELECT x.id, x.parent_id, x.slug, up.lvl + 1 FROM nodes x JOIN up ON x.id = up.parent_id) " +
	"SELECT string_agg(slug, '/' ORDER BY lvl DESC) FROM up) AS project_path, " +
	"p.branch, p.commit_sha, p.path, p.kind, " +
	"CASE WHEN p.in_stems THEN ts_headline(q.cfg, b.content, q.q, " + headlineOptions + ") " +
	"WHEN p.in_text THEN ts_headline('simple', b.content, pref.pos, " + headlineOptions + ") " +
	"ELSE left(COALESCE(b.content, ''), 300) END AS fragment " +
	"FROM page p JOIN nodes n ON n.id = p.project_id LEFT JOIN knowledge_blobs b ON b.sha256 = p.sha256, q, pref " +
	"ORDER BY p.in_path DESC, p.rank DESC, p.project_id, p.branch, p.path"

func like(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *SearchStore) Search(ctx context.Context, v domain.Viewer, q domain.Search) ([]domain.Hit, bool, error) {
	words := []string{}
	for _, w := range domain.PathWords(q.Q) {
		words = append(words, "%"+like(w)+"%")
	}
	pos, neg, _ := domain.PrefixQuery(q.Q)
	rows, err := s.pool.Query(ctx, searchQuery, v.UserID, v.Superadmin, q.Project, q.Branch, q.Q, like(q.Path)+"%", words,
		q.Limit+1, q.Offset, pos, neg, domain.ExcludedWords(q.Q))
	if err != nil {
		return nil, false, dbErr(err)
	}
	hits, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Hit, error) {
		var h domain.Hit
		var kind, fragment string
		var id uuid.UUID
		err := r.Scan(&id, &h.ProjectName, &h.ProjectPath, &h.Branch, &h.Commit, &h.Path, &kind, &fragment)
		h.ProjectID, h.Kind = id, domain.Kind(kind)
		h.Snippet = trim(domain.Segments(fragment), 300)
		return h, err
	})
	if err != nil {
		return nil, false, dbErr(err)
	}
	more := len(hits) > q.Limit
	if more {
		hits = hits[:q.Limit]
	}
	return hits, more, nil
}

func trim(segments []domain.Segment, n int) []domain.Segment {
	out := []domain.Segment{}
	for _, s := range segments {
		if n <= 0 {
			break
		}
		if c := utf8.RuneCountInString(s.Text); c > n {
			s.Text = string([]rune(s.Text)[:n])
		}
		n -= utf8.RuneCountInString(s.Text)
		out = append(out, s)
	}
	return out
}

const readableProjects = "WITH RECURSIVE bound AS (SELECT node_id FROM role_bindings WHERE user_id = $1 " +
	"UNION ALL SELECT b.node_id FROM role_bindings b JOIN group_members gm ON gm.group_id = b.group_id WHERE gm.user_id = $1), " +
	"readable AS (SELECT n.id FROM nodes n WHERE $2 OR n.id IN (SELECT node_id FROM bound) " +
	"UNION SELECT c.id FROM nodes c JOIN readable r ON c.parent_id = r.id) " +
	"SELECT n.id FROM nodes n WHERE n.kind = 'project' AND n.id IN (SELECT id FROM readable) AND ($3::uuid IS NULL OR n.id = $3)"

const defaultBranches = "SELECT n.id, CASE WHEN src.kind = 'local_dir' THEN 'local' " +
	"ELSE COALESCE(n.default_branch, r.default_branch, NULLIF(src.default_branch, ''), '') END " +
	"FROM nodes n LEFT JOIN knowledge_sources src ON src.project_id = n.id " +
	"LEFT JOIN forge_repositories r ON r.project_id = n.id AND r.orphaned_at IS NULL WHERE n.id = ANY($1)"

const projectNames = "SELECT p.id, p.name, (WITH RECURSIVE up AS (SELECT id, parent_id, slug, 0 AS lvl FROM nodes WHERE id = p.id " +
	"UNION ALL SELECT x.id, x.parent_id, x.slug, up.lvl + 1 FROM nodes x JOIN up ON x.id = up.parent_id) " +
	"SELECT string_agg(slug, '/' ORDER BY lvl DESC) FROM up) FROM nodes p WHERE p.id = ANY($1)"

func (s *SearchStore) Readable(ctx context.Context, v domain.Viewer, project *uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, readableProjects, v.UserID, v.Superadmin, project)
	if err != nil {
		return nil, dbErr(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, dbErr(err)
}

func (s *SearchStore) DefaultBranches(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := s.pool.Query(ctx, defaultBranches, projects)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var branch string
		if err := rows.Scan(&id, &branch); err != nil {
			return nil, dbErr(err)
		}
		out[id] = branch
	}
	return out, dbErr(rows.Err())
}

func (s *SearchStore) Projects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ProjectName, error) {
	rows, err := s.pool.Query(ctx, projectNames, ids)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]domain.ProjectName{}
	for rows.Next() {
		var id uuid.UUID
		var p domain.ProjectName
		if err := rows.Scan(&id, &p.Name, &p.Path); err != nil {
			return nil, dbErr(err)
		}
		out[id] = p
	}
	return out, dbErr(rows.Err())
}
