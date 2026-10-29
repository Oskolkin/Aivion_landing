package migrationpreflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Report is a read-only preflight summary for migration blockers.
type Report struct {
	Status                  string
	ActiveSyncDuplicates    int64
	ImportJobDuplicates     int64
	ActionItemsWithoutRunID int64
	AmbiguousActionItems    int64
	DuplicateActionKeys     int64
	OrphanDashboardLinks    int64
	OrphanActionsLinks      int64
	PublishedOrphanLinks    int64
}

func (r Report) Blocked() bool {
	return r.Status == "BLOCKED"
}

// Run executes read-only checks against the current database schema/data.
func Run(ctx context.Context, pool *pgxpool.Pool) (Report, error) {
	var r Report
	if pool == nil {
		return r, fmt.Errorf("pool is nil")
	}

	queries := []struct {
		dest *int64
		sql  string
	}{
		{&r.ActiveSyncDuplicates, `
SELECT COUNT(*) FROM (
  SELECT seller_account_id
  FROM sync_jobs
  WHERE status IN ('pending', 'running')
  GROUP BY seller_account_id
  HAVING COUNT(*) > 1
) t`},
		{&r.ImportJobDuplicates, `
SELECT COUNT(*) FROM (
  SELECT sync_job_id, domain
  FROM import_jobs
  WHERE COALESCE(superseded_by_import_job_id, 0) = 0
    AND status <> 'superseded'
    AND domain !~ '__dedup_[0-9]+$'
  GROUP BY sync_job_id, domain
  HAVING COUNT(*) > 1
) t`},
		{&r.ActionItemsWithoutRunID, `
SELECT COUNT(*) FROM action_items WHERE run_id IS NULL`},
		{&r.AmbiguousActionItems, `
SELECT COUNT(*) FROM action_items ai
WHERE ai.run_id IS NULL
  AND (
    SELECT COUNT(DISTINCT agr.id)
    FROM action_generation_runs agr
    WHERE agr.seller_account_id = ai.seller_account_id
      AND agr.metric_date = ai.metric_date
  ) > 1`},
		{&r.DuplicateActionKeys, `
SELECT COUNT(*) FROM (
  SELECT run_id, action_key
  FROM action_items
  WHERE run_id IS NOT NULL
  GROUP BY run_id, action_key
  HAVING COUNT(*) > 1
) t`},
		{&r.OrphanDashboardLinks, `
SELECT COUNT(*) FROM ai_pipeline_runs p
WHERE p.dashboard_run_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM ai_dashboard_narratives d WHERE d.id = p.dashboard_run_id)`},
		{&r.OrphanActionsLinks, `
SELECT COUNT(*) FROM ai_pipeline_runs p
WHERE p.actions_run_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM action_generation_runs a WHERE a.id = p.actions_run_id)`},
		{&r.PublishedOrphanLinks, `
SELECT COUNT(*) FROM ai_pipeline_runs p
WHERE p.published_at IS NOT NULL
  AND (
    (p.dashboard_run_id IS NOT NULL AND NOT EXISTS (
      SELECT 1 FROM ai_dashboard_narratives d WHERE d.id = p.dashboard_run_id))
    OR
    (p.actions_run_id IS NOT NULL AND NOT EXISTS (
      SELECT 1 FROM action_generation_runs a WHERE a.id = p.actions_run_id))
  )`},
	}

	for _, q := range queries {
		if err := pool.QueryRow(ctx, q.sql).Scan(q.dest); err != nil {
			// Backward compatibility before 000051 columns exist.
			if strings.Contains(err.Error(), "superseded_by_import_job_id") {
				*q.dest = 0
				continue
			}
			return r, err
		}
	}

	if r.ActiveSyncDuplicates > 0 ||
		r.ImportJobDuplicates > 0 ||
		r.ActionItemsWithoutRunID > 0 ||
		r.AmbiguousActionItems > 0 ||
		r.DuplicateActionKeys > 0 ||
		r.OrphanDashboardLinks > 0 ||
		r.OrphanActionsLinks > 0 ||
		r.PublishedOrphanLinks > 0 {
		r.Status = "BLOCKED"
	} else {
		r.Status = "PASS"
	}
	return r, nil
}

func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "status=%s\n", r.Status)
	fmt.Fprintf(&b, "active_sync_duplicates: %d\n", r.ActiveSyncDuplicates)
	fmt.Fprintf(&b, "import_job_duplicates: %d\n", r.ImportJobDuplicates)
	fmt.Fprintf(&b, "action_items_without_run_id: %d\n", r.ActionItemsWithoutRunID)
	fmt.Fprintf(&b, "ambiguous_action_items: %d\n", r.AmbiguousActionItems)
	fmt.Fprintf(&b, "duplicate_action_keys: %d\n", r.DuplicateActionKeys)
	fmt.Fprintf(&b, "orphan_dashboard_links: %d\n", r.OrphanDashboardLinks)
	fmt.Fprintf(&b, "orphan_actions_links: %d\n", r.OrphanActionsLinks)
	fmt.Fprintf(&b, "published_orphan_links: %d\n", r.PublishedOrphanLinks)
	return b.String()
}
