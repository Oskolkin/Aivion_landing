package price

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
func (r *Repository) ResolvePriceUniverse(ctx context.Context, seller int64, cutoff time.Time) ([]FrozenSKU, error) {
	rows, e := r.pool.Query(ctx, `SELECT cs.id,cs.ozon_sku,COALESCE(v.provider_product_id,0),COALESCE(v.offer_id,'')FROM canonical_skus cs JOIN LATERAL(SELECT provider_product_id,offer_id,catalog_state FROM canonical_sku_versions x WHERE x.canonical_sku_id=cs.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND v.catalog_state<>'UNOBSERVED' ORDER BY cs.id`, seller, cutoff)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []FrozenSKU
	for rows.Next() {
		var x FrozenSKU
		if e = rows.Scan(&x.CanonicalSKUID, &x.OzonSKU, &x.ProviderProductID, &x.OfferID); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *Repository) Finalize(ctx context.Context, seller, run int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("price finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, e := lockRun(ctx, tx, seller, run)
		if e != nil || already {
			return e
		}
		at := r.now().UTC()
		if result.StartedAt.IsZero() {
			result.StartedAt = at
		}
		if result.CompletedAt.IsZero() {
			result.CompletedAt = at
		}
		resolved, unresolved := 0, 0
		for _, o := range result.Observations {
			if o.SKUResolutionStatus == "RESOLVED" {
				resolved++
			} else {
				unresolved++
			}
		}
		result.ResolvedSKUCount, result.UnresolvedSKUCount = resolved, unresolved
		if result.CompletenessStatus == "" {
			result.CompletenessStatus = "COMPLETE"
		}
		state := priceStateHash(*result)
		e = tx.QueryRow(ctx, `INSERT INTO price_collection_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,batches_expected,batches_completed,batches_failed,completeness_status,state_hash)VALUES($1,$2,$3,$4,$5,'OBSERVED_APPROX',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)RETURNING id`, seller, run, at, result.StartedAt, result.CompletedAt, result.UniverseHash, result.ExpectedSKUCount, result.RequestedSKUCount, result.ReceivedSKUCount, resolved, unresolved, result.BatchesExpected, result.BatchesCompleted, result.BatchesFailed, result.CompletenessStatus, state).Scan(&result.SnapshotID)
		if e != nil {
			return e
		}
		semanticPartial := false
		for _, o := range result.Observations {
			if o.SemanticMappingStatus != "VALIDATED" {
				semanticPartial = true
			}
			_, e = tx.Exec(ctx, `INSERT INTO price_snapshots(seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,seller_promo_price,customer_price,old_reference_price,minimum_allowed_price,retail_price,vat_rate,auto_action_enabled,price_index_color,price_index_json,promotion_context_json,provider_fields_json,semantic_mapping_status,state_hash,source_payload_id,premium_source_payload_id,reconciliation_status)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,'OBSERVED_APPROX',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NULLIF($19,''),$20,$21,$22,$23,$24,$25,$26,$27)`, seller, result.SnapshotID, o.CanonicalSKUID, o.SourceSKU, o.ProviderProductID, o.SKUResolutionStatus, run, at, o.CurrencyCode, decimalString(o.ProviderCurrentPrice), o.CurrentPriceState, decimalString(o.SellerPromoPrice), decimalString(o.CustomerPrice), decimalString(o.OldReferencePrice), decimalString(o.MinimumAllowedPrice), decimalString(o.RetailPrice), decimalString(o.VATRate), o.AutoActionEnabled, o.PriceIndexColor, jsonOrEmpty(o.PriceIndexJSON), jsonOrEmpty(o.PromotionContextJSON), jsonOrEmpty(o.ProviderFields), o.SemanticMappingStatus, observationHash(o), o.SourcePayloadID, o.PremiumSourcePayloadID, o.ReconciliationStatus)
			if e != nil {
				return e
			}
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" || unresolved > 0 || semanticPartial {
			status = "PARTIAL"
		}
		cmd, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$5,pages_received=$6,completeness_status=$7,finished_at=$8,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, result.ReceivedSKUCount, len(result.Observations), result.BatchesCompleted, result.CompletenessStatus, at)
		if e != nil {
			return e
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale price domain transition")
		}
		return advanceHead(ctx, tx, seller, run, at)
	})
}
func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:PRICE:%d", seller)); e != nil {
		return false, e
	}
	var status, domain string
	if e := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); e != nil {
		return false, e
	}
	if domain != "PRICE" {
		return false, errors.New("expected PRICE domain")
	}
	var head int64
	e := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='PRICE'`, seller).Scan(&head)
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
		return false, fmt.Errorf("PRICE domain must be RUNNING, got %s", status)
	}
	return false, nil
}
func advanceHead(ctx context.Context, tx pgx.Tx, seller, run int64, at time.Time) error {
	_, e := tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'PRICE',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, at)
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
func observationHash(o Observation) string {
	return factualapi.StableHash(fmt.Sprint(o.SourceSKU), fmt.Sprint(o.ProviderProductID), o.CurrencyCode, fmt.Sprint(decimalString(o.ProviderCurrentPrice)), fmt.Sprint(decimalString(o.SellerPromoPrice)), fmt.Sprint(decimalString(o.OldReferencePrice)), fmt.Sprint(decimalString(o.MinimumAllowedPrice)), o.PriceIndexColor, string(o.ProviderFields))
}
func priceStateHash(r Result) string {
	parts := []string{r.UniverseHash, fmt.Sprint(r.ExpectedSKUCount), fmt.Sprint(r.BatchesCompleted), fmt.Sprint(r.BatchesFailed)}
	for _, o := range r.Observations {
		parts = append(parts, observationHash(o))
	}
	sort.Strings(parts[4:])
	return factualapi.StableHash(parts...)
}
