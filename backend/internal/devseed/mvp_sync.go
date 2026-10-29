package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// MVPSyncImportDomains matches worker coordinator + sales ledger imports for admin UI.
var MVPSyncImportDomains = []string{"products", "orders", "sales", "stocks", "ads"}

// SeedMVPSyncArtifacts creates completed initial + incremental sync jobs, a historical failed sync, import_jobs, and cursors.
func SeedMVPSyncArtifacts(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	anchorUTC time.Time,
	commerce *CommerceSeedStats,
	ads *AdsSeedStats,
) error {
	if commerce == nil {
		return fmt.Errorf("commerce stats required")
	}
	if ads == nil {
		return fmt.Errorf("ads stats required")
	}

	adsRecv := int32(ads.Campaigns + ads.MetricRows + ads.SKULinks)
	domainStats := map[string]struct{ recv, imp int32 }{
		"products": {int32(commerce.Products), int32(commerce.Products)},
		"orders":   {int32(commerce.Orders), int32(commerce.Orders)},
		"sales":    {int32(commerce.Sales), int32(commerce.Sales)},
		"stocks":   {int32(commerce.Stocks), int32(commerce.Stocks)},
		"ads":      {adsRecv, adsRecv},
	}

	// Historical failed sync (diagnostics): most domains succeed, advertising fails.
	failedDay := anchorUTC.AddDate(0, 0, -5).UTC()
	failedStart := time.Date(failedDay.Year(), failedDay.Month(), failedDay.Day(), 11, 5, 0, 0, time.UTC)
	failedEnd := failedStart.Add(13 * time.Minute)
	failedJob, err := q.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sellerAccountID,
		Type:            "initial_sync",
		Status:          "failed",
		StartedAt:       pgtype.Timestamptz{Time: failedStart, Valid: true},
		FinishedAt:      pgtype.Timestamptz{Time: failedEnd, Valid: true},
		ErrorMessage:    pgtype.Text{String: "Partial failure: advertising import did not complete", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create failed sync job: %w", err)
	}

	for _, domain := range MVPSyncImportDomains {
		ds := domainStats[domain]
		recv := ds.recv
		imp := ds.imp
		curPayload := map[string]any{
			"mvp_seed":    true,
			"domain":      domain,
			"anchor_date": anchorUTC.Format("2006-01-02"),
			"sync_day":    failedDay.Format("2006-01-02"),
		}
		b, _ := json.Marshal(curPayload)
		st := "completed"
		var recF int32
		var errMsg pgtype.Text
		if domain == "ads" {
			st = "failed"
			recv = 10
			imp = 7
			recF = 3
			errMsg = pgtype.Text{String: "Performance API temporary timeout during demo import", Valid: true}
		}
		_, err := q.CreateImportJob(ctx, dbgen.CreateImportJobParams{
			SellerAccountID: sellerAccountID,
			SyncJobID:       failedJob.ID,
			Domain:          domain,
			Status:          st,
			SourceCursor:    pgtype.Text{String: string(b), Valid: true},
			RecordsReceived: recv,
			RecordsImported: imp,
			RecordsFailed:   recF,
			StartedAt:       pgtype.Timestamptz{Time: failedStart, Valid: true},
			FinishedAt:      pgtype.Timestamptz{Time: failedEnd, Valid: true},
			ErrorMessage:    errMsg,
		})
		if err != nil {
			return fmt.Errorf("create failed import job %s: %w", domain, err)
		}
	}

	// Completed initial sync (day before anchor).
	initDay := anchorUTC.AddDate(0, 0, -1).UTC()
	initStart := time.Date(initDay.Year(), initDay.Month(), initDay.Day(), 8, 10, 0, 0, time.UTC)
	initEnd := initStart.Add(22 * time.Minute)
	initJob, err := q.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sellerAccountID,
		Type:            "initial_sync",
		Status:          "completed",
		StartedAt:       pgtype.Timestamptz{Time: initStart, Valid: true},
		FinishedAt:      pgtype.Timestamptz{Time: initEnd, Valid: true},
		ErrorMessage:    pgtype.Text{},
	})
	if err != nil {
		return fmt.Errorf("create initial sync job: %w", err)
	}

	if err := seedImportJobsForCompletedSync(ctx, q, sellerAccountID, initJob.ID, anchorUTC, initDay, domainStats, initStart, initEnd); err != nil {
		return err
	}

	// Completed incremental sync on anchor date.
	incStart := time.Date(anchorUTC.Year(), anchorUTC.Month(), anchorUTC.Day(), 10, 12, 0, 0, time.UTC)
	incEnd := incStart.Add(26 * time.Minute)
	incJob, err := q.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sellerAccountID,
		Type:            "incremental_sync",
		Status:          "completed",
		StartedAt:       pgtype.Timestamptz{Time: incStart, Valid: true},
		FinishedAt:      pgtype.Timestamptz{Time: incEnd, Valid: true},
		ErrorMessage:    pgtype.Text{},
	})
	if err != nil {
		return fmt.Errorf("create incremental sync job: %w", err)
	}

	if err := seedImportJobsForCompletedSync(ctx, q, sellerAccountID, incJob.ID, anchorUTC, anchorUTC, domainStats, incStart, incEnd); err != nil {
		return err
	}

	// Cursors: high-water aligned to anchor (post-incremental).
	for _, domain := range MVPSyncImportDomains {
		curPayload := map[string]any{
			"since":                       anchorUTC.Format(time.RFC3339),
			"as_of":                       anchorUTC.Format("2006-01-02"),
			"domain":                      domain,
			"mvp_seed":                    true,
			"last_successful_sync_type":   "incremental_sync",
			"last_successful_sync_job_id": incJob.ID,
		}
		b, _ := json.Marshal(curPayload)
		if _, err := q.UpsertSyncCursor(ctx, dbgen.UpsertSyncCursorParams{
			SellerAccountID: sellerAccountID,
			Domain:          domain,
			CursorType:      "since",
			CursorValue:     pgtype.Text{String: string(b), Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert sync cursor %s: %w", domain, err)
		}
	}

	return nil
}

func seedImportJobsForCompletedSync(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID, syncJobID int64,
	anchorUTC, syncCalendarDay time.Time,
	domainStats map[string]struct{ recv, imp int32 },
	started, finished time.Time,
) error {
	for _, domain := range MVPSyncImportDomains {
		ds := domainStats[domain]
		curPayload := map[string]any{
			"mvp_seed":    true,
			"domain":      domain,
			"anchor_date": anchorUTC.Format("2006-01-02"),
			"sync_day":    syncCalendarDay.Format("2006-01-02"),
			"sync_job_id": syncJobID,
		}
		b, _ := json.Marshal(curPayload)
		_, err := q.CreateImportJob(ctx, dbgen.CreateImportJobParams{
			SellerAccountID: sellerAccountID,
			SyncJobID:       syncJobID,
			Domain:          domain,
			Status:          "completed",
			SourceCursor:    pgtype.Text{String: string(b), Valid: true},
			RecordsReceived: ds.recv,
			RecordsImported: ds.imp,
			RecordsFailed:   0,
			StartedAt:       pgtype.Timestamptz{Time: started, Valid: true},
			FinishedAt:      pgtype.Timestamptz{Time: finished, Valid: true},
			ErrorMessage:    pgtype.Text{},
		})
		if err != nil {
			return fmt.Errorf("create import job %s: %w", domain, err)
		}
	}
	return nil
}
