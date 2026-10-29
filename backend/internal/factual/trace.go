package factual

import (
	"context"
	"errors"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) TraceFact(ctx context.Context, c factualapi.FactualReadContext, ref string) (factualapi.FactTrace, error) {
	var out factualapi.FactTrace
	if e := s.validate(c); e != nil {
		return out, e
	}
	in, e := s.signer.ParseFactRef(ref, c)
	if e != nil {
		return out, e
	}
	if e = s.verifyFactResult(ctx, c, in); e != nil {
		return out, e
	}
	out.Kind, out.ContextHash, out.ResultHash, out.PolicyVersion = in.Kind, c.ContextHash(), in.ResultHash, c.PolicyVersions().API
	if in.Kind == "ACCOUNT" {
		if in.EntityID != c.SellerAccountID() {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "account reference is outside context")
		}
		out.Sources = append(out.Sources, factualapi.TraceSource{CanonicalFactID: in.EntityID, EndpointKey: "CANONICAL_ACCOUNT_IDENTITY", SemanticRole: "CANONICAL_IDENTITY"})
		out.Limitations = append(out.Limitations, factualapi.Limitation{Code: "IDENTITY_ONLY", Message: "account trace contains only the immutable seller identity"})
		return out, nil
	}
	var query string
	var args []any
	switch in.Kind {
	case "SKU":
		query = `SELECT v.id,0,0,'CANONICAL_CATALOG',v.observed_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM canonical_skus sku JOIN LATERAL(SELECT * FROM canonical_sku_versions x WHERE x.canonical_sku_id=sku.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE sku.seller_account_id=$1 AND sku.id=$2 AND sku.first_observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "CATEGORY":
		query = `SELECT v.id,0,0,'CANONICAL_TAXONOMY',v.observed_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM seller_categories sc JOIN provider_categories pc ON pc.id=sc.provider_category_id JOIN LATERAL(SELECT * FROM provider_category_versions x WHERE x.provider_category_id=pc.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true JOIN taxonomy_sync_runs ts ON ts.id=v.taxonomy_sync_run_id JOIN collection_domain_runs dr ON dr.id=ts.collection_domain_run_id WHERE sc.seller_account_id=$1 AND sc.id=$2 AND sc.first_observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ORDER":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM sales_orders o JOIN sales_posting_versions v ON v.canonical_order_id=o.id AND v.observed_at<=$3 JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE o.seller_account_id=$1 AND o.id=$2 AND o.first_observed_at<=$3 ORDER BY v.observed_at,v.id LIMIT 1`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ACCOUNT_DEMAND":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM(SELECT DISTINCT ON(business_date)* FROM sales_account_daily WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
	case "SKU_DEMAND", "SKU_DEMAND_DAY", "CATEGORY_DEMAND":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		if in.Kind == "CATEGORY_DEMAND" {
			query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM(SELECT DISTINCT ON(canonical_sku_id,business_date)* FROM sales_demand_daily WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY canonical_sku_id,business_date,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date,f.canonical_sku_id`
			args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
			out.Limitations = append(out.Limitations, factualapi.Limitation{Code: "CATEGORY_ASSIGNMENT_TRACE", Message: "category assignment is resolved by historical interval intersection; ambiguous days remain UNMAPPED"})
		} else {
			query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM(SELECT DISTINCT ON(business_date)* FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date`
			args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
		}
	case "POSTING":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM sales_postings p JOIN LATERAL(SELECT * FROM sales_posting_versions x WHERE x.canonical_posting_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE p.seller_account_id=$1 AND p.id=$2`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "RETURN":
		query = `SELECT e.id,e.version_no,e.source_payload_id,sp.endpoint_key,sp.received_at,e.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM sales_returns r JOIN LATERAL(SELECT * FROM sales_return_events x WHERE x.sales_return_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)e ON true JOIN source_payloads sp ON sp.id=e.source_payload_id JOIN collection_domain_runs dr ON dr.id=e.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.id=$2`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "LIFECYCLE_EVENT":
		query = `SELECT e.id,0,e.source_payload_id,sp.endpoint_key,sp.received_at,e.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'') FROM sales_lifecycle_events e JOIN source_payloads sp ON sp.id=e.source_payload_id JOIN collection_domain_runs dr ON dr.id=e.collection_domain_run_id WHERE e.seller_account_id=$1 AND e.id=$2 AND e.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SKU_STOCK":
		if in.AuxiliaryID <= 0 {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "stock snapshot id missing")
		}
		query = `SELECT l.id,0,l.source_payload_id,sp.endpoint_key,sp.received_at,l.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM stock_snapshot_lines l JOIN stock_snapshots s ON s.id=l.snapshot_id JOIN source_payloads sp ON sp.id=l.source_payload_id JOIN collection_domain_runs dr ON dr.id=s.collection_domain_run_id WHERE l.seller_account_id=$1 AND l.canonical_sku_id=$2 AND s.id=$3 AND s.snapshot_at<=$4 ORDER BY l.id`
		args = []any{c.SellerAccountID(), in.EntityID, in.AuxiliaryID, c.CutoffAt()}
	case "STOCK_COVERAGE":
		if in.AuxiliaryID <= 0 {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "stock snapshot id missing")
		}
		query = `SELECT l.id,0,l.source_payload_id,sp.endpoint_key,sp.received_at,l.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM stock_snapshot_lines l JOIN stock_snapshots s ON s.id=l.snapshot_id JOIN source_payloads sp ON sp.id=l.source_payload_id JOIN collection_domain_runs dr ON dr.id=s.collection_domain_run_id WHERE l.seller_account_id=$1 AND l.canonical_sku_id=$2 AND s.id=$3 AND s.snapshot_at<=$4 ORDER BY l.id`
		args = []any{c.SellerAccountID(), in.EntityID, in.AuxiliaryID, c.CutoffAt()}
	case "STOCK_SNAPSHOT":
		query = `SELECT l.id,0,l.source_payload_id,sp.endpoint_key,sp.received_at,l.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM stock_snapshot_lines l JOIN stock_snapshots s ON s.id=l.snapshot_id JOIN source_payloads sp ON sp.id=l.source_payload_id JOIN collection_domain_runs dr ON dr.id=s.collection_domain_run_id WHERE s.seller_account_id=$1 AND s.id=$2 AND s.snapshot_at<=$3 ORDER BY l.id`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "PRICE_OBSERVATION":
		query = `SELECT p.id,0,p.source_payload_id,sp.endpoint_key,sp.received_at,p.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM price_snapshots p JOIN source_payloads sp ON sp.id=p.source_payload_id JOIN collection_domain_runs dr ON dr.id=p.collection_domain_run_id WHERE p.seller_account_id=$1 AND p.id=$2 AND p.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "CAMPAIGN":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM advertising_campaigns c JOIN LATERAL(SELECT * FROM advertising_campaign_versions x WHERE x.advertising_campaign_id=c.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE c.seller_account_id=$1 AND c.id=$2`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "AD_DAILY", "CAMPAIGN_DRR":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(business_date,attribution_kind)* FROM advertising_daily_facts WHERE seller_account_id=$1 AND advertising_campaign_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date,f.attribution_kind`
		args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
	case "AD_ACCOUNT", "AD_SPEND_SHARE":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(advertising_campaign_id,business_date)* FROM advertising_daily_facts WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND attribution_kind='PROVIDER_DEFAULT'AND observed_at<=$4 ORDER BY advertising_campaign_id,business_date,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date,f.advertising_campaign_id`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
	case "SKU_ADVERTISING":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(advertising_campaign_id,business_date,attribution_kind)* FROM advertising_product_daily_facts WHERE seller_account_id=$1 AND promoted_canonical_sku_id=$2 AND business_date BETWEEN $3 AND $4 AND observed_at<=$5 ORDER BY advertising_campaign_id,business_date,attribution_kind,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date`
		args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
	case "CPO_ATTRIBUTION":
		query = `SELECT f.id,0,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM advertising_cpo_facts f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ACCOUNT_HEALTH_SUMMARY":
		query = `SELECT r.id,0,r.source_payload_id,sp.endpoint_key,sp.received_at,r.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM account_health_snapshot_runs r JOIN source_payloads sp ON sp.id=r.source_payload_id JOIN collection_domain_runs dr ON dr.id=r.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.id=$2 AND r.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ACCOUNT_RATING":
		query = `SELECT s.id,v.version_no,s.source_payload_id,sp.endpoint_key,sp.received_at,r.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM account_health_snapshot_runs r JOIN account_rating_snapshots s ON s.snapshot_run_id=r.id JOIN account_health_metric_versions v ON v.account_health_metric_id=s.account_health_metric_id JOIN source_payloads sp ON sp.id=s.source_payload_id JOIN collection_domain_runs dr ON dr.id=r.collection_domain_run_id WHERE r.seller_account_id=$1 AND s.account_health_metric_id=$2 AND r.observed_at<=$3 AND v.observed_at<=$3 ORDER BY r.observed_at DESC,v.observed_at DESC LIMIT 1`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "RATING_PERIOD":
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM account_rating_period_facts f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "RATING_THRESHOLD":
		query = `SELECT f.id,f.version_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM account_rating_threshold_observations f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ACCOUNT_PENALTY_EVENT":
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM account_penalty_event_facts f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "FBS_ERROR_INDEX":
		query = `SELECT f.id,0,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM fbs_error_index_facts f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "FBS_RATING_POSTING_ERROR":
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM fbs_posting_error_events f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "ACCOUNT_FUNNEL":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(metric_key,period_start,period_end)* FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='ACCOUNT' AND((period_end=period_start AND period_start BETWEEN $2 AND $3)OR(period_start=$2 AND period_end=$3))AND observed_at<=$4 ORDER BY metric_key,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.period_start,f.metric_key`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
	case "SKU_FUNNEL":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(metric_key,period_start,period_end)* FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND canonical_sku_id=$2 AND((period_end=period_start AND period_start BETWEEN $3 AND $4)OR(period_start=$3 AND period_end=$4))AND observed_at<=$5 ORDER BY metric_key,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.period_start,f.metric_key`
		args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
	case "CATEGORY_FUNNEL":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(canonical_sku_id,metric_key,period_start,period_end)* FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='SKU' AND((period_end=period_start AND period_start BETWEEN $2 AND $3)OR(period_start=$2 AND period_end=$3))AND observed_at<=$4 ORDER BY canonical_sku_id,metric_key,period_start,period_end,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.period_start,f.canonical_sku_id,f.metric_key`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
		out.Limitations = append(out.Limitations, factualapi.Limitation{Code: "CATEGORY_ASSIGNMENT_TRACE", Message: "category funnel rolls up SKU funnel observations by historical interval intersection; ambiguous days remain UNMAPPED"})
	case "FINANCE_DAY":
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM finance_accrual_day_revisions f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "FINANCE_ACCOUNT":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(business_date)* FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date BETWEEN $2 AND $3 AND observed_at<=$4 ORDER BY business_date,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.business_date`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
	case "FINANCE_ACCRUAL":
		query = `SELECT r.id,d.revision_no,d.source_payload_id,sp.endpoint_key,sp.received_at,d.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM finance_accrual_records r JOIN finance_accrual_day_revisions d ON d.id=r.day_revision_id AND d.seller_account_id=r.seller_account_id JOIN source_payloads sp ON sp.id=d.source_payload_id JOIN collection_domain_runs dr ON dr.id=d.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.id=$2 AND d.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "FINANCE_BALANCE":
		query = `SELECT f.id,0,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM finance_balance_snapshots f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.observed_at<=$2 ORDER BY f.observed_at DESC,f.id DESC LIMIT 1`
		args = []any{c.SellerAccountID(), c.CutoffAt()}
	case "SKU_SEARCH_SUMMARY":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT * FROM search_sku_summaries WHERE seller_account_id=$1 AND(canonical_sku_id=$2 OR(canonical_sku_id IS NULL AND source_sku=$2))AND period_start=$3 AND period_end=$4 AND observed_at<=$5 ORDER BY observed_at DESC,revision_no DESC,id DESC LIMIT 1)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id`
		args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
	case "SEARCH_DETAIL":
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM search_query_details f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.id=$2 AND f.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SEARCH_QUERY":
		query = `SELECT COALESCE(v.id,q.id),COALESCE(v.version_no,1),COALESCE(v.source_payload_id,0),COALESCE(sp.endpoint_key,'CANONICAL_SEARCH_QUERY'),COALESCE(sp.received_at,COALESCE(v.observed_at,q.created_at)),COALESCE(v.observed_at,q.created_at),COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM search_queries q LEFT JOIN LATERAL(SELECT * FROM search_query_versions x WHERE x.search_query_id=q.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN source_payloads sp ON sp.id=v.source_payload_id LEFT JOIN collection_domain_runs dr ON dr.id=COALESCE(v.collection_domain_run_id,sp.collection_domain_run_id) WHERE q.id=$1`
		args = []any{in.EntityID, c.CutoffAt()}
	case "CATEGORY_SEARCH":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,f.revision_no,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT DISTINCT ON(source_sku)* FROM search_sku_summaries WHERE seller_account_id=$1 AND period_start=$2 AND period_end=$3 AND observed_at<=$4 ORDER BY source_sku,observed_at DESC,revision_no DESC,id DESC)f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id ORDER BY f.source_sku`
		args = []any{c.SellerAccountID(), in.Period.From, in.Period.To, c.CutoffAt()}
		out.Limitations = append(out.Limitations, factualapi.Limitation{Code: "CATEGORY_ASSIGNMENT_TRACE", Message: "category search rolls up SKU search summaries by historical interval intersection; ambiguous assignments remain UNMAPPED"})
	case "MARKET_SEARCH":
		if in.Period == nil {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		query = `SELECT f.id,0,f.source_payload_id,sp.endpoint_key,sp.received_at,f.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM search_market_facts f JOIN source_payloads sp ON sp.id=f.source_payload_id JOIN collection_domain_runs dr ON dr.id=f.collection_domain_run_id WHERE f.seller_account_id=$1 AND f.search_query_id=$2 AND f.period_start=$3 AND f.period_end=$4 AND f.observed_at<=$5 ORDER BY f.observed_at DESC,f.id DESC`
		args = []any{c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()}
	case "PROMOTION":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM promotions p JOIN LATERAL(SELECT * FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE p.seller_account_id=$1 AND p.id=$2 AND p.first_observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SKU_PROMOTION":
		if in.AuxiliaryID <= 0 {
			return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "promotion membership id missing")
		}
		query = `SELECT m.id,0,m.source_payload_id,sp.endpoint_key,sp.received_at,m.known_from,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM promotion_sku_memberships m JOIN source_payloads sp ON sp.id=m.source_payload_id JOIN collection_domain_runs dr ON dr.id=m.collection_domain_run_id WHERE m.seller_account_id=$1 AND m.canonical_sku_id=$2 AND m.id=$3 AND m.known_from<=$4`
		args = []any{c.SellerAccountID(), in.EntityID, in.AuxiliaryID, c.CutoffAt()}
	case "PROMOTION_ELIGIBILITY":
		query = `SELECT r.id,0,COALESCE(r.source_payload_id,0),COALESCE(sp.endpoint_key,''),COALESCE(sp.received_at,r.observed_at),r.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM promotion_eligibility_snapshot_runs r LEFT JOIN source_payloads sp ON sp.id=r.source_payload_id JOIN collection_domain_runs dr ON dr.id=r.collection_domain_run_id WHERE r.seller_account_id=$1 AND r.promotion_id=$2 AND r.observed_at<=$3 ORDER BY r.observed_at DESC,r.id DESC LIMIT 1`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "PROMOTION_MEMBERSHIP_INTERVAL":
		query = `SELECT m.id,0,m.source_payload_id,sp.endpoint_key,sp.received_at,m.known_from,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM promotion_sku_memberships m JOIN source_payloads sp ON sp.id=m.source_payload_id JOIN collection_domain_runs dr ON dr.id=m.collection_domain_run_id WHERE m.seller_account_id=$1 AND m.id=$2 AND m.known_from<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SUPPLY_ORDER":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM supply_orders o JOIN LATERAL(SELECT * FROM supply_order_versions x WHERE x.supply_order_id=o.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE o.seller_account_id=$1 AND o.id=$2 AND o.first_observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SUPPLY":
		query = `SELECT v.id,v.version_no,v.source_payload_id,sp.endpoint_key,sp.received_at,v.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true JOIN source_payloads sp ON sp.id=v.source_payload_id JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE s.seller_account_id=$1 AND s.id=$2 AND s.first_observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SUPPLY_STATE_OBSERVATION":
		query = `SELECT o.id,0,o.source_payload_id,sp.endpoint_key,sp.received_at,o.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM supply_state_observations o JOIN source_payloads sp ON sp.id=o.source_payload_id JOIN collection_domain_runs dr ON dr.id=o.collection_domain_run_id WHERE o.seller_account_id=$1 AND o.id=$2 AND o.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SUPPLY_ACCEPTANCE_ACT":
		query = `SELECT a.id,0,a.source_payload_id,sp.endpoint_key,sp.received_at,a.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM supply_acceptance_acts a JOIN source_payloads sp ON sp.id=a.source_payload_id JOIN collection_domain_runs dr ON dr.id=a.collection_domain_run_id WHERE a.seller_account_id=$1 AND a.id=$2 AND a.observed_at<=$3`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	case "SKU_INBOUND":
		query = `SELECT i.id,b.version_no,i.source_payload_id,sp.endpoint_key,sp.received_at,b.observed_at,COALESCE(dr.source_contract_key,''),COALESCE(dr.source_contract_version,'')FROM(SELECT s.id,v.canonical_state FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE s.seller_account_id=$1 AND s.first_observed_at<=$3)current_supply JOIN LATERAL(SELECT * FROM supply_bundle_versions b WHERE b.supply_id=current_supply.id AND b.observed_at<=$3 AND current_supply.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')ORDER BY b.observed_at DESC,b.version_no DESC,b.id DESC LIMIT 1)b ON true JOIN supply_planned_items i ON i.supply_bundle_version_id=b.id AND i.canonical_sku_id=$2 JOIN source_payloads sp ON sp.id=i.source_payload_id JOIN collection_domain_runs dr ON dr.id=b.collection_domain_run_id ORDER BY i.id`
		args = []any{c.SellerAccountID(), in.EntityID, c.CutoffAt()}
	default:
		return out, factualapi.NewError(factualapi.CodeFactRefInvalid, "trace kind is not supported")
	}
	rows, e := s.pool.Query(ctx, query, args...)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.TraceSource
		var received, observed time.Time
		var contractKey, contractVersion string
		if e = rows.Scan(&x.CanonicalFactID, &x.RevisionNo, &x.SourcePayloadID, &x.EndpointKey, &received, &observed, &contractKey, &contractVersion); e != nil {
			return out, dbError(e)
		}
		x.SourceReceivedAt = received.UTC().Format(time.RFC3339Nano)
		x.CanonicalObservedAt = observed.UTC().Format(time.RFC3339Nano)
		actualHash, hashErr := s.traceContractHash(ctx, in.Kind, x.CanonicalFactID, x.SourcePayloadID)
		if hashErr != nil {
			return out, dbError(hashErr)
		}
		if actualHash != "" {
			x.ContractHash = actualHash
		} else {
			out.Limitations = append(out.Limitations, factualapi.Limitation{Code: "SOURCE_CONTRACT_HASH_UNAVAILABLE", Message: "accepted source revision did not persist a semantic contract hash"})
		}
		x.SemanticRole = "CANONICAL_FACT_SOURCE"
		out.Sources = append(out.Sources, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if in.Kind == "POSTING" {
		relations, qe := s.pool.Query(ctx, `SELECT rv.id,rv.revision_no,rv.source_payload_id,sp.endpoint_key,sp.received_at,rv.observed_at FROM sales_posting_relations r JOIN LATERAL(SELECT * FROM sales_posting_relation_versions x WHERE x.posting_relation_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.revision_no DESC,x.id DESC LIMIT 1)rv ON true JOIN source_payloads sp ON sp.id=rv.source_payload_id WHERE r.seller_account_id=$1 AND r.child_posting_id=$2 ORDER BY r.id`, c.SellerAccountID(), in.EntityID, c.CutoffAt())
		if qe != nil {
			return out, dbError(qe)
		}
		for relations.Next() {
			var x factualapi.TraceSource
			var received, observed time.Time
			if qe = relations.Scan(&x.CanonicalFactID, &x.RevisionNo, &x.SourcePayloadID, &x.EndpointKey, &received, &observed); qe != nil {
				relations.Close()
				return out, dbError(qe)
			}
			x.SourceReceivedAt = received.UTC().Format(time.RFC3339Nano)
			x.CanonicalObservedAt = observed.UTC().Format(time.RFC3339Nano)
			x.SemanticRole = "POSTING_RELATION_KNOWLEDGE"
			x.ContractHash, qe = s.traceContractHash(ctx, in.Kind, x.CanonicalFactID, x.SourcePayloadID)
			if qe != nil {
				relations.Close()
				return out, dbError(qe)
			}
			out.Sources = append(out.Sources, x)
		}
		if qe = relations.Err(); qe != nil {
			return out, qe
		}
		relations.Close()
	}
	if in.Kind == "CATEGORY_DEMAND" || in.Kind == "CATEGORY_FUNNEL" || in.Kind == "CATEGORY_SEARCH" {
		moscow, _ := time.LoadLocation("Europe/Moscow")
		fromBoundary := time.Date(in.Period.From.Year(), in.Period.From.Month(), in.Period.From.Day(), 0, 0, 0, 0, moscow)
		toBoundary := time.Date(in.Period.To.Year(), in.Period.To.Month(), in.Period.To.Day(), 0, 0, 0, 0, moscow).AddDate(0, 0, 1)
		var assignmentsQuery string
		var assignmentArgs []any
		switch in.Kind {
		case "CATEGORY_FUNNEL":
			assignmentsQuery = `SELECT a.id,a.observed_from,COALESCE(dr.source_revision_hash,'') FROM sku_category_assignments a JOIN canonical_skus sku ON sku.id=a.canonical_sku_id JOIN collection_domain_runs dr ON dr.id=a.collection_domain_run_id WHERE sku.seller_account_id=$1 AND a.observed_from<=$2 AND a.observed_from<$5 AND(a.observed_to IS NULL OR a.observed_to>$6)AND EXISTS(SELECT 1 FROM funnel_observations f WHERE f.seller_account_id=$1 AND f.canonical_sku_id=a.canonical_sku_id AND f.scope_kind='SKU' AND((f.period_end=f.period_start AND f.period_start BETWEEN $3 AND $4)OR(f.period_start=$3 AND f.period_end=$4))AND f.observed_at<=$2)ORDER BY a.id`
			assignmentArgs = []any{c.SellerAccountID(), c.CutoffAt(), in.Period.From, in.Period.To, toBoundary, fromBoundary}
		case "CATEGORY_SEARCH":
			assignmentsQuery = `SELECT a.id,a.observed_from,COALESCE(dr.source_revision_hash,'') FROM sku_category_assignments a JOIN canonical_skus sku ON sku.id=a.canonical_sku_id JOIN collection_domain_runs dr ON dr.id=a.collection_domain_run_id WHERE sku.seller_account_id=$1 AND a.observed_from<=$2 AND a.observed_from<$5 AND(a.observed_to IS NULL OR a.observed_to>$6)AND EXISTS(SELECT 1 FROM search_sku_summaries f WHERE f.seller_account_id=$1 AND f.canonical_sku_id=a.canonical_sku_id AND f.period_start=$3 AND f.period_end=$4 AND f.observed_at<=$2)ORDER BY a.id`
			assignmentArgs = []any{c.SellerAccountID(), c.CutoffAt(), in.Period.From, in.Period.To, toBoundary, fromBoundary}
		default:
			assignmentsQuery = `SELECT a.id,a.observed_from,COALESCE(dr.source_revision_hash,'') FROM sku_category_assignments a JOIN canonical_skus sku ON sku.id=a.canonical_sku_id JOIN collection_domain_runs dr ON dr.id=a.collection_domain_run_id WHERE sku.seller_account_id=$1 AND a.observed_from<=$2 AND a.observed_from<$5 AND(a.observed_to IS NULL OR a.observed_to>$6)AND EXISTS(SELECT 1 FROM sales_demand_daily f WHERE f.seller_account_id=$1 AND f.canonical_sku_id=a.canonical_sku_id AND f.business_date BETWEEN $3 AND $4 AND f.observed_at<=$2)ORDER BY a.id`
			assignmentArgs = []any{c.SellerAccountID(), c.CutoffAt(), in.Period.From, in.Period.To, toBoundary, fromBoundary}
		}
		assignments, queryErr := s.pool.Query(ctx, assignmentsQuery, assignmentArgs...)
		if queryErr != nil {
			return out, dbError(queryErr)
		}
		defer assignments.Close()
		for assignments.Next() {
			var source factualapi.TraceSource
			var observed time.Time
			if queryErr = assignments.Scan(&source.CanonicalFactID, &observed, &source.ContractHash); queryErr != nil {
				return out, dbError(queryErr)
			}
			source.EndpointKey = "CANONICAL_CATEGORY_ASSIGNMENT"
			source.SemanticRole = "HISTORICAL_CATEGORY_ASSIGNMENT"
			source.CanonicalObservedAt = observed.UTC().Format(time.RFC3339Nano)
			out.Sources = append(out.Sources, source)
		}
		if queryErr = assignments.Err(); queryErr != nil {
			return out, queryErr
		}
	}
	if in.Kind == "STOCK_COVERAGE" {
		x, traceErr := s.GetStockCoverage(ctx, c, in.EntityID)
		if traceErr != nil {
			return out, traceErr
		}
		out.Inputs = append(out.Inputs, x.Demand7DFactRef, x.Demand30DFactRef)
		out.Limitations = append(out.Limitations, factualapi.Limitation{Code: x.FormulaPolicy, Message: "coverage derives only from canonical sellable stock and canonical Sales demand"})
	}
	if in.Kind == "AD_SPEND_SHARE" && in.Period != nil {
		x, traceErr := s.GetAdvertisingSpendShare(ctx, c, "", in.Period)
		if traceErr != nil {
			return out, traceErr
		}
		out.Inputs = append(out.Inputs, x.AdvertisingFactRef, x.SalesFactRef)
		out.Limitations = append(out.Limitations, factualapi.Limitation{Code: x.FormulaPolicy, Message: "spend share denominator is canonical Sales ordered amount"})
	}
	return out, nil
}

func (s *Service) verifyFactResult(ctx context.Context, c factualapi.FactualReadContext, in factualapi.TraceInput) error {
	var ref string
	var e error
	switch in.Kind {
	case "ACCOUNT":
		var x factualapi.Account
		x, e = s.GetAccount(ctx, c)
		ref = x.Meta.FactRef
	case "CPO_ATTRIBUTION":
		var x factualapi.CPOAttribution
		x, e = s.getCPOAttribution(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "ACCOUNT_HEALTH_SUMMARY":
		var x factualapi.AccountHealthSummary
		x, e = s.GetAccountHealthSummary(ctx, c)
		ref = x.Meta.FactRef
	case "ACCOUNT_RATING":
		var x factualapi.AccountRating
		x, e = s.GetRating(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "RATING_PERIOD":
		var x factualapi.RatingPeriodFact
		var value pgtype.Text
		e = s.pool.QueryRow(ctx, `SELECT id,account_health_metric_id,revision_no,period_from,period_to,provider_value::text,COALESCE(provider_status,''),observed_at FROM account_rating_period_facts WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.RatingID, &x.RevisionNo, &x.Period.From, &x.Period.To, &value, &x.ProviderStatus, &x.ObservedAt)
		if e == nil {
			x.Value = decimalValue(textValue(value), "RATING_VALUE_UNAVAILABLE")
			x.Meta, e = s.meta(c, "HEALTH.RATING_HISTORY", "RATING_PERIOD", x.ID, nil, factHash(x))
			ref = x.Meta.FactRef
		}
	case "RATING_THRESHOLD":
		var x factualapi.RatingThreshold
		var from, to pgtype.Text
		e = s.pool.QueryRow(ctx, `SELECT id,account_health_metric_id,version_no,provider_status,threshold_from::text,threshold_to::text,observed_at FROM account_rating_threshold_observations WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.RatingID, &x.VersionNo, &x.ProviderStatus, &from, &to, &x.ObservedAt)
		if e == nil {
			x.ThresholdFrom = decimalValue(textValue(from), "THRESHOLD_FROM_UNAVAILABLE")
			x.ThresholdTo = decimalValue(textValue(to), "THRESHOLD_TO_UNAVAILABLE")
			x.Meta, e = s.meta(c, "HEALTH.THRESHOLDS", "RATING_THRESHOLD", x.ID, nil, factHash(x))
			ref = x.Meta.FactRef
		}
	case "ACCOUNT_PENALTY_EVENT":
		var x factualapi.AccountPenaltyEvent
		var rating pgtype.Int8
		var occurred pgtype.Timestamptz
		var amount pgtype.Text
		var currency string
		e = s.pool.QueryRow(ctx, `SELECT id,revision_no,provider_event_key,account_health_metric_id,occurred_at,penalty_amount::text,COALESCE(currency_code,''),COALESCE(description,''),observed_at FROM account_penalty_event_facts WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.RevisionNo, &x.ProviderEventKey, &rating, &occurred, &amount, &currency, &x.Description, &x.ObservedAt)
		if e == nil {
			if rating.Valid {
				x.RatingID = &rating.Int64
			}
			if occurred.Valid {
				x.OccurredAt = &occurred.Time
			}
			x.Amount = moneyValue(amount, currency, "PENALTY_AMOUNT_UNAVAILABLE")
			x.Meta, e = s.meta(c, "HEALTH.PENALTY_EVENTS", "ACCOUNT_PENALTY_EVENT", x.ID, nil, factHash(x))
			ref = x.Meta.FactRef
		}
	case "FBS_ERROR_INDEX":
		var x factualapi.FBSErrorIndex
		x, e = s.GetFBSErrorIndex(ctx, c)
		ref = x.Meta.FactRef
	case "FBS_RATING_POSTING_ERROR":
		var item factualapi.FBSRatingPostingError
		var posting pgtype.Int8
		var occurred pgtype.Timestamptz
		e = s.pool.QueryRow(ctx, `SELECT id,revision_no,provider_event_key,canonical_posting_id,COALESCE(source_posting_number,''),posting_resolution_status,COALESCE(error_code,''),COALESCE(error_message,''),occurred_at,observed_at FROM fbs_posting_error_events WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&item.ID, &item.RevisionNo, &item.ProviderEventKey, &posting, &item.SourcePostingNumber, &item.PostingResolutionStatus, &item.ErrorCode, &item.ErrorMessage, &occurred, &item.ObservedAt)
		if e == nil {
			if posting.Valid {
				item.CanonicalPostingID = &posting.Int64
			}
			if occurred.Valid {
				item.OccurredAt = &occurred.Time
			}
			item.Meta, e = s.meta(c, "HEALTH.FBS_POSTING_ERRORS", "FBS_RATING_POSTING_ERROR", item.ID, nil, factHash(item))
			ref = item.Meta.FactRef
		}
	case "SKU":
		var x factualapi.SKU
		x, e = s.GetSKU(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "CATEGORY":
		var x factualapi.Category
		x, e = s.GetCategory(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "ORDER":
		var x factualapi.Order
		x, e = s.GetOrder(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "POSTING":
		var x factualapi.Posting
		x, e = s.GetPosting(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "RETURN":
		var x factualapi.Return
		x, e = s.GetReturn(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "SKU_STOCK":
		if in.AuxiliaryID <= 0 {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "stock snapshot id missing")
		}
		var m map[int64]factualapi.SKUStock
		m, e = s.fetchStocks(ctx, c, in.AuxiliaryID, []int64{in.EntityID})
		if e == nil {
			x, ok := m[in.EntityID]
			if !ok {
				return notFound("SKU stock")
			}
			x.Meta, e = s.metaWithAux(c, "STOCK.SKU_CURRENT", "SKU_STOCK", in.EntityID, in.AuxiliaryID, nil, factHash(x))
			ref = x.Meta.FactRef
		}
	case "STOCK_SNAPSHOT":
		var x factualapi.StockSnapshot
		x, e = s.GetStockSnapshot(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "STOCK_COVERAGE":
		var x factualapi.StockCoverage
		x, e = s.GetStockCoverage(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "PRICE_OBSERVATION":
		var x factualapi.SKUPrice
		x, e = s.getPriceObservation(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "CAMPAIGN":
		var x factualapi.Campaign
		x, e = s.GetCampaign(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "AD_DAILY":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x []factualapi.AdvertisingDailyFact
		x, e = s.GetCampaignDailyFacts(ctx, c, in.EntityID, "", in.Period)
		if e == nil && len(x) > 0 {
			ref = x[0].Meta.FactRef
		}
	case "CAMPAIGN_DRR":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.CampaignDRR
		x, e = s.GetCampaignDRR(ctx, c, in.EntityID, "", in.Period)
		ref = x.Meta.FactRef
	case "AD_ACCOUNT":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.AdvertisingAccountSummary
		x, e = s.GetAdvertisingAccountSummary(ctx, c, "", in.Period)
		ref = x.Meta.FactRef
	case "AD_SPEND_SHARE":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.AdvertisingSpendShare
		x, e = s.GetAdvertisingSpendShare(ctx, c, "", in.Period)
		ref = x.Meta.FactRef
	case "SKU_ADVERTISING":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.SKUAdvertising
		x, e = s.GetSKUAdvertising(ctx, c, in.EntityID, "", in.Period)
		ref = x.Meta.FactRef
	case "ACCOUNT_DEMAND":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.Demand
		x, e = s.GetAccountDemand(ctx, c, "", in.Period)
		ref = x.Meta.FactRef
	case "SKU_DEMAND":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.SKUDemand
		x, e = s.GetSKUDemand(ctx, c, in.EntityID, "", in.Period)
		ref = x.Demand.Meta.FactRef
	case "CATEGORY_DEMAND":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var id *int64
		if in.EntityID > 0 {
			v := in.EntityID
			id = &v
		}
		var x factualapi.CategoryDemand
		x, e = s.GetCategoryDemand(ctx, c, id, "", in.Period)
		ref = x.Demand.Meta.FactRef
	case "SKU_DEMAND_DAY":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x []factualapi.DemandPoint
		x, e = s.GetDemandSeries(ctx, c, in.EntityID, "", in.Period)
		if e == nil && len(x) == 1 {
			ref = x[0].Meta.FactRef
		}
	case "LIFECYCLE_EVENT":
		var x factualapi.LifecycleEvent
		var order, posting, sku, ret pgtype.Int8
		var effective, wf, wt pgtype.Timestamptz
		var pd, bd pgtype.Date
		if e = s.pool.QueryRow(ctx, `SELECT id,event_type,time_quality,canonical_order_id,canonical_posting_id,canonical_sku_id,canonical_return_id,effective_at,provider_date,business_date,effective_window_from_at,effective_window_to_at,observed_at FROM sales_lifecycle_events WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.EventType, &x.TimeQuality, &order, &posting, &sku, &ret, &effective, &pd, &bd, &wf, &wt, &x.ObservedAt); e != nil {
			return dbError(e)
		}
		if order.Valid {
			x.CanonicalOrderID = &order.Int64
		}
		if posting.Valid {
			x.CanonicalPostingID = &posting.Int64
		}
		if sku.Valid {
			x.CanonicalSKUID = &sku.Int64
		}
		if ret.Valid {
			x.CanonicalReturnID = &ret.Int64
		}
		if effective.Valid {
			x.EffectiveAt = &effective.Time
		}
		if pd.Valid {
			x.ProviderDate = &pd.Time
		}
		if bd.Valid {
			x.BusinessDate = &bd.Time
		}
		if wf.Valid {
			x.WindowFrom = &wf.Time
		}
		if wt.Valid {
			x.WindowTo = &wt.Time
		}
		if factHash(x) != in.ResultHash {
			return factualapi.NewError(factualapi.CodeFactTraceResultMismatch, "fact result hash no longer matches deterministic re-execution")
		}
		return nil
	case "FINANCE_DAY":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var days []factualapi.FinanceDailyNet
		days, e = s.GetFinanceDailyLedger(ctx, c, "", in.Period)
		if e == nil {
			for _, day := range days {
				parsed, pe := s.signer.ParseFactRef(day.Meta.FactRef, c)
				if pe == nil && parsed.EntityID == in.EntityID {
					ref = day.Meta.FactRef
					break
				}
			}
			if ref == "" {
				return notFound("finance day")
			}
		}
	case "FINANCE_ACCOUNT":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.FinanceAccountSummary
		x, e = s.GetFinanceAccountSummary(ctx, c, "", in.Period)
		ref = x.Meta.FactRef
	case "FINANCE_ACCRUAL":
		var x factualapi.FinanceAccrual
		var amount, currency string
		e = s.pool.QueryRow(ctx, `SELECT r.id,r.day_revision_id,d.business_date,r.row_ordinal,r.provider_type_id,COALESCE(r.provider_accrued_category,''),COALESCE(r.unit_number,''),r.total_amount_signed::text,r.currency_code,COALESCE(v.mapping_status,'UNMAPPED')FROM finance_accrual_records r JOIN finance_accrual_day_revisions d ON d.id=r.day_revision_id AND d.seller_account_id=r.seller_account_id LEFT JOIN finance_provider_type_versions v ON v.finance_provider_type_id=r.finance_provider_type_id AND v.valid_from<=$3 AND(v.valid_to IS NULL OR v.valid_to>$3)WHERE r.seller_account_id=$1 AND r.id=$2 AND d.observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.DayRevisionID, &x.BusinessDate, &x.RowOrdinal, &x.ProviderTypeID, &x.ProviderAccruedCategory, &x.UnitNumber, &amount, &currency, &x.MappingStatus)
		if e == nil {
			x.Total = factualapi.SignedMoney{Amount: amount, CurrencyCode: currency}
			x.Meta, e = s.meta(c, "FINANCE.ACCRUAL_BREAKDOWN", "FINANCE_ACCRUAL", x.ID, in.Period, factHash(x))
			ref = x.Meta.FactRef
		}
	case "FINANCE_BALANCE":
		var x factualapi.FinanceBalance
		x, e = s.GetFinanceBalance(ctx, c)
		ref = x.Meta.FactRef
	case "SKU_SEARCH_SUMMARY":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.SKUSearchSummary
		x, e = s.GetSKUSearchSummary(ctx, c, in.EntityID, "", in.Period)
		if e != nil {
			var apiErr *factualapi.Error
			if errors.As(e, &apiErr) && apiErr.Code == factualapi.CodeNotFound {
				var summaryID, sourceSKU int64
				var canonical pgtype.Int8
				var resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion string
				var p factualapi.Period
				e = s.pool.QueryRow(ctx, `SELECT x.id,x.canonical_sku_id,x.source_sku,x.sku_resolution_status,x.period_kind,x.provider_search_gmv::text,x.orders::text,COALESCE(x.unique_search_users::text,''),COALESCE(x.unique_view_users::text,''),COALESCE(x.position::text,''),COALESCE(x.view_conversion::text,''),x.period_start,x.period_end FROM search_sku_summaries x WHERE x.seller_account_id=$1 AND x.source_sku=$2 AND x.period_start=$3 AND x.period_end=$4 AND x.observed_at<=$5 ORDER BY x.observed_at DESC,x.revision_no DESC,x.id DESC LIMIT 1`, c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()).Scan(&summaryID, &canonical, &sourceSKU, &resolution, &periodKind, &gmv, &orders, &searchUsers, &viewUsers, &position, &conversion, &p.From, &p.To)
				if e == nil {
					var cid *int64
					if canonical.Valid {
						v := canonical.Int64
						cid = &v
					}
					x, e = s.mapSKUSearchSummary(c, summaryID, cid, sourceSKU, resolution, periodKind, gmv, orders, searchUsers, viewUsers, position, conversion, p)
				}
			}
		}
		if e == nil {
			ref = x.Meta.FactRef
		}
	case "SEARCH_DETAIL":
		var x factualapi.SearchQueryDetail
		var gmv, orders string
		e = s.pool.QueryRow(ctx, `SELECT id,search_query_id,canonical_sku_id,source_sku,sku_resolution_status,provider_rank,provider_search_gmv::text,orders::text,retrieval_scope,retrieval_reason,top_n_limit,COALESCE(limit_by_sku,top_n_limit),COALESCE(sort_by,'orders'),COALESCE(sort_dir,'DESC'),COALESCE(page,1),COALESCE(page_size,100),collection_policy,period_start,period_end FROM search_query_details WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &x.SearchQueryID, &x.CanonicalSKUID, &x.SourceSKU, &x.SKUResolutionStatus, &x.ProviderRank, &gmv, &orders, &x.RetrievalScope, &x.RetrievalReason, &x.TopNLimit, &x.LimitBySKU, &x.SortBy, &x.SortDir, &x.Page, &x.PageSize, &x.CollectionPolicy, &x.Period.From, &x.Period.To)
		if e == nil {
			x.ProviderSearchGMV, e = searchMoney(gmv)
			if e == nil {
				x.Orders, e = searchDecimal(orders)
			}
			if e == nil {
				x.Meta, e = s.meta(c, "SEARCH.QUERY_DETAILS", "SEARCH_DETAIL", x.ID, &x.Period, factHash(x))
				ref = x.Meta.FactRef
			}
		}
	case "SEARCH_QUERY":
		var x factualapi.SearchQueryFact
		x, e = s.GetSearchQueryFact(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "CATEGORY_SEARCH":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.SearchCategorySummary
		x, e = s.GetCategorySearch(ctx, c, in.EntityID, "", in.Period)
		ref = x.Meta.FactRef
	case "MARKET_SEARCH":
		if in.Period == nil {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "period missing")
		}
		var x factualapi.MarketSearchFact
		e = s.pool.QueryRow(ctx, `SELECT search_query_id,provider_rank,period_start,period_end FROM search_market_facts WHERE seller_account_id=$1 AND search_query_id=$2 AND period_start=$3 AND period_end=$4 AND observed_at<=$5 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), in.EntityID, in.Period.From, in.Period.To, c.CutoffAt()).Scan(&x.SearchQueryID, &x.ProviderRank, &x.Period.From, &x.Period.To)
		if e == nil {
			x.Meta, e = s.meta(c, "SEARCH.MARKET_SEARCH", "MARKET_SEARCH", x.SearchQueryID, &x.Period, factHash(x))
			ref = x.Meta.FactRef
		}
	case "PROMOTION":
		var x factualapi.Promotion
		x, e = s.GetPromotion(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "SKU_PROMOTION":
		if in.AuxiliaryID <= 0 {
			return factualapi.NewError(factualapi.CodeFactRefInvalid, "promotion membership id missing")
		}
		var item factualapi.SKUPromotion
		var starts, ends, knownTo pgtype.Timestamptz
		var discount, actionPrice pgtype.Text
		var canonical, product, sku, stock pgtype.Int8
		e = s.pool.QueryRow(ctx, `SELECT m.id,p.id,p.provider_action_id,v.version_no,p.promotion_origin,COALESCE(v.title,''),v.provider_state,COALESCE(v.discount_type,''),COALESCE(v.currency_code,''),v.starts_at,v.ends_at,v.discount_value::text,v.observed_at,m.canonical_sku_id,m.source_product_id,m.source_sku,COALESCE(m.source_offer_id,''),m.sku_resolution_status,m.time_quality,m.action_price::text,m.action_stock,m.known_from,m.known_to FROM promotion_sku_memberships m JOIN promotions p ON p.id=m.promotion_id AND p.seller_account_id=m.seller_account_id JOIN LATERAL(SELECT * FROM promotion_versions x WHERE x.promotion_id=p.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE m.seller_account_id=$1 AND m.canonical_sku_id=$3 AND m.id=$4 AND m.known_from<=$2 AND(m.known_to IS NULL OR m.known_to>$2)AND p.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), in.EntityID, in.AuxiliaryID).Scan(&item.MembershipID, &item.ID, &item.ProviderActionID, &item.VersionNo, &item.Origin, &item.Title, &item.ProviderState, &item.DiscountType, &item.CurrencyCode, &starts, &ends, &discount, &item.ObservedAt, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &item.TimeQuality, &actionPrice, &stock, &item.KnownFrom, &knownTo)
		if e == nil {
			if starts.Valid {
				item.StartsAt = &starts.Time
			}
			if ends.Valid {
				item.EndsAt = &ends.Time
			}
			item.DiscountValue = decimalValue(textValue(discount), "DISCOUNT_VALUE_UNAVAILABLE")
			assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
			item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
			item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
			item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
			item.Meta, e = s.metaWithAux(c, "PROMOTIONS.MEMBERSHIP", "SKU_PROMOTION", in.EntityID, item.MembershipID, nil, factHash(item))
			ref = item.Meta.FactRef
		}
	case "PROMOTION_ELIGIBILITY":
		var x factualapi.PromotionEligibility
		x, e = s.GetPromotionEligibility(ctx, c, in.EntityID, factualapi.PageRequest{})
		ref = x.Meta.FactRef
	case "PROMOTION_MEMBERSHIP_INTERVAL":
		var item factualapi.PromotionMembershipInterval
		var canonical, product, sku, stock pgtype.Int8
		var actionPrice pgtype.Text
		var knownTo pgtype.Timestamptz
		e = s.pool.QueryRow(ctx, `SELECT id,promotion_id,canonical_sku_id,source_product_id,source_sku,COALESCE(source_offer_id,''),sku_resolution_status,time_quality,action_price::text,action_stock,known_from,known_to FROM promotion_sku_memberships WHERE seller_account_id=$1 AND id=$2 AND known_from<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&item.ID, &item.PromotionID, &canonical, &product, &sku, &item.SourceOfferID, &item.ResolutionStatus, &item.TimeQuality, &actionPrice, &stock, &item.KnownFrom, &knownTo)
		if e == nil {
			assignIDs(&item.CanonicalSKUID, &item.SourceProductID, &item.SourceSKU, canonical, product, sku)
			item.KnownTo = knownToAsOf(knownTo, c.CutoffAt())
			item.ActionPrice = moneyValue(actionPrice, "RUB", "ACTION_PRICE_UNAVAILABLE")
			item.ActionStock = intValue(stock, "ACTION_STOCK_UNAVAILABLE")
			item.Meta, e = s.meta(c, "PROMOTIONS.HISTORY", "PROMOTION_MEMBERSHIP_INTERVAL", item.ID, nil, factHash(item))
			ref = item.Meta.FactRef
		}
	case "SUPPLY_ORDER":
		var x factualapi.SupplyOrder
		x, e = s.GetSupplyOrder(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "SUPPLY":
		var x factualapi.InboundSupply
		x, e = s.GetSupply(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	case "SUPPLY_STATE_OBSERVATION":
		var x factualapi.SupplyStateObservation
		var orderID, sid pgtype.Int8
		e = s.pool.QueryRow(ctx, `SELECT id,supply_order_id,supply_id,provider_state_code,canonical_state,state_mapping_status,state_mapping_version,observed_at FROM supply_state_observations WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&x.ID, &orderID, &sid, &x.ProviderState, &x.CanonicalState, &x.StateMappingStatus, &x.StateMappingVersion, &x.ObservedAt)
		if e == nil {
			if orderID.Valid {
				v := orderID.Int64
				x.SupplyOrderID = &v
			}
			if sid.Valid {
				v := sid.Int64
				x.SupplyID = &v
			}
			x.Meta, e = s.meta(c, "SUPPLY.HISTORY", "SUPPLY_STATE_OBSERVATION", x.ID, nil, factHash(x))
			ref = x.Meta.FactRef
		}
	case "SUPPLY_ACCEPTANCE_ACT":
		var out factualapi.SupplyAcceptance
		var amount, currency pgtype.Text
		e = s.pool.QueryRow(ctx, `SELECT id,supply_id,provider_act_id,COALESCE(provider_state,''),total_amount::text,currency_code,completeness_status,observed_at FROM supply_acceptance_acts WHERE seller_account_id=$1 AND id=$2 AND observed_at<=$3`, c.SellerAccountID(), in.EntityID, c.CutoffAt()).Scan(&out.ID, &out.SupplyID, &out.ProviderActID, &out.ProviderState, &amount, &currency, &out.CompletenessStatus, &out.ObservedAt)
		if e == nil {
			out.CurrencyCode = textValue(currency)
			out.TotalAmount = moneyValue(amount, out.CurrencyCode, "ACCEPTANCE_AMOUNT_UNAVAILABLE")
			out.Meta, e = s.meta(c, "SUPPLY.ACCEPTANCE_SUMMARY", "SUPPLY_ACCEPTANCE_ACT", out.ID, nil, factHash(out))
			if out.CompletenessStatus == "PARTIAL" {
				out.Meta.Limitations = append(out.Meta.Limitations, factualapi.Limitation{Code: "SUPPLY_ACCEPTANCE_PARTIAL", Message: "acceptance product pages were incomplete"})
			}
			ref = out.Meta.FactRef
		}
	case "SKU_INBOUND":
		var x factualapi.SKUInbound
		x, e = s.GetSKUInbound(ctx, c, in.EntityID)
		ref = x.Meta.FactRef
	default:
		return nil
	}
	if e != nil {
		return e
	}
	verified, e := s.signer.ParseFactRef(ref, c)
	if e != nil {
		return e
	}
	if verified.ResultHash != in.ResultHash {
		return factualapi.NewError(factualapi.CodeFactTraceResultMismatch, "fact result hash no longer matches deterministic re-execution")
	}
	return nil
}

func (s *Service) traceContractHash(ctx context.Context, kind string, fact, payload int64) (string, error) {
	var hash string
	if payload > 0 {
		e := s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(sp.metadata_json->>'semantic_mapping_hash',''),dr.source_revision_hash,'') FROM source_payloads sp JOIN collection_domain_runs dr ON dr.id=sp.collection_domain_run_id WHERE sp.id=$1`, payload).Scan(&hash)
		return hash, e
	}
	var e error
	switch kind {
	case "SKU":
		e = s.pool.QueryRow(ctx, `SELECT COALESCE(dr.source_revision_hash,'') FROM canonical_sku_versions v JOIN collection_domain_runs dr ON dr.id=v.collection_domain_run_id WHERE v.id=$1`, fact).Scan(&hash)
	case "CATEGORY":
		e = s.pool.QueryRow(ctx, `SELECT COALESCE(dr.source_revision_hash,'') FROM provider_category_versions v JOIN taxonomy_sync_runs ts ON ts.id=v.taxonomy_sync_run_id JOIN collection_domain_runs dr ON dr.id=ts.collection_domain_run_id WHERE v.id=$1`, fact).Scan(&hash)
	default:
		return "", nil
	}
	return hash, e
}
