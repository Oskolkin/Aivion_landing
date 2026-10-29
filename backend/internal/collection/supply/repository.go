package supply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func ptrInt64Marker(v *int64) string {
	if v == nil {
		return "NULL"
	}
	return strconv.FormatInt(*v, 10)
}

func ptrTimeMarker(v *time.Time) string {
	if v == nil {
		return "NULL"
	}
	return v.UTC().Format(time.RFC3339Nano)
}

func (r *Repository) ResolveSupplySKU(ctx context.Context, seller int64, productID, sourceSKU *int64, offerID string, cutoff time.Time) (*int64, string, error) {
	rows, err := r.pool.Query(ctx, `WITH candidates AS(
 SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND first_observed_at<=$2 AND $3::bigint IS NOT NULL AND ozon_sku=$3
 UNION SELECT c.id FROM canonical_skus c JOIN LATERAL(SELECT provider_product_id,offer_id FROM canonical_sku_versions v WHERE v.canonical_sku_id=c.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true
 WHERE c.seller_account_id=$1 AND c.first_observed_at<=$2 AND(($4::bigint IS NOT NULL AND v.provider_product_id=$4)OR(NULLIF($5,'')IS NOT NULL AND v.offer_id=$5))
)SELECT id FROM candidates ORDER BY id`, seller, cutoff, sourceSKU, productID, offerID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, "", err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	if len(ids) == 1 {
		return &ids[0], "RESOLVED", nil
	}
	if len(ids) > 1 {
		return nil, "CONFLICT", nil
	}
	return nil, "UNRESOLVED", nil
}

func (r *Repository) Finalize(ctx context.Context, seller, run int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("supply finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, err := lockRun(ctx, tx, seller, run)
		if err != nil || already {
			return err
		}
		at := result.StartedAt.UTC()
		if at.IsZero() {
			return errors.New("supply observation time is required")
		}
		orderIDs := map[string]int64{}
		for _, o := range result.Orders {
			var id int64
			var created bool
			err = tx.QueryRow(ctx, `INSERT INTO supply_orders(seller_account_id,provider_order_id,first_observed_at,last_observed_at)VALUES($1,$2,$3,$3)ON CONFLICT(seller_account_id,provider_order_id)DO UPDATE SET last_observed_at=GREATEST(supply_orders.last_observed_at,EXCLUDED.last_observed_at)RETURNING id,(xmax=0)`, seller, o.ProviderOrderID, at).Scan(&id, &created)
			if err != nil {
				return err
			}
			orderIDs[o.ProviderOrderID] = id
			if created {
				result.OrdersCreated++
			}
			location, err := upsertLocation(ctx, tx, seller, o.DropoffProviderLocationID, "SUPPLY_DROPOFF", at)
			if err != nil {
				return err
			}
			hash := factualapi.StableHash(o.ProviderState, o.CanonicalState, o.StateMappingStatus, ptrInt64Marker(location), ptrTimeMarker(o.TimeslotFrom), ptrTimeMarker(o.TimeslotTo), string(o.ProviderFields))
			written, err := writeOrderVersion(ctx, tx, seller, run, id, o, location, hash, at)
			if err != nil {
				return err
			}
			if written {
				result.VersionsWritten++
			}
			inserted, err := insertOrderStateObservation(ctx, tx, seller, run, id, o, at)
			if err != nil {
				return err
			}
			if inserted {
				result.StateObservationsWritten++
			}
		}
		supplyIDs := map[string]int64{}
		for _, supply := range result.Supplies {
			orderID := orderIDs[supply.ProviderOrderID]
			if orderID == 0 {
				return fmt.Errorf("supply %s references unknown order %s", supply.ProviderSupplyID, supply.ProviderOrderID)
			}
			var id, actualOrder int64
			var created bool
			err = tx.QueryRow(ctx, `INSERT INTO supplies(seller_account_id,supply_order_id,provider_supply_id,first_observed_at,last_observed_at)VALUES($1,$2,$3,$4,$4)ON CONFLICT(seller_account_id,provider_supply_id)DO UPDATE SET last_observed_at=GREATEST(supplies.last_observed_at,EXCLUDED.last_observed_at)RETURNING id,supply_order_id,(xmax=0)`, seller, orderID, supply.ProviderSupplyID, at).Scan(&id, &actualOrder, &created)
			if err != nil {
				return err
			}
			if actualOrder != orderID {
				return fmt.Errorf("provider supply %s changed parent order", supply.ProviderSupplyID)
			}
			supplyIDs[supply.ProviderSupplyID] = id
			if created {
				result.SuppliesCreated++
			}
			location, err := upsertLocation(ctx, tx, seller, supply.StorageProviderLocationID, "STORAGE_WAREHOUSE", at)
			if err != nil {
				return err
			}
			hash := factualapi.StableHash(supply.ProviderState, supply.CanonicalState, supply.StateMappingStatus, ptrInt64Marker(location), string(supply.ProviderFields))
			written, err := writeSupplyVersion(ctx, tx, seller, run, id, supply, location, hash, at)
			if err != nil {
				return err
			}
			if written {
				result.VersionsWritten++
			}
			inserted, err := insertSupplyStateObservation(ctx, tx, seller, run, id, supply, at)
			if err != nil {
				return err
			}
			if inserted {
				result.StateObservationsWritten++
			}
		}
		for _, bundle := range result.Bundles {
			supplyID := supplyIDs[bundle.ProviderSupplyID]
			if supplyID == 0 {
				return fmt.Errorf("bundle references unknown supply %s", bundle.ProviderSupplyID)
			}
			parts := []string{bundle.ProviderBundleID, bundle.CompletenessStatus}
			for _, item := range bundle.Items {
				parts = append(parts, ptrInt64Marker(item.SourceSKU), ptrInt64Marker(item.SourceProductID), item.SourceOfferID, ptrInt64Marker(item.DeclaredUnits), ptrInt64Marker(item.CanonicalSKUID), item.SKUResolutionStatus)
			}
			hash := factualapi.StableHash(parts...)
			var latest int64
			var latestHash, latestStatus string
			err = tx.QueryRow(ctx, `SELECT version_no,composition_hash,completeness_status FROM supply_bundle_versions WHERE supply_id=$1 ORDER BY version_no DESC LIMIT 1`, supplyID).Scan(&latest, &latestHash, &latestStatus)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash && latestStatus == bundle.CompletenessStatus {
				continue
			}
			var versionID int64
			err = tx.QueryRow(ctx, `INSERT INTO supply_bundle_versions(seller_account_id,supply_id,version_no,provider_bundle_id,completeness_status,page_count,item_count,composition_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11)RETURNING id`, seller, supplyID, latest+1, bundle.ProviderBundleID, bundle.CompletenessStatus, bundle.PageCount, len(bundle.Items), hash, at, bundle.SourcePayloadID, run).Scan(&versionID)
			if err != nil {
				return err
			}
			for i, item := range bundle.Items {
				if _, err = tx.Exec(ctx, `INSERT INTO supply_planned_items(seller_account_id,supply_bundle_version_id,line_ordinal,canonical_sku_id,source_sku,source_product_id,source_offer_id,sku_resolution_status,declared_units,provider_fields_json,source_payload_id)VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11)`, seller, versionID, i+1, item.CanonicalSKUID, item.SourceSKU, item.SourceProductID, item.SourceOfferID, item.SKUResolutionStatus, item.DeclaredUnits, jsonOrEmpty(item.ProviderFields), item.SourcePayloadID); err != nil {
					return err
				}
			}
			result.BundlesWritten++
		}
		for _, act := range result.Acceptances {
			supplyID := supplyIDs[act.ProviderSupplyID]
			if supplyID == 0 {
				return fmt.Errorf("acceptance references unknown supply %s", act.ProviderSupplyID)
			}
			completeness := act.CompletenessStatus
			if completeness != "COMPLETE" && completeness != "PARTIAL" {
				completeness = "PARTIAL"
			}
			var amount any
			if strings.TrimSpace(act.TotalAmount) != "" {
				amount = act.TotalAmount
			}
			var currency any
			if strings.TrimSpace(act.CurrencyCode) != "" {
				currency = act.CurrencyCode
			}
			var actID int64
			err = tx.QueryRow(ctx, `INSERT INTO supply_acceptance_acts(seller_account_id,supply_id,provider_act_id,provider_state,total_amount,currency_code,completeness_status,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10)RETURNING id`, seller, supplyID, act.ProviderActID, act.ProviderState, amount, currency, completeness, at, act.SourcePayloadID, run).Scan(&actID)
			if err != nil {
				return err
			}
			for i, item := range act.Items {
				var itemAmount any
				if strings.TrimSpace(item.Amount) != "" {
					itemAmount = item.Amount
				}
				var itemCurrency any
				if strings.TrimSpace(item.CurrencyCode) != "" {
					itemCurrency = item.CurrencyCode
				}
				if _, err = tx.Exec(ctx, `INSERT INTO supply_acceptance_items(seller_account_id,supply_acceptance_act_id,line_ordinal,canonical_sku_id,source_sku,source_offer_id,sku_resolution_status,accepted_units,shortage_units,surplus_units,amount,currency_code,provider_fields_json,source_payload_id)VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,$14)`, seller, actID, i+1, item.CanonicalSKUID, item.SourceSKU, item.SourceOfferID, item.SKUResolutionStatus, item.AcceptedUnits, item.ShortageUnits, item.SurplusUnits, itemAmount, itemCurrency, jsonOrEmpty(item.ProviderFields), item.SourcePayloadID); err != nil {
					return err
				}
			}
			result.AcceptancesWritten++
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" {
			status = "PARTIAL"
		}
		cmd, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$5,pages_received=$6,completeness_status=$7,finished_at=$8,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, len(result.Orders)+len(result.Supplies)+len(result.Bundles)+len(result.Acceptances), result.VersionsWritten+result.BundlesWritten+result.AcceptancesWritten+result.StateObservationsWritten, result.ListPages+result.DetailRequests+result.BundlePages+result.AcceptancePages, result.CompletenessStatus, result.CompletedAt)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale SUPPLY domain transition")
		}
		_, err = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'SUPPLY',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, result.CompletedAt)
		return err
	})
}

