package advertising

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, now: func() time.Time { return time.Now().UTC() }}
}
func (r *Repository) ResolveAdvertisingSKU(ctx context.Context, seller, sku int64, cutoff time.Time) (*int64, error) {
	var id int64
	e := r.pool.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2 AND first_observed_at<=$3`, seller, sku, cutoff).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &id, nil
}
func (r *Repository) ResolveAdvertisingOrder(ctx context.Context, seller, ozonOrderID int64, cutoff time.Time) (*int64, error) {
	var id int64
	e := r.pool.QueryRow(ctx, `SELECT id FROM sales_orders WHERE seller_account_id=$1 AND ozon_order_id=$2 AND first_observed_at<=$3`, seller, ozonOrderID, cutoff).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &id, nil
}
func (r *Repository) LookupReportRequest(ctx context.Context, seller, run int64, fingerprint string) (ReportRequestObservation, bool, error) {
	var rr ReportRequestObservation
	e := r.pool.QueryRow(ctx, `SELECT source_contract_key,request_fingerprint,COALESCE(provider_report_id,''),status,status_observed_at,result_source_payload_id FROM advertising_report_requests WHERE seller_account_id=$1 AND collection_domain_run_id=$2 AND request_fingerprint=$3`, seller, run, fingerprint).Scan(&rr.SourceContractKey, &rr.RequestFingerprint, &rr.ProviderReportID, &rr.Status, &rr.StatusObservedAt, &rr.ResultSourcePayloadID)
	if errors.Is(e, pgx.ErrNoRows) {
		return ReportRequestObservation{}, false, nil
	}
	if e != nil {
		return ReportRequestObservation{}, false, e
	}
	return rr, true, nil
}
func (r *Repository) UpsertReportRequest(ctx context.Context, seller, run int64, rr ReportRequestObservation) error {
	_, e := r.pool.Exec(ctx, `INSERT INTO advertising_report_requests(seller_account_id,collection_domain_run_id,source_contract_key,request_fingerprint,provider_report_id,status,status_observed_at,result_source_payload_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)ON CONFLICT(collection_domain_run_id,request_fingerprint)DO UPDATE SET provider_report_id=EXCLUDED.provider_report_id,status=EXCLUDED.status,status_observed_at=EXCLUDED.status_observed_at,result_source_payload_id=EXCLUDED.result_source_payload_id`, seller, run, rr.SourceContractKey, rr.RequestFingerprint, rr.ProviderReportID, rr.Status, rr.StatusObservedAt, rr.ResultSourcePayloadID)
	return e
}
func (r *Repository) Finalize(ctx context.Context, seller, run int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("advertising finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, e := lockRun(ctx, tx, seller, run)
		if e != nil || already {
			return e
		}
		at := r.now().UTC()
		campaignIDs := map[int64]int64{}
		for _, c := range result.Campaigns {
			var id int64
			var created bool
			e = tx.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)VALUES($1,$2,$3,$3)ON CONFLICT(seller_account_id,provider_campaign_id)DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at RETURNING id,(xmax=0)`, seller, c.ProviderCampaignID, at).Scan(&id, &created)
			if e != nil {
				return e
			}
			campaignIDs[c.ProviderCampaignID] = id
			if created {
				result.CampaignsCreated++
			}
			h := campaignHash(c)
			var latest int64
			var latestHash string
			e = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM advertising_campaign_versions WHERE advertising_campaign_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latest, &latestHash)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if latestHash == h {
				result.VersionsReused++
				continue
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_campaign_versions(seller_account_id,advertising_campaign_id,version_no,title,provider_state,payment_type,adv_object_type,campaign_family,placement_json,budget,weekly_budget,currency_code,from_date,to_date,strategy,product_autopilot_strategy,is_autocreated,autostop_status,target_scope_type,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,NULLIF($12,''),$13,$14,NULLIF($15,''),NULLIF($16,''),$17,NULLIF($18,''),$19,$20,$21,$22,$23,$24)`, seller, id, latest+1, c.Title, c.ProviderState, c.PaymentType, c.AdvObjectType, c.CampaignFamily, jsonOrEmpty(c.PlacementJSON), decimalString(c.Budget), decimalString(c.WeeklyBudget), c.CurrencyCode, c.FromDate, c.ToDate, c.Strategy, c.ProductAutopilotStrategy, c.IsAutocreated, c.AutostopStatus, c.TargetScopeType, jsonOrEmpty(c.ProviderFields), h, at, c.SourcePayloadID, run)
			if e != nil {
				return e
			}
			result.VersionsWritten++
		}
		seenTargets := map[int64]map[int64]bool{}
		for _, providerID := range result.AuthoritativeTargetCampaignIDs {
			if id, ok := campaignIDs[providerID]; ok {
				if seenTargets[id] == nil {
					seenTargets[id] = map[int64]bool{}
				}
			}
		}
		for _, t := range result.Targets {
			id, ok := campaignIDs[t.ProviderCampaignID]
			if !ok {
				continue
			}
			if seenTargets[id] == nil {
				seenTargets[id] = map[int64]bool{}
			}
			source := int64(0)
			if t.SourceSKU != nil {
				source = *t.SourceSKU
				seenTargets[id][source] = true
			}
			targetState := targetHash(t)
			var open int64
			var openHash string
			e = tx.QueryRow(ctx, `SELECT id,state_hash FROM advertising_target_intervals WHERE advertising_campaign_id=$1 AND COALESCE(source_sku,0)=$2 AND observed_to IS NULL`, id, source).Scan(&open, &openHash)
			if e == nil && openHash == targetState {
				continue
			}
			if !errors.Is(e, pgx.ErrNoRows) {
				if e != nil {
					return e
				}
				if _, e = tx.Exec(ctx, `UPDATE advertising_target_intervals SET observed_to=$2 WHERE id=$1`, open, at); e != nil {
					return e
				}
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_target_intervals(seller_account_id,advertising_campaign_id,canonical_sku_id,source_sku,sku_resolution_status,assignment_state,bid,bid_unit,state_hash,observed_from,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12)`, seller, id, t.CanonicalSKUID, t.SourceSKU, t.SKUResolutionStatus, t.AssignmentState, decimalString(t.Bid), t.BidUnit, targetState, at, t.SourcePayloadID, run)
			if e != nil {
				return e
			}
			result.TargetIntervalsWritten++
		}
		for campaign, seen := range seenTargets {
			rows, e := tx.Query(ctx, `SELECT id,COALESCE(source_sku,0)FROM advertising_target_intervals WHERE advertising_campaign_id=$1 AND observed_to IS NULL`, campaign)
			if e != nil {
				return e
			}
			var closeIDs []int64
			for rows.Next() {
				var id, sku int64
				if e = rows.Scan(&id, &sku); e != nil {
					rows.Close()
					return e
				}
				if !seen[sku] {
					closeIDs = append(closeIDs, id)
				}
			}
			rows.Close()
			if len(closeIDs) > 0 {
				_, e = tx.Exec(ctx, `UPDATE advertising_target_intervals SET observed_to=$2 WHERE advertising_campaign_id=$1 AND id=ANY($3)`, campaign, at, closeIDs)
				if e != nil {
					return e
				}
			}
		}
		for _, d := range result.Daily {
			campaign, ok := campaignIDs[d.ProviderCampaignID]
			if !ok {
				continue
			}
			h := dailyHash(d)
			var latest int64
			var latestHash string
			e = tx.QueryRow(ctx, `SELECT revision_no,normalized_day_hash FROM advertising_daily_facts WHERE advertising_campaign_id=$1 AND business_date=$2 AND attribution_kind=$3 ORDER BY revision_no DESC LIMIT 1`, campaign, d.BusinessDate, d.AttributionKind).Scan(&latest, &latestHash)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if latestHash == h {
				result.DailyRevisionsReused++
				continue
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_daily_facts(seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,bonus_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,provider_drr,currency_code,traffic_state,attribution_state,normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, seller, campaign, d.BusinessDate, latest+1, d.Impressions, d.Clicks, decimalString(d.PaidSpendAmount), decimalString(d.BonusSpendAmount), d.AttributedOrderUnits, decimalString(d.AttributedOrderAmount), d.AttributionKind, decimalString(d.ProviderDRR), d.CurrencyCode, d.TrafficState, d.AttributionState, h, DailyContract.ContractKey, d.SourcePayloadID, run, at)
			if e != nil {
				return e
			}
			result.DailyRevisionsWritten++
		}
		for _, d := range result.ProductDaily {
			campaign, ok := campaignIDs[d.ProviderCampaignID]
			if !ok {
				continue
			}
			h := productDailyHash(d)
			var latest int64
			var latestHash string
			e = tx.QueryRow(ctx, `SELECT revision_no,state_hash FROM advertising_product_daily_facts WHERE advertising_campaign_id=$1 AND business_date=$2 AND source_promoted_sku=$3 AND attribution_kind=$4 ORDER BY revision_no DESC LIMIT 1`, campaign, d.BusinessDate, d.SourcePromotedSKU, d.AttributionKind).Scan(&latest, &latestHash)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if latestHash == h {
				result.ProductRevisionsReused++
				continue
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_product_daily_facts(seller_account_id,advertising_campaign_id,business_date,promoted_canonical_sku_id,source_promoted_sku,sku_resolution_status,impressions,clicks,paid_spend_amount,bonus_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,revision_no,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, seller, campaign, d.BusinessDate, d.CanonicalSKUID, d.SourcePromotedSKU, d.SKUResolutionStatus, d.Impressions, d.Clicks, decimalString(d.PaidSpendAmount), decimalString(d.BonusSpendAmount), d.AttributedOrderUnits, decimalString(d.AttributedOrderAmount), d.AttributionKind, d.CurrencyCode, latest+1, h, at, d.SourcePayloadID, run)
			if e != nil {
				return e
			}
			result.ProductRevisionsWritten++
		}
		dailyPaidByDate := map[string]*sales.Decimal{}
		dailyCurrency := map[string]string{}
		for _, d := range result.Daily {
			key := d.BusinessDate.Format("2006-01-02")
			dailyCurrency[key] = d.CurrencyCode
			if d.PaidSpendAmount == nil {
				continue
			}
			if cur := dailyPaidByDate[key]; cur == nil {
				cp := *d.PaidSpendAmount
				dailyPaidByDate[key] = &cp
			} else {
				sum := cur.Add(*d.PaidSpendAmount)
				dailyPaidByDate[key] = &sum
			}
		}
		expenseByDate := map[string]ExpenseObservation{}
		for _, eobs := range result.Expense {
			expenseByDate[eobs.BusinessDate.Format("2006-01-02")] = eobs
		}
		dates := map[string]time.Time{}
		for _, d := range result.Daily {
			dates[d.BusinessDate.Format("2006-01-02")] = d.BusinessDate
		}
		for _, eobs := range result.Expense {
			dates[eobs.BusinessDate.Format("2006-01-02")] = eobs.BusinessDate
		}
		for key, day := range dates {
			dailyPaid := dailyPaidByDate[key]
			expense := expenseByDate[key]
			status := "PENDING_ACCEPTANCE"
			details := map[string]any{}
			currency := dailyCurrency[key]
			if currency == "" {
				currency = expense.CurrencyCode
			}
			if expense.PaidSpendAmount != nil && dailyPaid != nil {
				if expense.CurrencyCode != "" && currency != "" && expense.CurrencyCode != currency {
					status = "NOT_COMPARABLE"
					details["reason"] = "CURRENCY_MISMATCH"
				} else if expense.PaidSpendAmount.String() == dailyPaid.String() {
					status = "PASS"
				} else {
					status = "DIFFERENT"
					details["daily_paid"] = dailyPaid.String()
					details["expense_paid"] = expense.PaidSpendAmount.String()
				}
			} else if expense.PaidSpendAmount == nil {
				status = "PENDING_ACCEPTANCE"
				details["reason"] = "EXPENSE_SOURCE_UNAVAILABLE_OR_EMPTY"
			}
			detailsJSON, _ := json.Marshal(details)
			var payload any
			if expense.SourcePayloadID > 0 {
				payload = expense.SourcePayloadID
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_spend_reconciliations(seller_account_id,business_date,daily_paid_spend,expense_paid_spend,currency_code,reconciliation_status,details_json,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10)`, seller, day, decimalString(dailyPaid), decimalString(expense.PaidSpendAmount), currency, status, detailsJSON, at, payload, run)
			if e != nil {
				return e
			}
			result.ExpenseRowsWritten++
		}
		for _, cpo := range result.CPO {
			campaign, ok := campaignIDs[cpo.ProviderCampaignID]
			if !ok {
				continue
			}
			h := cpoHash(cpo)
			providerKey := cpoProviderFactKey(cpo)
			var latest int64
			var latestHash string
			e = tx.QueryRow(ctx, `SELECT revision_no,state_hash FROM advertising_cpo_facts WHERE seller_account_id=$1 AND provider_fact_key=$2 ORDER BY revision_no DESC LIMIT 1`, seller, providerKey).Scan(&latest, &latestHash)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if latestHash == h {
				result.CPORowsReused++
				continue
			}
			_, e = tx.Exec(ctx, `INSERT INTO advertising_cpo_facts(seller_account_id,advertising_campaign_id,business_date,promoted_canonical_sku_id,ordered_canonical_sku_id,canonical_order_id,source_promoted_sku,source_ordered_sku,source_order_id,promoted_sku_resolution_status,ordered_sku_resolution_status,order_resolution_status,attributed_order_units,attributed_order_amount,currency_code,attribution_kind,state_hash,observed_at,source_payload_id,collection_domain_run_id,revision_no,provider_fact_key)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''),$16,$17,$18,$19,$20,$21,$22)`, seller, campaign, cpo.BusinessDate, cpo.PromotedCanonicalSKUID, cpo.OrderedCanonicalSKUID, cpo.CanonicalOrderID, cpo.SourcePromotedSKU, cpo.SourceOrderedSKU, cpo.SourceOrderID, cpo.PromotedSKUResolutionStatus, cpo.OrderedSKUResolutionStatus, cpo.OrderResolutionStatus, cpo.AttributedOrderUnits, decimalString(cpo.AttributedOrderAmount), cpo.CurrencyCode, cpo.AttributionKind, h, at, cpo.SourcePayloadID, run, latest+1, providerKey)
			if e != nil {
				return e
			}
			result.CPORowsWritten++
		}
		for _, rr := range result.ReportRequests {
			_, e = tx.Exec(ctx, `INSERT INTO advertising_report_requests(seller_account_id,collection_domain_run_id,source_contract_key,request_fingerprint,provider_report_id,status,status_observed_at,result_source_payload_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)ON CONFLICT(collection_domain_run_id,request_fingerprint)DO UPDATE SET provider_report_id=EXCLUDED.provider_report_id,status=EXCLUDED.status,status_observed_at=EXCLUDED.status_observed_at,result_source_payload_id=EXCLUDED.result_source_payload_id`, seller, run, rr.SourceContractKey, rr.RequestFingerprint, rr.ProviderReportID, rr.Status, rr.StatusObservedAt, rr.ResultSourcePayloadID)
			if e != nil {
				return e
			}
			result.ReportRequestsWritten++
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" {
			status = "PARTIAL"
		}
		cmd, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,coverage_from=$4,coverage_to=$5,records_received=$6,records_written=$7,pages_received=$8,completeness_status=$9,finished_at=$10,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, result.RequestedFrom, result.RequestedTo, len(result.Campaigns)+len(result.Daily)+len(result.Targets)+len(result.ProductDaily)+len(result.Expense)+len(result.CPO), result.VersionsWritten+result.DailyRevisionsWritten+result.TargetIntervalsWritten+result.ProductRevisionsWritten+result.ExpenseRowsWritten+result.CPORowsWritten, result.CampaignPages+result.DailyBatches+result.TargetPages+result.ProductBatches+result.ExpenseBatches+result.CPOReports, result.CompletenessStatus, at)
		if e != nil {
			return e
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale advertising domain transition")
		}
		return advanceHead(ctx, tx, seller, run, at)
	})
}
func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:ADVERTISING:%d", seller)); e != nil {
		return false, e
	}
	var status, domain string
	if e := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); e != nil {
		return false, e
	}
	if domain != "ADVERTISING" {
		return false, errors.New("expected ADVERTISING domain")
	}
	var head int64
	e := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='ADVERTISING'`, seller).Scan(&head)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return false, e
	}
	if (status == "COMPLETED" || status == "PARTIAL") && e == nil && head >= run {
		return true, nil
	}
	if e == nil && head > run {
		_, e = tx.Exec(ctx, `UPDATE collection_domain_runs SET status='CANCELLED',finished_at=NOW(),error_code='SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN'WHERE id=$1`, run)
		return false, e
	}
	if status != "RUNNING" {
		return false, fmt.Errorf("ADVERTISING domain must be RUNNING, got %s", status)
	}
	return false, nil
}
func advanceHead(ctx context.Context, tx pgx.Tx, seller, run int64, at time.Time) error {
	_, e := tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'ADVERTISING',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, at)
	return e
}
func decimalString(v *sales.Decimal) any {
	if v == nil {
		return nil
	}
	return v.String()
}
func jsonOrEmpty(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(`{}`)
	}
	return v
}
func campaignHash(c CampaignObservation) string {
	return factualapi.StableHash(c.Title, c.ProviderState, c.PaymentType, c.AdvObjectType, c.CampaignFamily, string(c.PlacementJSON), fmt.Sprint(decimalString(c.Budget)), fmt.Sprint(decimalString(c.WeeklyBudget)), c.CurrencyCode, fmt.Sprint(c.FromDate), fmt.Sprint(c.ToDate), c.Strategy, c.ProductAutopilotStrategy, fmt.Sprint(c.IsAutocreated), c.AutostopStatus, c.TargetScopeType, string(c.ProviderFields))
}
func targetHash(t TargetObservation) string {
	return factualapi.StableHash(fmt.Sprint(t.CanonicalSKUID), fmt.Sprint(t.SourceSKU), t.SKUResolutionStatus, t.AssignmentState, fmt.Sprint(decimalString(t.Bid)), t.BidUnit)
}
func dailyHash(d DailyObservation) string {
	return factualapi.StableHash(d.BusinessDate.Format("2006-01-02"), fmt.Sprint(d.Impressions), fmt.Sprint(d.Clicks), fmt.Sprint(decimalString(d.PaidSpendAmount)), fmt.Sprint(decimalString(d.BonusSpendAmount)), fmt.Sprint(d.AttributedOrderUnits), fmt.Sprint(decimalString(d.AttributedOrderAmount)), fmt.Sprint(decimalString(d.ProviderDRR)), d.AttributionKind, d.CurrencyCode, d.TrafficState, d.AttributionState)
}
func productDailyHash(d ProductDailyObservation) string {
	return factualapi.StableHash(d.BusinessDate.Format("2006-01-02"), fmt.Sprint(d.SourcePromotedSKU), fmt.Sprint(d.Impressions), fmt.Sprint(d.Clicks), fmt.Sprint(decimalString(d.PaidSpendAmount)), fmt.Sprint(decimalString(d.BonusSpendAmount)), fmt.Sprint(d.AttributedOrderUnits), fmt.Sprint(decimalString(d.AttributedOrderAmount)), d.AttributionKind, d.CurrencyCode)
}
func cpoHash(c CPOObservation) string {
	return factualapi.StableHash(c.BusinessDate.Format("2006-01-02"), fmt.Sprint(c.ProviderCampaignID), fmt.Sprint(c.SourcePromotedSKU), fmt.Sprint(c.SourceOrderedSKU), fmt.Sprint(c.SourceOrderID), fmt.Sprint(c.AttributedOrderUnits), fmt.Sprint(decimalString(c.AttributedOrderAmount)), c.AttributionKind, c.CurrencyCode)
}
func cpoProviderFactKey(c CPOObservation) string {
	return factualapi.StableHash(fmt.Sprint(c.ProviderCampaignID), c.BusinessDate.Format("2006-01-02"), fmt.Sprint(c.SourceOrderID), fmt.Sprint(c.SourcePromotedSKU), fmt.Sprint(c.SourceOrderedSKU), c.AttributionKind)
}
