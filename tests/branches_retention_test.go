package tests

import (
	"context"
	"testing"

	"svc-registry/internal/service"
)

func TestRetentionDeletesExpiredBranchesOnly(t *testing.T) {
	t.Parallel()
	app := startApp(t, "BRANCH_RETENTION_DAYS", "30", "BRANCH_STALE_DAYS", "90")
	root := app.admin()
	project := seedBranches(t, app, root)
	execSQL(t, app.db, "UPDATE branches SET sources = '{forge,ingest}', gone_at = now() - interval '31 days' WHERE name IN ('feature/a', 'feature/b', 'main')")
	execSQL(t, app.db, "UPDATE branches SET pinned = true WHERE name = 'feature/b'")
	execSQL(t, app.db, "UPDATE branches SET last_activity_at = now() - interval '121 days' WHERE name = 'release/1'")
	n, err := service.PruneBranches(context.Background(), app.state)
	must(t, err)
	eq(t, n, int64(2))
	eq(t, app.branchNames(root, project, "state=all"), "main,feature/b")
	execSQL(t, app.db, "UPDATE branches SET sources = '{ingest}', gone_at = NULL, pinned = false, last_activity_at = now() - interval '100 days' WHERE name = 'feature/b'")
	n, err = service.PruneBranches(context.Background(), app.state)
	must(t, err)
	eq(t, n, int64(0))
	eq(t, len(list(t, app.get(nodePath(project)+"/deployments?branch=release%2F1", root).json(t), "items")), 1)
}
