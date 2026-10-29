package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool      *pgxpool.Pool
	clock     Clock
	semantics SemanticRegistry
}

func NewService(pool *pgxpool.Pool, clock Clock, semantics SemanticRegistry) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	if semantics == nil {
		semantics = ProductionSemanticRegistry{}
	}
	return &Service{pool, clock, semantics}
}

type domainEvidence struct {
	domain, status, completeness, contractKey, contractVersion            string
	endpointFamily, mappingVersion, semanticMappingHash, providerRevision string
	runID                                                                 int64
	finalized, finishedAt                                                 time.Time
	coverageFrom, coverageTo                                              *time.Time
	records                                                               int64
}

func (s *Service) Evaluate(ctx context.Context, seller int64) (Result, error) {
	var out Result
	if s == nil || s.pool == nil || seller <= 0 {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "seller and readiness repository are required")
	}
	cutoff := s.clock.Now().UTC()
	out.SellerAccountID, out.CutoffAt, out.Policies = seller, cutoff, factualapi.CurrentPolicies()
	var exists bool
	if e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seller_accounts WHERE id=$1)`, seller).Scan(&exists); e != nil || !exists {
		if e == nil {
			return out, factualapi.NewError(factualapi.CodeNotFound, "seller not found")
		}
		return out, e
	}
	domains, e := s.domainEvidence(ctx, seller, cutoff)
	if e != nil {
		return out, e
	}
	inputParts := []string{fmt.Sprint(seller), cutoff.Format(time.RFC3339Nano), out.Policies.DQ}
	for _, d := range domains {
		inputParts = append(inputParts, d.domain, fmt.Sprint(d.runID), d.finalized.Format(time.RFC3339Nano), d.status, d.completeness)
	}
	out.InputHash = factualapi.StableHash(inputParts...)
	if e = s.pool.QueryRow(ctx, `WITH inserted AS(INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,status,started_at)VALUES($1,$2,$3,$4,'RUNNING',$2)ON CONFLICT DO NOTHING RETURNING id)SELECT id FROM inserted UNION ALL SELECT id FROM data_quality_runs WHERE seller_account_id=$1 AND cutoff_at=$2 AND policy_version=$3 AND input_hash=$4 LIMIT 1`, seller, cutoff, out.Policies.DQ, out.InputHash).Scan(&out.DQRunID); e != nil {
		return out, e
	}
	stats, e := s.stats(ctx, seller, cutoff)
	if e != nil {
		return out, e
	}
	out.Watermarks = s.watermarks(domains, stats, cutoff)
	out.Issues = s.issues(domains, stats, cutoff)
	sort.Slice(out.Issues, func(i, j int) bool {
		a, b := out.Issues[i], out.Issues[j]
		return a.Domain+a.Dimension+a.Code+tstr(a.PeriodStart) < b.Domain+b.Dimension+b.Code+tstr(b.PeriodStart)
	})
	out.Capabilities = s.capabilities(domains, stats, cutoff)
	for i := range out.Watermarks {
		if e = s.persistWatermark(ctx, seller, &out.Watermarks[i]); e != nil {
			return out, e
		}
	}
	for i := range out.Issues {
		if e = s.persistIssue(ctx, seller, out.DQRunID, out.Issues[i], cutoff); e != nil {
			return out, e
		}
	}
	for i := range out.Capabilities {
		if e = s.persistCapability(ctx, seller, out.DQRunID, &out.Capabilities[i]); e != nil {
			return out, e
		}
	}
	out.Candidates, out.AnalysisDate, e = s.resolveDates(ctx, seller, out.DQRunID, cutoff, stats, out.Capabilities, out.Watermarks)
	if e != nil {
		return out, e
	}
	parts := []string{out.InputHash}
	for _, w := range out.Watermarks {
		parts = append(parts, w.StateHash)
	}
	for _, i := range out.Issues {
		b, _ := json.Marshal(i)
		parts = append(parts, string(b))
	}
	for _, c := range out.Capabilities {
		parts = append(parts, c.Key, c.Scope, c.FinalState, c.SemanticState)
	}
	for _, c := range out.Candidates {
		d := ""
		if c.Date != nil {
			d = c.Date.Format("2006-01-02")
		}
		parts = append(parts, d, c.Status, fmt.Sprint(c.Selected), strings.Join(c.BlockingReasons, ","))
	}
	out.ResultHash = factualapi.StableHash(parts...)
	if _, e = s.pool.Exec(ctx, `UPDATE data_quality_runs SET result_hash=$2,status='COMPLETED',completed_at=$3 WHERE id=$1`, out.DQRunID, out.ResultHash, cutoff); e != nil {
		return out, e
	}
	if out.AnalysisDate != nil {
		fc, e := s.buildContext(out, domains)
		if e != nil {
			return out, e
		}
		out.Context = &fc
	}
	return out, nil
}

func (s *Service) domainEvidence(ctx context.Context, seller int64, cutoff time.Time) ([]domainEvidence, error) {
	rows, e := s.pool.Query(ctx, `SELECT h.domain,h.collection_domain_run_id,h.finalized_at,dr.status,COALESCE(dr.completeness_status,''),COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,''),COALESCE(dr.semantic_mapping_hash,''),COALESCE(dr.source_revision_hash,''),dr.coverage_from,dr.coverage_to,dr.records_received,COALESCE(dr.source_endpoint_family,''),COALESCE(dr.semantic_mapping_version,''),COALESCE(dr.finished_at,h.finalized_at) FROM collection_authoritative_heads h JOIN collection_domain_runs dr ON dr.id=h.collection_domain_run_id WHERE h.seller_account_id=$1 AND h.finalized_at<=$2 AND dr.status IN('COMPLETED','PARTIAL') ORDER BY h.domain`, seller, cutoff)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domainEvidence
	for rows.Next() {
		var d domainEvidence
		var from, to pgtype.Date
		if e = rows.Scan(&d.domain, &d.runID, &d.finalized, &d.status, &d.completeness, &d.contractKey, &d.contractVersion, &d.semanticMappingHash, &d.providerRevision, &from, &to, &d.records, &d.endpointFamily, &d.mappingVersion, &d.finishedAt); e != nil {
			return nil, e
		}
		if from.Valid {
			v := from.Time
			d.coverageFrom = &v
		}
		if to.Valid {
			v := to.Time
			d.coverageTo = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type evidenceStats struct {
	skuCount, mappedCount                                                                 int64
	categoryExposureTotal, categoryExposureMapped                                         string
	salesFrom, salesTo, maxAccountDate                                                    *time.Time
	salesCompleteThrough                                                                  *time.Time
	salesDays                                                                             int
	latestSalesObserved                                                                   *time.Time
	matureDates                                                                           map[string]bool
	salesDates                                                                            map[string]bool
	reconciliation                                                                        map[string]string
	currentDomainState                                                                    map[string]string
	currentDomainAccess                                                                   map[string]string
	currentDomainError                                                                    map[string]string
	connectionAccess                                                                      string
	canonicalOptionalAccess                                                               string
	postingCount, returnCount, unknownEvents                                              int64
	missingSalesDays, mixedCurrencyDays, missingPostingSKUs                               int64
	postingOrderResolutionIssues, postingSKUResolutionIssues, returnResolutionIssues      int64
	productTypeResolutionIssues                                                           int64
	explainedSalesDays                                                                    int64
	latestPostingObserved, latestReturnObserved                                           *time.Time
	latestStockObserved, latestPriceObserved, latestAdvertisingObserved                   *time.Time
	latestStockComplete, latestPriceComplete                                              *time.Time
	adTrafficCompleteThrough                                                              *time.Time
	stockSnapshotCount, stockPositionCount, stockUnresolved, stockAmbiguousPools          int64
	stockPendingSemantics, stockFailedBatches, stockMissingResponse                       int64
	priceSnapshotCount, priceResolved, priceUnresolved, priceInvalid                      int64
	pricePendingSemantics, priceMixedCurrencies                                           int64
	adCampaignCount, adDailyCount, adUnknownFamilies, adUnresolvedTargets                 int64
	adMixedCurrencies, adPendingAttribution, adReconciliationIssues                       int64
	adUnknownTargetScopes, adCPOUnavailable                                               int64
	adHistoryFrom, adHistoryTo, adMatureThrough                                           *time.Time
	funnelAdditiveCount, funnelDirectCount                                                int64
	funnelLatestObserved                                                                  *time.Time
	funnelAdditiveFrom, funnelAdditiveTo, funnelDirectFrom, funnelDirectTo                *time.Time
	financeLatestObserved, financeBalanceLatestObserved                                   *time.Time
	financeCompleteThrough, financeHistoryFrom, financeHistoryTo                          *time.Time
	financeDayCount, financePartialDays, financeUnmappedTypes, financeMixedCurrencies     int64
	financeBalanceCount                                                                   int64
	searchSummaryCount, searchDetailCount, searchMarketCount, searchUnresolvedDetails     int64
	searchSummaryObserved, searchDetailObserved, searchMarketObserved                     *time.Time
	searchSummaryFrom, searchSummaryTo, searchDetailFrom, searchDetailTo                  *time.Time
	searchMarketFrom, searchMarketTo                                                      *time.Time
	healthSnapshotCount, healthHistoryCount, healthThresholdCount                         int64
	healthPenaltyCount, healthFBSIndexCount, healthFBSErrorCount                          int64
	healthUnmappedMetrics, healthUnresolvedPostings                                       int64
	healthSnapshotObserved, healthHistoryObserved, healthFBSIndexObserved                 *time.Time
	healthFBSErrorObserved, healthHistoryFrom, healthHistoryTo                            *time.Time
	supplyStateCount, supplyBundleCount, supplyAcceptanceCount, supplyAcceptanceItemCount int64
	supplyPartialBundles, supplyUnknownStates, supplyAcceptancePartial                    int64
	supplyStateObserved, supplyBundleObserved, supplyBundleCompleteObserved               *time.Time
	supplyAcceptanceObserved, supplyAcceptanceCompleteObserved                            *time.Time
}

func (s *Service) stats(ctx context.Context, seller int64, cutoff time.Time) (evidenceStats, error) {
	st := evidenceStats{matureDates: map[string]bool{}, salesDates: map[string]bool{}, reconciliation: map[string]string{}, currentDomainState: map[string]string{}, currentDomainAccess: map[string]string{}, currentDomainError: map[string]string{}, connectionAccess: "UNKNOWN", canonicalOptionalAccess: "UNKNOWN", categoryExposureTotal: "0", categoryExposureMapped: "0"}
	e := s.pool.QueryRow(ctx, `WITH current AS(SELECT cs.id,(SELECT v.catalog_state FROM canonical_sku_versions v WHERE v.canonical_sku_id=cs.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)state FROM canonical_skus cs WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2)SELECT count(*)FILTER(WHERE state IS NOT NULL AND state<>'UNOBSERVED'),count(*)FILTER(WHERE state IS NOT NULL AND state<>'UNOBSERVED' AND EXISTS(SELECT 1 FROM sku_category_assignments a WHERE a.canonical_sku_id=current.id AND a.observed_from<=$2 AND(a.observed_to IS NULL OR a.observed_to>$2)AND a.mapping_status='MAPPED'))FROM current`, seller, cutoff).Scan(&st.skuCount, &st.mappedCount)
	if e != nil {
		return st, e
	}
	var sf, sto pgtype.Date
	var obs pgtype.Timestamptz
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(business_date)business_date,observed_at FROM sales_account_daily WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)SELECT min(business_date),max(business_date),count(*),max(observed_at)FROM latest`, seller, cutoff).Scan(&sf, &sto, &st.salesDays, &obs)
	if e != nil {
		return st, e
	}
	if sf.Valid {
		v := sf.Time
		st.salesFrom = &v
	}
	if sto.Valid {
		v := sto.Time
		st.salesTo = &v
		st.maxAccountDate = &v
	}
	if obs.Valid {
		v := obs.Time
		st.latestSalesObserved = &v
	}
	dateRows, e := s.pool.Query(ctx, `SELECT DISTINCT business_date FROM sales_account_daily WHERE seller_account_id=$1 AND observed_at<=$2`, seller, cutoff)
	if e != nil {
		return st, e
	}
	for dateRows.Next() {
		var d time.Time
		if e = dateRows.Scan(&d); e != nil {
			dateRows.Close()
			return st, e
		}
		st.salesDates[d.Format("2006-01-02")] = true
	}
	if e = dateRows.Err(); e != nil {
		dateRows.Close()
		return st, e
	}
	dateRows.Close()
	rows, e := s.pool.Query(ctx, `SELECT r.business_date,r.overall_status,r.normalized_day_hash,r.collection_domain_run_id FROM sales_reconciliations r JOIN collection_domain_runs dr ON dr.id=r.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.observed_at<=$2 AND dr.status IN('COMPLETED','PARTIAL') ORDER BY r.business_date,r.observed_at DESC,r.id DESC`, seller, cutoff)
	if e != nil {
		return st, e
	}
	type rec struct {
		hash, status string
		run          int64
	}
	by := map[string][]rec{}
	for rows.Next() {
		var d time.Time
		var status string
		var hash pgtype.Text
		var run int64
		if e = rows.Scan(&d, &status, &hash, &run); e != nil {
			rows.Close()
			return st, e
		}
		k := d.Format("2006-01-02")
		if _, ok := st.reconciliation[k]; !ok {
			st.reconciliation[k] = status
		}
		h := ""
		if hash.Valid {
			h = hash.String
		}
		seenRun := false
		for _, existing := range by[k] {
			if existing.run == run {
				seenRun = true
				break
			}
		}
		if !seenRun {
			by[k] = append(by[k], rec{h, status, run})
		}
	}
	rows.Close()
	var authMode, state, oauthState string
	var refreshError *string
	e = s.pool.QueryRow(ctx, `SELECT auth_mode,status,oauth_connection_state,oauth_last_refresh_error FROM ozon_connections WHERE seller_account_id=$1`, seller).Scan(&authMode, &state, &oauthState, &refreshError)
	if e == nil {
		if (authMode == "PRIVATE_OAUTH" && oauthState == "CONNECTED") || (authMode == "LEGACY" && state == "valid") {
			st.connectionAccess = "ACCESSIBLE"
		}
		if authMode == "PRIVATE_OAUTH" && oauthState == "CONNECTED" {
			st.canonicalOptionalAccess = "ACCESSIBLE"
		}
		if oauthState == "REFRESH_ERROR" || state == "invalid" {
			st.connectionAccess = "AUTH_FAILED"
			if authMode == "PRIVATE_OAUTH" {
				st.canonicalOptionalAccess = "AUTH_FAILED"
			}
		}
	} else if e != pgx.ErrNoRows {
		return st, e
	}
	accessRows, e := s.pool.Query(ctx, `SELECT DISTINCT ON(domain)domain,COALESCE(error_code,'') FROM collection_domain_runs WHERE seller_account_id=$1 AND started_at<=$2 ORDER BY domain,started_at DESC,id DESC`, seller, cutoff)
	if e != nil {
		return st, e
	}
	for accessRows.Next() {
		var domain, code string
		if e = accessRows.Scan(&domain, &code); e != nil {
			accessRows.Close()
			return st, e
		}
		access := st.connectionAccess
		if domain == "STOCK" || domain == "PRICE" || domain == "ADVERTISING" || domain == "FUNNEL" || domain == "FINANCE" || domain == "PROMOTIONS" || domain == "SUPPLY" {
			access = st.canonicalOptionalAccess
		}
		switch code {
		case "AUTH_FAILED", "OAUTH_REFRESH_FAILED", "OAUTH_TOKEN_INVALID":
			access = "AUTH_FAILED"
		case "PROVIDER_FORBIDDEN":
			access = "PROVIDER_FORBIDDEN"
		case "UNAVAILABLE_SCOPE":
			access = "UNAVAILABLE_SCOPE"
		case "UNAVAILABLE_SUBSCRIPTION":
			access = "UNAVAILABLE_SUBSCRIPTION"
		}
		st.currentDomainAccess[domain] = access
		st.currentDomainError[domain] = code
	}
	accessRows.Close()
	// Complete-through is the end of the first contiguous sequence of usable
	// account days, not merely the latest account row.
	st.salesCompleteThrough = nil
	if st.salesFrom != nil && st.salesTo != nil {
		for d := *st.salesFrom; !d.After(*st.salesTo); d = d.AddDate(0, 0, 1) {
			k := d.Format("2006-01-02")
			status, ok := st.reconciliation[k]
			if !st.salesDates[k] || !ok || (status != "PASS" && status != "EXPLAINED_GAP") {
				break
			}
			v := d
			st.salesCompleteThrough = &v
		}
	}
	if st.skuCount > 0 {
		e = s.pool.QueryRow(ctx, `WITH facts AS(SELECT DISTINCT ON(canonical_sku_id,business_date)canonical_sku_id,business_date,ordered_amount FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id IS NOT NULL AND observed_at<=$2 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC),classified AS(SELECT f.ordered_amount,EXISTS(SELECT 1 FROM sku_category_assignments a WHERE a.canonical_sku_id=f.canonical_sku_id AND a.mapping_status='MAPPED' AND a.seller_category_id IS NOT NULL AND a.observed_from<((f.business_date+1)::timestamp AT TIME ZONE 'Europe/Moscow')AND(a.observed_to IS NULL OR a.observed_to>(f.business_date::timestamp AT TIME ZONE 'Europe/Moscow'))AND a.observed_from<=$2)mapped FROM facts f)SELECT COALESCE(sum(ordered_amount),0)::text,COALESCE(sum(ordered_amount)FILTER(WHERE mapped),0)::text FROM classified`, seller, cutoff).Scan(&st.categoryExposureTotal, &st.categoryExposureMapped)
		if e != nil {
			return st, e
		}
	}
	if e = s.pool.QueryRow(ctx, `WITH current AS(SELECT cs.id FROM canonical_skus cs JOIN LATERAL(SELECT catalog_state FROM canonical_sku_versions v WHERE v.canonical_sku_id=cs.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true WHERE cs.seller_account_id=$1 AND v.catalog_state<>'UNOBSERVED')SELECT count(*) FROM current c LEFT JOIN LATERAL(SELECT product_type_resolution_status FROM sku_category_assignments a WHERE a.canonical_sku_id=c.id AND a.observed_from<=$2 AND(a.observed_to IS NULL OR a.observed_to>$2)ORDER BY a.observed_from DESC,a.id DESC LIMIT 1)a ON true WHERE COALESCE(a.product_type_resolution_status,'NOT_APPLICABLE') NOT IN('RESOLVED','NOT_APPLICABLE')`, seller, cutoff).Scan(&st.productTypeResolutionIssues); e != nil {
		return st, e
	}
	for k, v := range by {
		usable := func(x rec) bool { return x.hash != "" && (x.status == "PASS" || x.status == "EXPLAINED_GAP") }
		if len(v) >= 2 && usable(v[0]) && usable(v[1]) && v[0].run != v[1].run && v[0].hash == v[1].hash {
			st.matureDates[k] = true
		}
	}
	for _, status := range st.reconciliation {
		if status == "EXPLAINED_GAP" {
			st.explainedSalesDays++
		}
	}
	rows, e = s.pool.Query(ctx, `SELECT DISTINCT ON(domain)domain,status FROM collection_domain_runs WHERE seller_account_id=$1 AND started_at<=$2 ORDER BY domain,started_at DESC,id DESC`, seller, cutoff)
	if e != nil {
		return st, e
	}
	for rows.Next() {
		var d, status string
		if e = rows.Scan(&d, &status); e != nil {
			rows.Close()
			return st, e
		}
		st.currentDomainState[d] = status
	}
	rows.Close()
	e = s.pool.QueryRow(ctx, `SELECT count(*),max(v.observed_at) FROM sales_postings p LEFT JOIN sales_posting_versions v ON v.canonical_posting_id=p.id AND v.observed_at<=$2 WHERE p.seller_account_id=$1 AND p.first_observed_at<=$2`, seller, cutoff).Scan(&st.postingCount, &obs)
	if e != nil {
		return st, e
	}
	if obs.Valid {
		v := obs.Time
		st.latestPostingObserved = &v
	}
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(p.id)p.id,v.order_resolution_status,v.id version_id FROM sales_postings p JOIN sales_posting_versions v ON v.canonical_posting_id=p.id AND v.observed_at<=$2 WHERE p.seller_account_id=$1 AND p.first_observed_at<=$2 ORDER BY p.id,v.observed_at DESC,v.version_no DESC,v.id DESC)SELECT count(*)FILTER(WHERE order_resolution_status<>'RESOLVED'),count(*)FILTER(WHERE EXISTS(SELECT 1 FROM sales_posting_products pp WHERE pp.posting_version_id=latest.version_id AND pp.sku_resolution_status<>'RESOLVED'))FROM latest`, seller, cutoff).Scan(&st.postingOrderResolutionIssues, &st.postingSKUResolutionIssues)
	if e != nil {
		return st, e
	}
	e = s.pool.QueryRow(ctx, `SELECT count(*),max(e.observed_at) FROM sales_returns r LEFT JOIN sales_return_events e ON e.sales_return_id=r.id AND e.observed_at<=$2 WHERE r.seller_account_id=$1 AND r.first_observed_at<=$2`, seller, cutoff).Scan(&st.returnCount, &obs)
	if e != nil {
		return st, e
	}
	if obs.Valid {
		v := obs.Time
		st.latestReturnObserved = &v
	}
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(r.id)e.order_resolution_status,e.posting_resolution_status,e.sku_resolution_status FROM sales_returns r JOIN sales_return_events e ON e.sales_return_id=r.id AND e.observed_at<=$2 WHERE r.seller_account_id=$1 AND r.first_observed_at<=$2 ORDER BY r.id,e.observed_at DESC,e.version_no DESC,e.id DESC)SELECT count(*)FILTER(WHERE order_resolution_status<>'RESOLVED' OR posting_resolution_status<>'RESOLVED' OR sku_resolution_status<>'RESOLVED')FROM latest`, seller, cutoff).Scan(&st.returnResolutionIssues)
	if e != nil {
		return st, e
	}
	e = s.pool.QueryRow(ctx, `SELECT count(*) FROM sales_lifecycle_events WHERE seller_account_id=$1 AND observed_at<=$2 AND time_quality='UNKNOWN'`, seller, cutoff).Scan(&st.unknownEvents)
	if e != nil {
		return st, e
	}
	if st.salesFrom != nil && st.salesTo != nil {
		if e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT business_date FROM sales_account_daily WHERE seller_account_id=$1 AND observed_at<=$2)SELECT count(*) FROM generate_series($3::date,$4::date,'1 day'::interval)g(d)LEFT JOIN latest l ON l.business_date=g.d::date WHERE l.business_date IS NULL`, seller, cutoff, *st.salesFrom, *st.salesTo).Scan(&st.missingSalesDays); e != nil {
			return st, e
		}
	}
	if e = s.pool.QueryRow(ctx, `WITH combined AS(SELECT business_date,currency_code FROM sales_account_daily WHERE seller_account_id=$1 AND observed_at<=$2 UNION ALL SELECT business_date,currency_code FROM sales_demand_daily WHERE seller_account_id=$1 AND observed_at<=$2)SELECT count(*) FROM(SELECT business_date FROM combined GROUP BY business_date HAVING count(DISTINCT currency_code)>1)x`, seller, cutoff).Scan(&st.mixedCurrencyDays); e != nil {
		return st, e
	}
	if e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(p.id)v.id FROM sales_postings p JOIN sales_posting_versions v ON v.canonical_posting_id=p.id AND v.observed_at<=$2 WHERE p.seller_account_id=$1 ORDER BY p.id,v.observed_at DESC,v.version_no DESC,v.id DESC)SELECT count(*) FROM sales_posting_products pp WHERE pp.posting_version_id IN(SELECT id FROM latest)AND pp.source_sku IS NULL`, seller, cutoff).Scan(&st.missingPostingSKUs); e != nil {
		return st, e
	}
	var stockObserved, stockComplete, priceObserved, priceComplete, adObserved pgtype.Timestamptz
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT * FROM stock_snapshots WHERE seller_account_id=$1 AND snapshot_at<=$2 ORDER BY snapshot_at DESC,id DESC LIMIT 1),complete AS(SELECT snapshot_at FROM stock_snapshots WHERE seller_account_id=$1 AND snapshot_at<=$2 AND completeness_status='COMPLETE' ORDER BY snapshot_at DESC,id DESC LIMIT 1)SELECT count(*),max(snapshot_at),(SELECT snapshot_at FROM complete),COALESCE((SELECT position_count FROM latest LIMIT 1),0),COALESCE((SELECT unresolved_sku_count FROM latest LIMIT 1),0),COALESCE((SELECT batches_failed FROM latest LIMIT 1),0),COALESCE((SELECT count(*)FROM stock_snapshot_lines l JOIN inventory_pools p ON p.id=l.inventory_pool_id WHERE l.snapshot_id=(SELECT id FROM latest)AND p.independence_status='UNVERIFIED' AND COALESCE(l.absence_reason,'')=''),0),COALESCE((SELECT count(*)FROM stock_snapshot_lines l WHERE l.snapshot_id=(SELECT id FROM latest)AND l.quantity_semantic_status='PENDING_ACCEPTANCE' AND COALESCE(l.absence_reason,'')=''),0),COALESCE((SELECT missing_response_sku_count FROM latest LIMIT 1),0)FROM latest`, seller, cutoff).Scan(&st.stockSnapshotCount, &stockObserved, &stockComplete, &st.stockPositionCount, &st.stockUnresolved, &st.stockFailedBatches, &st.stockAmbiguousPools, &st.stockPendingSemantics, &st.stockMissingResponse)
	if e != nil {
		return st, e
	}
	if stockObserved.Valid {
		v := stockObserved.Time
		st.latestStockObserved = &v
	}
	if stockComplete.Valid {
		v := stockComplete.Time
		st.latestStockComplete = &v
	}
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(canonical_sku_id)* FROM price_snapshots WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY canonical_sku_id,observed_at DESC,id DESC),collection AS(SELECT * FROM price_collection_snapshots WHERE seller_account_id=$1 AND snapshot_at<=$2 ORDER BY snapshot_at DESC,id DESC LIMIT 1),complete AS(SELECT snapshot_at FROM price_collection_snapshots WHERE seller_account_id=$1 AND snapshot_at<=$2 AND completeness_status='COMPLETE' ORDER BY snapshot_at DESC,id DESC LIMIT 1)SELECT count(*),max(observed_at),(SELECT snapshot_at FROM complete),count(*)FILTER(WHERE sku_resolution_status='RESOLVED'),count(*)FILTER(WHERE sku_resolution_status<>'RESOLVED'),count(*)FILTER(WHERE current_price_state='INVALID'),count(*)FILTER(WHERE semantic_mapping_status='PENDING_ACCEPTANCE'),CASE WHEN count(DISTINCT currency_code)>1 THEN 1 ELSE 0 END FROM latest`, seller, cutoff).Scan(&st.priceSnapshotCount, &priceObserved, &priceComplete, &st.priceResolved, &st.priceUnresolved, &st.priceInvalid, &st.pricePendingSemantics, &st.priceMixedCurrencies)
	if e != nil {
		return st, e
	}
	if priceObserved.Valid {
		v := priceObserved.Time
		st.latestPriceObserved = &v
	}
	if priceComplete.Valid {
		v := priceComplete.Time
		st.latestPriceComplete = &v
	}
	var adFrom, adTo, adMature, adTrafficComplete pgtype.Date
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(advertising_campaign_id,business_date,attribution_kind)* FROM advertising_daily_facts WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC),recon AS(SELECT DISTINCT ON(business_date)business_date,reconciliation_status FROM advertising_spend_reconciliations WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY business_date,observed_at DESC,id DESC),day_ok AS(SELECT business_date,bool_and(traffic_state='USABLE')AS ok FROM latest WHERE attribution_kind='PROVIDER_DEFAULT' GROUP BY business_date),ordered AS(SELECT business_date,ok,SUM(CASE WHEN NOT ok THEN 1 ELSE 0 END)OVER(ORDER BY business_date ROWS UNBOUNDED PRECEDING)AS holes FROM day_ok),covered AS(SELECT business_date FROM ordered WHERE ok AND holes=0)SELECT (SELECT count(*)FROM advertising_campaigns WHERE seller_account_id=$1 AND first_observed_at<=$2),count(*),max(observed_at),min(business_date),max(business_date),max(business_date)FILTER(WHERE attribution_state='MATURE'),count(*)FILTER(WHERE attribution_state IN('PROVISIONAL','PENDING_ACCEPTANCE')),COALESCE((SELECT count(*)FROM advertising_campaigns c JOIN LATERAL(SELECT campaign_family FROM advertising_campaign_versions v WHERE v.advertising_campaign_id=c.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true WHERE c.seller_account_id=$1 AND v.campaign_family='UNKNOWN'),0),COALESCE((SELECT count(*)FROM advertising_target_intervals WHERE seller_account_id=$1 AND observed_from<=$2 AND(observed_to IS NULL OR observed_to>$2)AND sku_resolution_status NOT IN('RESOLVED','NOT_APPLICABLE')),0),COALESCE((SELECT count(*)FROM(SELECT business_date FROM latest GROUP BY business_date HAVING count(DISTINCT currency_code)>1)x),0),COALESCE((SELECT count(*)FROM recon WHERE reconciliation_status IN('DIFFERENT','NOT_COMPARABLE')),0),COALESCE((SELECT count(*)FROM advertising_campaigns c JOIN LATERAL(SELECT target_scope_type FROM advertising_campaign_versions v WHERE v.advertising_campaign_id=c.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true WHERE c.seller_account_id=$1 AND v.target_scope_type='UNKNOWN'),0),(SELECT max(business_date)FROM covered)FROM latest`, seller, cutoff).Scan(&st.adCampaignCount, &st.adDailyCount, &adObserved, &adFrom, &adTo, &adMature, &st.adPendingAttribution, &st.adUnknownFamilies, &st.adUnresolvedTargets, &st.adMixedCurrencies, &st.adReconciliationIssues, &st.adUnknownTargetScopes, &adTrafficComplete)
	if e != nil {
		return st, e
	}
	if adObserved.Valid {
		v := adObserved.Time
		st.latestAdvertisingObserved = &v
	}
	if adFrom.Valid {
		v := adFrom.Time
		st.adHistoryFrom = &v
	}
	if adTo.Valid {
		v := adTo.Time
		st.adHistoryTo = &v
	}
	if adMature.Valid {
		v := adMature.Time
		st.adMatureThrough = &v
	}
	if adTrafficComplete.Valid {
		v := adTrafficComplete.Time
		st.adTrafficCompleteThrough = &v
	}
	var funnelObserved pgtype.Timestamptz
	var funnelAddFrom, funnelAddTo, funnelDirectFrom, funnelDirectTo pgtype.Date
	e = s.pool.QueryRow(ctx, `WITH latest AS(SELECT DISTINCT ON(scope_kind,COALESCE(source_sku,0),metric_key,period_start,period_end)* FROM funnel_observations WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY scope_kind,COALESCE(source_sku,0),metric_key,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)SELECT count(*)FILTER(WHERE temporal_semantics='ADDITIVE_FLOW'),count(*)FILTER(WHERE temporal_semantics IN('DIRECT_PERIOD','PROVIDER_RATIO')),max(observed_at),min(period_start)FILTER(WHERE temporal_semantics='ADDITIVE_FLOW'),max(period_end)FILTER(WHERE temporal_semantics='ADDITIVE_FLOW'),min(period_start)FILTER(WHERE temporal_semantics IN('DIRECT_PERIOD','PROVIDER_RATIO')),max(period_end)FILTER(WHERE temporal_semantics IN('DIRECT_PERIOD','PROVIDER_RATIO'))FROM latest`, seller, cutoff).Scan(&st.funnelAdditiveCount, &st.funnelDirectCount, &funnelObserved, &funnelAddFrom, &funnelAddTo, &funnelDirectFrom, &funnelDirectTo)
	if e != nil {
		return st, e
	}
	if funnelObserved.Valid {
		v := funnelObserved.Time
		st.funnelLatestObserved = &v
	}
	if funnelAddFrom.Valid {
		v := funnelAddFrom.Time
		st.funnelAdditiveFrom = &v
	}
	if funnelAddTo.Valid {
		v := funnelAddTo.Time
		st.funnelAdditiveTo = &v
	}
	if funnelDirectFrom.Valid {
		v := funnelDirectFrom.Time
		st.funnelDirectFrom = &v
	}
	if funnelDirectTo.Valid {
		v := funnelDirectTo.Time
		st.funnelDirectTo = &v
	}
	var financeObserved, balanceObserved pgtype.Timestamptz
	var financeFrom, financeTo, financeComplete pgtype.Date
	e = s.pool.QueryRow(ctx, `WITH latest AS(
		SELECT DISTINCT ON(business_date)* FROM finance_accrual_day_revisions
		WHERE seller_account_id=$1 AND observed_at<=$2
		ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)
		SELECT count(*),max(observed_at),min(business_date),max(business_date),
		       max(business_date)FILTER(WHERE completeness_status='COMPLETE'),
		       count(*)FILTER(WHERE completeness_status='PARTIAL'),
		       COALESCE((SELECT count(DISTINCT r.provider_type_id)FROM finance_accrual_records r JOIN latest l ON l.id=r.day_revision_id LEFT JOIN finance_provider_type_versions v ON v.finance_provider_type_id=r.finance_provider_type_id AND v.valid_from<=$2 AND(v.valid_to IS NULL OR v.valid_to>$2)WHERE COALESCE(v.mapping_status,'UNMAPPED')='UNMAPPED'),0),
		       COALESCE((SELECT count(*)FROM(SELECT l.business_date FROM latest l JOIN finance_accrual_records r ON r.day_revision_id=l.id GROUP BY l.business_date HAVING count(DISTINCT r.currency_code)>1)x),0)
		FROM latest`, seller, cutoff).Scan(&st.financeDayCount, &financeObserved, &financeFrom, &financeTo, &financeComplete, &st.financePartialDays, &st.financeUnmappedTypes, &st.financeMixedCurrencies)
	if e != nil {
		return st, e
	}
	if financeObserved.Valid {
		v := financeObserved.Time
		st.financeLatestObserved = &v
	}
	if financeFrom.Valid {
		v := financeFrom.Time
		st.financeHistoryFrom = &v
	}
	if financeTo.Valid {
		v := financeTo.Time
		st.financeHistoryTo = &v
	}
	if financeComplete.Valid {
		v := financeComplete.Time
		st.financeCompleteThrough = &v
	}
	e = s.pool.QueryRow(ctx, `SELECT count(*),max(observed_at)FROM finance_balance_snapshots WHERE seller_account_id=$1 AND observed_at<=$2`, seller, cutoff).Scan(&st.financeBalanceCount, &balanceObserved)
	if e != nil {
		return st, e
	}
	if balanceObserved.Valid {
		v := balanceObserved.Time
		st.financeBalanceLatestObserved = &v
	}
	var searchSummaryObserved, searchDetailObserved, searchMarketObserved pgtype.Timestamptz
	var searchSummaryFrom, searchSummaryTo, searchDetailFrom, searchDetailTo, searchMarketFrom, searchMarketTo pgtype.Date
	e = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM search_sku_summaries WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM search_sku_summaries WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT min(period_start) FROM search_sku_summaries WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(period_end) FROM search_sku_summaries WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM search_query_details WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM search_query_details WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT min(period_start) FROM search_query_details WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(period_end) FROM search_query_details WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM search_market_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM search_market_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT min(period_start) FROM search_market_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(period_end) FROM search_market_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM search_query_details WHERE seller_account_id=$1 AND observed_at<=$2 AND sku_resolution_status='UNRESOLVED')`,
		seller, cutoff).Scan(&st.searchSummaryCount, &searchSummaryObserved, &searchSummaryFrom, &searchSummaryTo, &st.searchDetailCount, &searchDetailObserved, &searchDetailFrom, &searchDetailTo, &st.searchMarketCount, &searchMarketObserved, &searchMarketFrom, &searchMarketTo, &st.searchUnresolvedDetails)
	if e != nil {
		return st, e
	}
	if searchSummaryObserved.Valid {
		v := searchSummaryObserved.Time
		st.searchSummaryObserved = &v
	}
	if searchDetailObserved.Valid {
		v := searchDetailObserved.Time
		st.searchDetailObserved = &v
	}
	if searchMarketObserved.Valid {
		v := searchMarketObserved.Time
		st.searchMarketObserved = &v
	}
	if searchSummaryFrom.Valid {
		v := searchSummaryFrom.Time
		st.searchSummaryFrom = &v
	}
	if searchSummaryTo.Valid {
		v := searchSummaryTo.Time
		st.searchSummaryTo = &v
	}
	if searchDetailFrom.Valid {
		v := searchDetailFrom.Time
		st.searchDetailFrom = &v
	}
	if searchDetailTo.Valid {
		v := searchDetailTo.Time
		st.searchDetailTo = &v
	}
	if searchMarketFrom.Valid {
		v := searchMarketFrom.Time
		st.searchMarketFrom = &v
	}
	if searchMarketTo.Valid {
		v := searchMarketTo.Time
		st.searchMarketTo = &v
	}
	var healthSnapshotObserved, healthHistoryObserved, healthIndexObserved, healthErrorsObserved pgtype.Timestamptz
	var healthFrom, healthTo pgtype.Date
	e = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM account_health_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM account_health_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM account_rating_period_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM account_rating_period_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT min(period_from) FROM account_rating_period_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(period_to) FROM account_rating_period_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM account_rating_threshold_observations WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM account_penalty_event_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM fbs_error_index_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM fbs_error_index_facts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM fbs_posting_error_events WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM fbs_posting_error_events WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM account_health_metric_versions v JOIN account_health_metrics m ON m.id=v.account_health_metric_id WHERE v.seller_account_id=$1 AND v.observed_at<=$2 AND v.mapping_status='UNMAPPED' AND NOT EXISTS(SELECT 1 FROM account_health_metric_versions newer WHERE newer.account_health_metric_id=v.account_health_metric_id AND newer.observed_at<=$2 AND(newer.observed_at,newer.id)>(v.observed_at,v.id))),
		(SELECT count(*) FROM fbs_posting_error_events e WHERE seller_account_id=$1 AND observed_at<=$2 AND posting_resolution_status<>'RESOLVED' AND NOT EXISTS(SELECT 1 FROM fbs_posting_error_events newer WHERE newer.seller_account_id=e.seller_account_id AND newer.provider_event_key=e.provider_event_key AND newer.observed_at<=$2 AND(newer.observed_at,newer.revision_no,newer.id)>(e.observed_at,e.revision_no,e.id)))`,
		seller, cutoff).Scan(&st.healthSnapshotCount, &healthSnapshotObserved, &st.healthHistoryCount, &healthHistoryObserved, &healthFrom, &healthTo, &st.healthThresholdCount, &st.healthPenaltyCount, &st.healthFBSIndexCount, &healthIndexObserved, &st.healthFBSErrorCount, &healthErrorsObserved, &st.healthUnmappedMetrics, &st.healthUnresolvedPostings)
	if e != nil {
		return st, e
	}
	if healthSnapshotObserved.Valid {
		v := healthSnapshotObserved.Time
		st.healthSnapshotObserved = &v
	}
	if healthHistoryObserved.Valid {
		v := healthHistoryObserved.Time
		st.healthHistoryObserved = &v
	}
	if healthIndexObserved.Valid {
		v := healthIndexObserved.Time
		st.healthFBSIndexObserved = &v
	}
	if healthErrorsObserved.Valid {
		v := healthErrorsObserved.Time
		st.healthFBSErrorObserved = &v
	}
	if healthFrom.Valid {
		v := healthFrom.Time
		st.healthHistoryFrom = &v
	}
	if healthTo.Valid {
		v := healthTo.Time
		st.healthHistoryTo = &v
	}
	var supplyStateObserved, supplyBundleObserved, supplyBundleComplete, supplyAcceptanceObserved, supplyAcceptanceComplete pgtype.Timestamptz
	e = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM supply_state_observations WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM supply_state_observations WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT count(*) FROM supply_state_observations WHERE seller_account_id=$1 AND observed_at<=$2 AND state_mapping_status='UNKNOWN'),
		(SELECT count(*) FROM supply_bundle_versions WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM supply_bundle_versions WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM supply_bundle_versions WHERE seller_account_id=$1 AND observed_at<=$2 AND completeness_status='COMPLETE'),
		(SELECT count(*) FROM supply_bundle_versions WHERE seller_account_id=$1 AND observed_at<=$2 AND completeness_status='PARTIAL'),
		(SELECT count(*) FROM supply_acceptance_acts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM supply_acceptance_acts WHERE seller_account_id=$1 AND observed_at<=$2),
		(SELECT max(observed_at) FROM supply_acceptance_acts WHERE seller_account_id=$1 AND observed_at<=$2 AND completeness_status='COMPLETE'),
		(SELECT count(*) FROM supply_acceptance_items i JOIN supply_acceptance_acts a ON a.id=i.supply_acceptance_act_id WHERE a.seller_account_id=$1 AND a.observed_at<=$2),
		(SELECT count(*) FROM supply_acceptance_acts WHERE seller_account_id=$1 AND observed_at<=$2 AND completeness_status='PARTIAL')`,
		seller, cutoff).Scan(&st.supplyStateCount, &supplyStateObserved, &st.supplyUnknownStates, &st.supplyBundleCount, &supplyBundleObserved, &supplyBundleComplete, &st.supplyPartialBundles, &st.supplyAcceptanceCount, &supplyAcceptanceObserved, &supplyAcceptanceComplete, &st.supplyAcceptanceItemCount, &st.supplyAcceptancePartial)
	if e != nil {
		return st, e
	}
	if supplyStateObserved.Valid {
		v := supplyStateObserved.Time
		st.supplyStateObserved = &v
	}
	if supplyBundleObserved.Valid {
		v := supplyBundleObserved.Time
		st.supplyBundleObserved = &v
	}
	if supplyBundleComplete.Valid {
		v := supplyBundleComplete.Time
		st.supplyBundleCompleteObserved = &v
	}
	if supplyAcceptanceObserved.Valid {
		v := supplyAcceptanceObserved.Time
		st.supplyAcceptanceObserved = &v
	}
	if supplyAcceptanceComplete.Valid {
		v := supplyAcceptanceComplete.Time
		st.supplyAcceptanceCompleteObserved = &v
	}
	return st, e
}
