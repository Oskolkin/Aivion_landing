package stock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

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

func (r *Repository) ResolveStockUniverse(ctx context.Context, seller int64, cutoff time.Time) ([]FrozenSKU, error) {
	rows, err := r.pool.Query(ctx, `SELECT cs.id,cs.ozon_sku FROM canonical_skus cs JOIN LATERAL(SELECT catalog_state FROM canonical_sku_versions v WHERE v.canonical_sku_id=cs.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND v.catalog_state<>'UNOBSERVED' ORDER BY cs.ozon_sku`, seller, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FrozenSKU
	for rows.Next() {
		var x FrozenSKU
		if err = rows.Scan(&x.CanonicalSKUID, &x.OzonSKU); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *Repository) Finalize(ctx context.Context, seller, run int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("stock finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, err := lockRun(ctx, tx, seller, run, "STOCK")
		if err != nil || already {
			return err
		}
		accepted := r.now().UTC()
		if result.CompletedAt.IsZero() {
			result.CompletedAt = accepted
		}
		if result.StartedAt.IsZero() {
			result.StartedAt = result.CompletedAt
		}
		resolved, unresolved := 0, 0
		missing, unavailable := result.MissingResponseSKUCount, result.BatchUnavailableSKUCount
		for _, p := range result.Positions {
			if p.IsAbsence() {
				continue
			}
			if p.SKUResolutionStatus == "RESOLVED" {
				resolved++
			} else {
				unresolved++
			}
		}
		if missing == 0 && unavailable == 0 {
			for _, p := range result.Positions {
				switch p.AbsenceReason {
				case "MISSING_PROVIDER_RESPONSE_ROW":
					missing++
				case "SOURCE_BATCH_UNAVAILABLE":
					unavailable++
				}
			}
		}
		result.ResolvedSKUs, result.UnresolvedSKUs = resolved, unresolved
		result.MissingResponseSKUCount, result.BatchUnavailableSKUCount = missing, unavailable
		state := stockStateHash(*result)
		semantic := "VALIDATED"
		for _, p := range result.Positions {
			if p.QuantitySemanticStatus != "VALIDATED" || p.PoolIndependenceStatus == "UNVERIFIED" {
				semantic = "PENDING_ACCEPTANCE"
				break
			}
		}
		if result.CompletenessStatus == "" {
			result.CompletenessStatus = "COMPLETE"
		}
		err = tx.QueryRow(ctx, `INSERT INTO stock_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,missing_response_sku_count,batch_unavailable_sku_count,position_count,batches_expected,batches_completed,batches_failed,completeness_status,semantic_mapping_status,state_hash)VALUES($1,$2,$3,$4,$5,'OBSERVED_APPROX',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)RETURNING id`, seller, run, accepted, result.StartedAt, result.CompletedAt, result.UniverseHash, result.ExpectedSKUCount, result.RequestedSKUCount, result.ReceivedSKUCount, resolved, unresolved, missing, unavailable, len(result.Positions), result.BatchesExpected, result.BatchesCompleted, result.BatchesFailed, result.CompletenessStatus, semantic, state).Scan(&result.SnapshotID)
		if err != nil {
			return err
		}
		for _, p := range result.Positions {
			poolID, err := upsertPool(ctx, tx, seller, p, accepted)
			if err != nil {
				return err
			}
			locationID, err := upsertLocation(ctx, tx, seller, run, p, accepted)
			if err != nil {
				return err
			}
			var lineID int64
			err = tx.QueryRow(ctx, `INSERT INTO stock_snapshot_lines(seller_account_id,snapshot_id,canonical_sku_id,source_sku,sku_resolution_status,inventory_pool_id,inventory_location_id,fulfillment_type,sellable_units,reserved_units,non_sellable_units,provider_valid_stock_count,provider_present_count,provider_reserved_count,quantity_semantic_status,absence_reason,provider_fields_json,source_payload_id,observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NULLIF($16,''),$17,$18,$19)RETURNING id`, seller, result.SnapshotID, p.CanonicalSKUID, p.SourceSKU, p.SKUResolutionStatus, poolID, locationID, p.FulfillmentType, p.SellableUnits, p.ReservedUnits, p.NonSellableUnits, p.ProviderValidStockCount, p.ProviderPresentCount, p.ProviderReservedCount, p.QuantitySemanticStatus, p.AbsenceReason, jsonOrEmpty(p.ProviderFields), p.SourcePayloadID, accepted).Scan(&lineID)
			if err != nil {
				return err
			}
			if !p.IsAbsence() && p.SourcePayloadID != nil && (p.ProviderADS != "" || p.ProviderIDC != "" || p.ProviderTurnoverGrade != "") {
				_, err = tx.Exec(ctx, `INSERT INTO stock_turnover_observations(seller_account_id,stock_snapshot_line_id,provider_ads,provider_ads_cluster,provider_idc,provider_idc_cluster,provider_turnover_grade,provider_turnover_grade_cluster,provider_days_without_sales,provider_days_without_sales_cluster,source_payload_id,observed_at)VALUES($1,$2,NULLIF($3,'')::numeric,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,NULLIF($6,'')::numeric,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,$12)`, seller, lineID, p.ProviderADS, p.ProviderADSCluster, p.ProviderIDC, p.ProviderIDCCluster, p.ProviderTurnoverGrade, p.ProviderTurnoverGradeCluster, p.ProviderDaysWithoutSales, p.ProviderDaysWithoutSalesCluster, *p.SourcePayloadID, accepted)
				if err != nil {
					return err
				}
			}
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" || semantic != "VALIDATED" || unresolved > 0 || missing > 0 || unavailable > 0 {
			status = "PARTIAL"
		}
		cmd, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$5,pages_received=$6,completeness_status=$7,finished_at=$8,error_code=NULL,error_message=NULL,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, result.ReceivedSKUCount, len(result.Positions), result.BatchesCompleted, result.CompletenessStatus, accepted)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale stock domain transition")
		}
		return advanceHead(ctx, tx, seller, "STOCK", run, accepted)
	})
}

