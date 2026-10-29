package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return NewRepositoryWithClock(pool, func() time.Time { return time.Now().UTC() })
}
func NewRepositoryWithClock(pool *pgxpool.Pool, now func() time.Time) *Repository {
	return &Repository{pool: pool, now: now}
}

type SupersededRunError struct{ DomainRunID, SupersededBy int64 }

func (e *SupersededRunError) Error() string {
	return fmt.Sprintf("sales demand domain run %d superseded by %d", e.DomainRunID, e.SupersededBy)
}

func (r *Repository) Finalize(ctx context.Context, sellerID, domainRunID int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("sales finalization requires pool and result")
	}
	if result.Currency == "" {
		return errors.New("sales result currency is empty")
	}
	var superseded int64
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("sales-demand:%d", sellerID)); e != nil {
			return e
		}
		var status, domain string
		if e := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, domainRunID, sellerID).Scan(&status, &domain); e != nil {
			return e
		}
		if domain != "SALES_DEMAND" {
			return fmt.Errorf("domain run is %s, expected SALES_DEMAND", domain)
		}
		var head int64
		headErr := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='SALES_DEMAND'`, sellerID).Scan(&head)
		if headErr != nil && !errors.Is(headErr, pgx.ErrNoRows) {
			return headErr
		}
		if (status == "COMPLETED" || status == "PARTIAL") && headErr == nil && head >= domainRunID {
			return nil
		}
		if status != "RUNNING" {
			return fmt.Errorf("sales domain run must be RUNNING, got %s", status)
		}
		if headErr == nil && head > domainRunID {
			superseded = head
			_, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status='CANCELLED',finished_at=NOW(),error_code='SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN',error_message=$3,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, domainRunID, sellerID, fmt.Sprintf("newer sales demand run %d finalized", head))
			return e
		}
		// One acceptance boundary for every fact/reconciliation revision in this
		// atomic finalization. Source receipt remains on source_payloads.
		acceptanceAt := r.now().UTC()

		sort.Slice(result.sku, func(i, j int) bool {
			if result.sku[i].Date.Equal(result.sku[j].Date) {
				return result.sku[i].SourceSKU < result.sku[j].SourceSKU
			}
			return result.sku[i].Date.Before(result.sku[j].Date)
		})
		resolved := make(map[string]bool, len(result.sku))
		for _, c := range result.sku {
			var canonicalID *int64
			var id int64
			e := tx.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2`, sellerID, c.SourceSKU).Scan(&id)
			if e == nil {
				canonicalID = &id
				resolved[skuKey(c)] = true
			} else if !errors.Is(e, pgx.ErrNoRows) {
				return e
			} else {
				result.UnresolvedSKUs++
			}
			resolvedIdentity := "UNRESOLVED"
			if canonicalID != nil {
				resolvedIdentity = fmt.Sprintf("RESOLVED:%d", *canonicalID)
			}
			h := factHash(fmt.Sprintf("sku|%d|%d|%s|%s|%d|%s|%s", sellerID, c.SourceSKU, c.Date.Format("2006-01-02"), c.Amount.String(), c.Units, result.Currency, resolvedIdentity))
			var latestHash string
			var latestRev int64
			e = tx.QueryRow(ctx, `SELECT fact_hash,revision_no FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=$2 AND business_date=$3 ORDER BY revision_no DESC LIMIT 1`, sellerID, c.SourceSKU, c.Date).Scan(&latestHash, &latestRev)
			if e == nil && latestHash == h {
				result.ReusedFacts++
				continue
			}
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			resolution := "UNRESOLVED"
			if canonicalID != nil {
				resolution = "RESOLVED"
			}
			if _, e = tx.Exec(ctx, `INSERT INTO sales_demand_daily(seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, sellerID, canonicalID, c.SourceSKU, c.Date, c.Amount.String(), c.Units, result.Currency, resolution, latestRev+1, h, domainRunID, c.PayloadID, acceptanceAt); e != nil {
				return e
			}
			result.RevisionsWritten++
		}
		for _, c := range result.account {
			h := factHash(fmt.Sprintf("account|%d|%s|%s|%d|%s", sellerID, c.Date.Format("2006-01-02"), c.Amount.String(), c.Units, result.Currency))
			var latestHash string
			var latestRev int64
			e := tx.QueryRow(ctx, `SELECT fact_hash,revision_no FROM sales_account_daily WHERE seller_account_id=$1 AND business_date=$2 ORDER BY revision_no DESC LIMIT 1`, sellerID, c.Date).Scan(&latestHash, &latestRev)
			if e == nil && latestHash == h {
				result.ReusedFacts++
				continue
			}
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO sales_account_daily(seller_account_id,business_date,ordered_amount,ordered_units,currency_code,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, sellerID, c.Date, c.Amount.String(), c.Units, result.Currency, latestRev+1, h, domainRunID, c.PayloadID, acceptanceAt); e != nil {
				return e
			}
			result.RevisionsWritten++
		}
		limited, e := reconcile(ctx, tx, sellerID, domainRunID, result, resolved, acceptanceAt)
		if e != nil {
			return e
		}
		finalStatus := "COMPLETED"
		completeness := "COMPLETE"
		if limited {
			finalStatus = "PARTIAL"
			completeness = "PARTIAL"
		}
		result.RecordsWritten = result.RevisionsWritten + (daysInclusive(result.RequestedFrom, result.RequestedTo))
		coverageFrom, coverageTo := accountCoverage(result.account)
		cmd, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,coverage_from=$4,coverage_to=$5,records_received=$6,records_written=$7,pages_received=$8,completeness_status=$9,finished_at=$10,error_code=NULL,error_message=NULL,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, domainRunID, sellerID, finalStatus, coverageFrom, coverageTo, result.RecordsReceived, result.RecordsWritten, result.Pages, completeness, acceptanceAt)
		if e != nil {
			return e
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale sales domain transition")
		}
		_, e = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at) VALUES($1,'SALES_DEMAND',$2,$3) ON CONFLICT(seller_account_id,domain) DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW() WHERE collection_authoritative_heads.collection_domain_run_id <= EXCLUDED.collection_domain_run_id`, sellerID, domainRunID, acceptanceAt)
		return e
	})
	if err != nil {
		return err
	}
	if superseded > 0 {
		return &SupersededRunError{domainRunID, superseded}
	}
	return nil
}

