package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SearchRepository struct{ db *pgtx.DB }

func NewSearchRepository(db *pgtx.DB) *SearchRepository { return &SearchRepository{db: db} }

func like(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *SearchRepository) Search(ctx context.Context, v knowledgetypes.Viewer, q knowledgetypes.Search, t knowledgetypes.TextQuery) ([]knowledgetypes.Hit, error) {
	words := make([]string, 0, len(t.PathWords))
	for _, w := range t.PathWords {
		words = append(words, "%"+like(w)+"%")
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(hitRow.hit), `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $13::uuid[], $14::boolean)
		),
		readable AS (
			SELECT n.id
			FROM nodes n
			WHERE $2 OR n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN readable r ON c.parent_id = r.id
		),
		latest AS (
			SELECT DISTINCT ON (s.project_id, s.branch) s.id, s.project_id, s.branch, s.commit_sha
			FROM knowledge_snapshots s
			JOIN nodes n ON n.id = s.project_id
			LEFT JOIN knowledge_sources src ON src.project_id = s.project_id
			LEFT JOIN forge_repositories r ON r.project_id = s.project_id AND r.orphaned_at IS NULL
			WHERE s.status <> 'failed'
				AND s.project_id IN (SELECT id FROM readable)
				AND ($3::uuid IS NULL OR s.project_id = $3)
				AND (($4::text IS NULL AND s.branch = CASE
						WHEN src.kind = 'local_dir' THEN 'local'
						ELSE COALESCE(n.default_branch, r.default_branch, NULLIF(src.default_branch, '')) END) OR s.branch = $4)
			ORDER BY s.project_id, s.branch, s.collected_at DESC, s.id DESC
		),
		stems AS (
			SELECT websearch_to_tsquery('russian', $5) AS q
		),
		q AS (
			SELECT CASE WHEN numnode(s.q) = 0 THEN websearch_to_tsquery('simple', $5) ELSE s.q END AS q,
				(CASE WHEN numnode(s.q) = 0 THEN 'simple' ELSE 'russian' END)::regconfig AS cfg
			FROM stems s
		),
		pref AS (
			SELECT CASE WHEN $10::text = '' THEN NULL ELSE to_tsquery('simple', $10) END AS pos,
				CASE WHEN $11::text = '' THEN NULL ELSE to_tsquery('simple', $11) END AS neg,
				CASE WHEN $12::text = '' THEN NULL ELSE websearch_to_tsquery('russian', $12) END AS stem_neg
		),
		hits AS (
			SELECT l.project_id, l.branch, l.commit_sha, f.path, f.kind, f.sha256,
				(cardinality($7::text[]) > 0 AND (SELECT bool_and(lower(f.path) LIKE w ESCAPE '\') FROM unnest($7::text[]) w)) AS in_path,
				COALESCE(b.tsv @@ q.q, false) AS in_stems,
				COALESCE(b.tsv @@ q.q, false) OR COALESCE(b.tsv @@ pref.pos, false) AS in_text,
				COALESCE(b.tsv @@ pref.neg, false) OR COALESCE(b.tsv @@ pref.stem_neg, false) AS excluded,
				GREATEST(COALESCE(ts_rank_cd(b.tsv, q.q), 0), COALESCE(ts_rank_cd(b.tsv, pref.pos), 0)) AS rank
			FROM latest l
			JOIN knowledge_files f ON f.snapshot_id = l.id
			LEFT JOIN knowledge_blobs b ON b.sha256 = f.sha256, q, pref
			WHERE f.path LIKE $6 ESCAPE '\'
		),
		page AS (
			SELECT *
			FROM hits
			WHERE (in_path OR in_text)
				AND NOT excluded
			ORDER BY in_path DESC, rank DESC, project_id, branch, path
			LIMIT $8
			OFFSET $9
		)
		SELECT p.project_id, n.name, (
			WITH RECURSIVE up AS (
				SELECT id, parent_id, slug, 0 AS lvl
				FROM nodes
				WHERE id = p.project_id
				UNION ALL
				SELECT x.id, x.parent_id, x.slug, up.lvl + 1
				FROM nodes x
				JOIN up ON x.id = up.parent_id
			)
			SELECT string_agg(slug, '/' ORDER BY lvl DESC)
			FROM up
		) AS project_path, p.branch, p.commit_sha, p.path, p.kind,
			CASE
				WHEN p.in_stems THEN ts_headline(q.cfg, b.content, q.q,
					'StartSel=' || chr(57344) || ', StopSel=' || chr(57345) || ', MaxWords=40, MinWords=15, MaxFragments=1')
				WHEN p.in_text THEN ts_headline('simple', b.content, pref.pos,
					'StartSel=' || chr(57344) || ', StopSel=' || chr(57345) || ', MaxWords=40, MinWords=15, MaxFragments=1')
				ELSE left(COALESCE(b.content, ''), 300) END AS fragment
		FROM page p
		JOIN nodes n ON n.id = p.project_id
		LEFT JOIN knowledge_blobs b ON b.sha256 = p.sha256, q, pref
		ORDER BY p.in_path DESC, p.rank DESC, p.project_id, p.branch, p.path`,
		v.UserID, v.Superadmin, q.Project, q.Branch, q.Q, like(q.Path)+"%", words,
		q.Limit, q.Offset, t.Prefix, t.Negative, t.Excluded, v.Nodes, v.Admin)
}

func (s *SearchRepository) Readable(ctx context.Context, v knowledgetypes.Viewer, project *uuid.UUID) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $4::uuid[], $5::boolean)
		),
		readable AS (
			SELECT n.id
			FROM nodes n
			WHERE $2 OR n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN readable r ON c.parent_id = r.id
		)
		SELECT n.id
		FROM nodes n
		WHERE n.kind = 'project'
			AND n.id IN (SELECT id FROM readable)
			AND ($3::uuid IS NULL OR n.id = $3)`, v.UserID, v.Superadmin, project, v.Nodes, v.Admin)
}

func (s *SearchRepository) DefaultBranches(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID]string, error) {
	return postgres.Pairs[uuid.UUID, string](ctx, s.db.From(ctx), constraints, `
		SELECT n.id,
			CASE
				WHEN src.kind = 'local_dir' THEN 'local'
				ELSE COALESCE(n.default_branch, r.default_branch, NULLIF(src.default_branch, ''), '') END
		FROM nodes n
		LEFT JOIN knowledge_sources src ON src.project_id = n.id
		LEFT JOIN forge_repositories r ON r.project_id = n.id AND r.orphaned_at IS NULL
		WHERE n.id = ANY($1)`, projects)
}

func (s *SearchRepository) Projects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]knowledgetypes.ProjectName, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[projectNameRow], `
		SELECT p.id, p.name, (
			WITH RECURSIVE up AS (
				SELECT id, parent_id, slug, 0 AS lvl
				FROM nodes
				WHERE id = p.id
				UNION ALL
				SELECT x.id, x.parent_id, x.slug, up.lvl + 1
				FROM nodes x
				JOIN up ON x.id = up.parent_id
			)
			SELECT string_agg(slug, '/' ORDER BY lvl DESC)
			FROM up
		)
		FROM nodes p
		WHERE p.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]knowledgetypes.ProjectName{}
	for _, r := range rows {
		out[r.ID] = knowledgetypes.ProjectName{Name: r.Name, Path: r.Path}
	}
	return out, nil
}