func insertOrderStateObservation(ctx context.Context, tx pgx.Tx, seller, run, orderID int64, o OrderObservation, at time.Time) (bool, error) {
	var prevProvider, prevCanonical, prevStatus string
	err := tx.QueryRow(ctx, `SELECT provider_state_code,canonical_state,state_mapping_status FROM supply_state_observations WHERE supply_order_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1`, orderID).Scan(&prevProvider, &prevCanonical, &prevStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil && !StateObservationChanged(prevProvider, prevCanonical, prevStatus, o.ProviderState, o.CanonicalState, o.StateMappingStatus) {
		return false, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO supply_state_observations(seller_account_id,supply_order_id,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, seller, orderID, o.ProviderState, o.CanonicalState, StateMappingVersion, o.StateMappingStatus, at, o.SourcePayloadID, run)
	return err == nil, err
}

func insertSupplyStateObservation(ctx context.Context, tx pgx.Tx, seller, run, supplyID int64, s SupplyObservation, at time.Time) (bool, error) {
	var prevProvider, prevCanonical, prevStatus string
	err := tx.QueryRow(ctx, `SELECT provider_state_code,canonical_state,state_mapping_status FROM supply_state_observations WHERE supply_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1`, supplyID).Scan(&prevProvider, &prevCanonical, &prevStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil && !StateObservationChanged(prevProvider, prevCanonical, prevStatus, s.ProviderState, s.CanonicalState, s.StateMappingStatus) {
		return false, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO supply_state_observations(seller_account_id,supply_id,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, seller, supplyID, s.ProviderState, s.CanonicalState, StateMappingVersion, s.StateMappingStatus, at, s.SourcePayloadID, run)
	return err == nil, err
}

func writeOrderVersion(ctx context.Context, tx pgx.Tx, seller, run, id int64, o OrderObservation, location *int64, hash string, at time.Time) (bool, error) {
	var latest int64
	var old string
	err := tx.QueryRow(ctx, `SELECT version_no,state_hash FROM supply_order_versions WHERE supply_order_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latest, &old)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if old == hash {
		return false, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO supply_order_versions(seller_account_id,supply_order_id,version_no,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,dropoff_location_id,timeslot_from,timeslot_to,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, seller, id, latest+1, o.ProviderState, o.CanonicalState, StateMappingVersion, o.StateMappingStatus, location, o.TimeslotFrom, o.TimeslotTo, jsonOrEmpty(o.ProviderFields), hash, at, o.SourcePayloadID, run)
	return err == nil, err
}
func writeSupplyVersion(ctx context.Context, tx pgx.Tx, seller, run, id int64, s SupplyObservation, location *int64, hash string, at time.Time) (bool, error) {
	var latest int64
	var old string
	err := tx.QueryRow(ctx, `SELECT version_no,state_hash FROM supply_versions WHERE supply_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latest, &old)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if old == hash {
		return false, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO supply_versions(seller_account_id,supply_id,version_no,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,storage_location_id,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, seller, id, latest+1, s.ProviderState, s.CanonicalState, StateMappingVersion, s.StateMappingStatus, location, jsonOrEmpty(s.ProviderFields), hash, at, s.SourcePayloadID, run)
	return err == nil, err
}
func upsertLocation(ctx context.Context, tx pgx.Tx, seller int64, providerID *int64, kind string, at time.Time) (*int64, error) {
	if providerID == nil {
		return nil, nil
	}
	key := kind + ":" + strconv.FormatInt(*providerID, 10)
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO inventory_locations(seller_account_id,provider_location_key,location_type,first_observed_at,last_observed_at)VALUES($1,$2,'WAREHOUSE',$3,$3)ON CONFLICT(seller_account_id,provider_location_key)DO UPDATE SET last_observed_at=GREATEST(inventory_locations.last_observed_at,EXCLUDED.last_observed_at)RETURNING id`, seller, key, at).Scan(&id)
	return &id, err
}
func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:SUPPLY:%d", seller)); err != nil {
		return false, err
	}
	var status, domain string
	if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); err != nil {
		return false, err
	}
	if domain != "SUPPLY" {
		return false, errors.New("expected SUPPLY domain")
	}
	var head int64
	err := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='SUPPLY'`, seller).Scan(&head)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if (status == "COMPLETED" || status == "PARTIAL") && err == nil && head >= run {
		return true, nil
	}
	if status != "RUNNING" {
		return false, fmt.Errorf("SUPPLY domain must be RUNNING, got %s", status)
	}
	return false, nil
}

var _ = json.RawMessage{}