func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64, domain string) (bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:%s:%d", domain, seller)); err != nil {
		return false, err
	}
	var status, actual string
	if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &actual); err != nil {
		return false, err
	}
	if actual != domain {
		return false, fmt.Errorf("expected %s domain", domain)
	}
	var head int64
	err := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain=$2`, seller, domain).Scan(&head)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if (status == "COMPLETED" || status == "PARTIAL") && err == nil && head >= run {
		return true, nil
	}
	if err == nil && head > run {
		_, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status='CANCELLED',finished_at=NOW(),error_code='SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN',updated_at=NOW() WHERE id=$1`, run)
		return false, e
	}
	if status != "RUNNING" {
		return false, fmt.Errorf("%s domain must be RUNNING, got %s", domain, status)
	}
	return false, nil
}
func advanceHead(ctx context.Context, tx pgx.Tx, seller int64, domain string, run int64, at time.Time) error {
	_, e := tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,$2,$3,$4)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, domain, run, at)
	return e
}
func upsertPool(ctx context.Context, tx pgx.Tx, seller int64, p Position, at time.Time) (int64, error) {
	var id int64
	e := tx.QueryRow(ctx, `INSERT INTO inventory_pools(seller_account_id,provider_pool_key,fulfillment_type,pool_type,independence_status,first_observed_at,last_observed_at)VALUES($1,$2,$3,$4,$5,$6,$6)ON CONFLICT(seller_account_id,provider_pool_key)DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at,independence_status=EXCLUDED.independence_status RETURNING id`, seller, p.ProviderPoolKey, p.FulfillmentType, p.PoolType, p.PoolIndependenceStatus, at).Scan(&id)
	return id, e
}
func upsertLocation(ctx context.Context, tx pgx.Tx, seller, run int64, p Position, at time.Time) (*int64, error) {
	if p.ProviderLocationKey == "" || p.IsAbsence() {
		return nil, nil
	}
	var id int64
	e := tx.QueryRow(ctx, `INSERT INTO inventory_locations(seller_account_id,provider_location_key,location_type,first_observed_at,last_observed_at)VALUES($1,$2,$3,$4,$4)ON CONFLICT(seller_account_id,provider_location_key)DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at RETURNING id`, seller, p.ProviderLocationKey, p.LocationType, at).Scan(&id)
	if e != nil {
		return nil, e
	}
	if p.SourcePayloadID == nil {
		return &id, nil
	}
	h := factualapi.StableHash(p.LocationName, p.ParentProviderLocationKey)
	var latest int64
	var latestHash string
	e = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM inventory_location_versions WHERE inventory_location_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latest, &latestHash)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	if latestHash != h {
		_, e = tx.Exec(ctx, `INSERT INTO inventory_location_versions(seller_account_id,inventory_location_id,version_no,provider_name,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8)`, seller, id, latest+1, p.LocationName, h, at, *p.SourcePayloadID, run)
		if e != nil {
			return nil, e
		}
	}
	return &id, nil
}
func jsonOrEmpty(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(`{}`)
	}
	return v
}
func stockStateHash(r Result) string {
	parts := []string{r.UniverseHash, fmt.Sprint(r.ExpectedSKUCount), fmt.Sprint(r.BatchesCompleted), fmt.Sprint(r.BatchesFailed), fmt.Sprint(r.MissingResponseSKUCount), fmt.Sprint(r.BatchUnavailableSKUCount)}
	for _, p := range r.Positions {
		parts = append(parts, fmt.Sprint(p.SourceSKU), p.ProviderPoolKey, p.ProviderLocationKey, ptrString(p.SellableUnits), ptrString(p.ProviderValidStockCount), p.QuantitySemanticStatus, p.AbsenceReason)
	}
	sort.Strings(parts[6:])
	sum := sha256.Sum256([]byte(fmt.Sprint(parts)))
	return hex.EncodeToString(sum[:])
}
func ptrString(v *int64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(*v)
}