type totals struct {
	all, resolved, unresolved                Decimal
	allUnits, resolvedUnits, unresolvedUnits int64
	rows, resolvedRows, unresolvedRows       int64
}

func reconcile(ctx context.Context, tx pgx.Tx, sellerID, runID int64, r *Result, resolved map[string]bool, acceptanceAt time.Time) (bool, error) {
	byDay := map[string]*totals{}
	skuByDay := map[string][]candidate{}
	accounts := map[string]accountCandidate{}
	for _, c := range r.sku {
		k := c.Date.Format("2006-01-02")
		skuByDay[k] = append(skuByDay[k], c)
		t := byDay[k]
		if t == nil {
			t = &totals{all: Zero(), resolved: Zero(), unresolved: Zero()}
			byDay[k] = t
		}
		t.all = t.all.Add(c.Amount)
		t.allUnits += c.Units
		t.rows++
		if resolved[skuKey(c)] {
			t.resolved = t.resolved.Add(c.Amount)
			t.resolvedUnits += c.Units
			t.resolvedRows++
		} else {
			t.unresolved = t.unresolved.Add(c.Amount)
			t.unresolvedUnits += c.Units
			t.unresolvedRows++
		}
	}
	for _, a := range r.account {
		accounts[a.Date.Format("2006-01-02")] = a
	}
	limited := false
	for d := r.RequestedFrom; !d.After(r.RequestedTo); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		t := byDay[k]
		if t == nil {
			t = &totals{all: Zero(), resolved: Zero(), unresolved: Zero()}
		}
		a, ok := accounts[k]
		amountStatus, unitsStatus := "NOT_COMPARABLE", "NOT_COMPARABLE"
		var aa any
		var au any
		var gapAllA, gapResolvedA, gapAllU, gapResolvedU any
		if ok {
			aa = a.Amount.String()
			au = a.Units
			gapAllA = a.Amount.SubString(t.all)
			gapResolvedA = a.Amount.SubString(t.resolved)
			gapAllU = a.Units - t.allUnits
			gapResolvedU = a.Units - t.resolvedUnits
			amountStatus = measureStatus(a.Amount.Equal(t.all), !t.unresolved.IsZero())
			unitsStatus = measureStatus(a.Units == t.allUnits, t.unresolvedUnits != 0)
		}
		overall := overallStatus(amountStatus, unitsStatus)
		datasetHash := normalizedDayHash(sellerID, k, r.Currency, a, ok, skuByDay[k], resolved)
		if overall != "PASS" {
			limited = true
		}
		switch overall {
		case "PASS":
			r.ReconciliationPASS++
		case "EXPLAINED_GAP":
			r.ReconciliationExplained++
		case "UNEXPLAINED_GAP":
			r.ReconciliationUnexplained++
		default:
			r.ReconciliationNotComparable++
		}
		_, e := tx.Exec(ctx, `INSERT INTO sales_reconciliations(seller_account_id,business_date,currency_code,provider_account_ordered_amount,provider_account_ordered_units,source_sku_ordered_amount,source_sku_ordered_units,resolved_sku_ordered_amount,resolved_sku_ordered_units,unresolved_ordered_amount,unresolved_ordered_units,source_sku_row_count,resolved_sku_row_count,unresolved_sku_row_count,amount_gap_account_vs_all_source,units_gap_account_vs_all_source,amount_gap_account_vs_resolved,units_gap_account_vs_resolved,amount_status,units_status,overall_status,collection_domain_run_id,observed_at,normalized_day_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`, sellerID, d, r.Currency, aa, au, t.all.String(), t.allUnits, t.resolved.String(), t.resolvedUnits, t.unresolved.String(), t.unresolvedUnits, t.rows, t.resolvedRows, t.unresolvedRows, gapAllA, gapAllU, gapResolvedA, gapResolvedU, amountStatus, unitsStatus, overall, runID, acceptanceAt, datasetHash)
		if e != nil {
			return false, e
		}
	}
	if r.UnresolvedSKUs > 0 {
		limited = true
	}
	return limited, nil
}

func normalizedDayHash(sellerID int64, day, currency string, account accountCandidate, hasAccount bool, rows []candidate, resolved map[string]bool) string {
	parts := []string{fmt.Sprintf("seller=%d", sellerID), "day=" + day, "currency=" + currency}
	if hasAccount {
		parts = append(parts, "account="+account.Amount.String()+"/"+fmt.Sprint(account.Units))
	} else {
		parts = append(parts, "account=MISSING")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SourceSKU < rows[j].SourceSKU })
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("sku=%d/%s/%d/resolved=%t", row.SourceSKU, row.Amount.String(), row.Units, resolved[skuKey(row)]))
	}
	return factHash(strings.Join(parts, "|"))
}
func measureStatus(sourceEqual bool, unresolved bool) string {
	if !sourceEqual {
		return "UNEXPLAINED_GAP"
	}
	if unresolved {
		return "EXPLAINED_GAP"
	}
	return "PASS"
}
func overallStatus(a, u string) string {
	for _, s := range []string{"UNEXPLAINED_GAP", "NOT_COMPARABLE", "EXPLAINED_GAP"} {
		if a == s || u == s {
			return s
		}
	}
	return "PASS"
}
func factHash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func skuKey(c candidate) string {
	return fmt.Sprintf("%d/%s", c.SourceSKU, c.Date.Format("2006-01-02"))
}
func accountCoverage(rows []accountCandidate) (*time.Time, *time.Time) {
	if len(rows) == 0 {
		return nil, nil
	}
	from, to := rows[0].Date, rows[0].Date
	for _, row := range rows[1:] {
		if row.Date.Before(from) {
			from = row.Date
		}
		if row.Date.After(to) {
			to = row.Date
		}
	}
	return &from, &to
}
func daysInclusive(a, b time.Time) int { return int(b.Sub(a).Hours()/24) + 1 }
