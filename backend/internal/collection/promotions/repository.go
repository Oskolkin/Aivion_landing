package promotions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ResolvePromotionSKU(ctx context.Context, seller int64, productID, sourceSKU *int64, offerID string, cutoff time.Time) (*int64, string, error) {
	rows, err := r.pool.Query(ctx, `WITH candidates AS(
 SELECT c.id FROM canonical_skus c WHERE c.seller_account_id=$1 AND c.first_observed_at<=$2 AND $3::bigint IS NOT NULL AND c.ozon_sku=$3
 UNION
 SELECT c.id FROM canonical_skus c JOIN LATERAL(
   SELECT v.provider_product_id,v.offer_id FROM canonical_sku_versions v
   WHERE v.canonical_sku_id=c.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1
 )v ON true
 WHERE c.seller_account_id=$1 AND c.first_observed_at<=$2
   AND (($4::bigint IS NOT NULL AND v.provider_product_id=$4) OR (NULLIF($5,'') IS NOT NULL AND v.offer_id=$5))
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
		return errors.New("promotions finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, err := lockRun(ctx, tx, seller, run)
		if err != nil || already {
			return err
		}
		at := result.StartedAt.UTC()
		if at.IsZero() {
			return errors.New("promotions observation time is required")
		}
		ids := map[PromotionKey]int64{}
		for _, d := range result.Definitions {
			key := PromotionKey{Origin: d.Origin, ProviderActionID: d.ProviderActionID}
			var id int64
			var created bool
			err = tx.QueryRow(ctx, `INSERT INTO promotions(seller_account_id,promotion_origin,provider_action_id,first_observed_at,last_observed_at)VALUES($1,$2,$3,$4,$4)ON CONFLICT(seller_account_id,promotion_origin,provider_action_id)DO UPDATE SET last_observed_at=GREATEST(promotions.last_observed_at,EXCLUDED.last_observed_at)RETURNING id,(xmax=0)`, seller, d.Origin, d.ProviderActionID, at).Scan(&id, &created)
			if err != nil {
				return err
			}
			ids[key] = id
			if created {
				result.PromotionsCreated++
			}
			hash := definitionHash(d)
			var latest int64
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM promotion_versions WHERE promotion_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latest, &latestHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash {
				result.VersionsReused++
				continue
			}
			var versionID int64
			err = tx.QueryRow(ctx, `INSERT INTO promotion_versions(seller_account_id,promotion_id,version_no,title,provider_state,starts_at,ends_at,discount_type,discount_value,currency_code,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,NULLIF($8,''),$9,NULLIF($10,''),$11,$12,$13,$14,$15)RETURNING id`, seller, id, latest+1, d.Title, d.ProviderState, d.StartsAt, d.EndsAt, d.DiscountType, decimalString(d.DiscountValue), d.CurrencyCode, jsonOrEmpty(d.ProviderFields), hash, at, d.SourcePayloadID, run).Scan(&versionID)
			if err != nil {
				return err
			}
			for i, component := range d.RuleComponents {
				if _, err = tx.Exec(ctx, `INSERT INTO promotion_rule_components(seller_account_id,promotion_version_id,component_ordinal,component_type,component_json)VALUES($1,$2,$3,'PROVIDER_RULE',$4)`, seller, versionID, i+1, jsonOrEmpty(component)); err != nil {
					return err
				}
			}
			result.VersionsWritten++
		}
		complete := map[PromotionKey]bool{}
		for _, key := range result.CompleteParticipantKeys {
			complete[key] = true
		}
		seen := map[int64]map[string]bool{}
		for key := range complete {
			if id := ids[key]; id > 0 {
				seen[id] = map[string]bool{}
			}
		}
		for _, member := range result.Participants {
			promotionID := ids[PromotionKey{Origin: member.Origin, ProviderActionID: member.ProviderActionID}]
			if promotionID == 0 {
				continue
			}
			identity := skuIdentity(member)
			if seen[promotionID] != nil {
				seen[promotionID][identity] = true
			}
			hash := membershipHash(member)
			var openID int64
			var openHash string
			err = tx.QueryRow(ctx, `SELECT id,state_hash FROM promotion_sku_memberships WHERE promotion_id=$1 AND COALESCE(source_sku,0)=COALESCE($2,0)AND COALESCE(source_product_id,0)=COALESCE($3,0)AND COALESCE(source_offer_id,'')=COALESCE($4,'')AND known_to IS NULL`, promotionID, member.SourceSKU, member.SourceProductID, member.SourceOfferID).Scan(&openID, &openHash)
			if err == nil && openHash == hash {
				continue
			}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if openID > 0 {
				if _, err = tx.Exec(ctx, `UPDATE promotion_sku_memberships SET known_to=$2 WHERE id=$1`, openID, at); err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, `INSERT INTO promotion_sku_memberships(seller_account_id,promotion_id,canonical_sku_id,source_product_id,source_offer_id,source_sku,sku_resolution_status,action_price,action_stock,known_from,time_quality,state_hash,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,'OBSERVED_APPROX',$11,$12,$13)`, seller, promotionID, member.CanonicalSKUID, member.SourceProductID, member.SourceOfferID, member.SourceSKU, member.SKUResolutionStatus, decimalString(member.ActionPrice), member.ActionStock, at, hash, member.SourcePayloadID, run)
			if err != nil {
				return err
			}
			result.MembershipIntervalsWritten++
		}
		// Absence closes intervals only for promotions whose participant list was complete.
		// Collect close IDs first — pgx forbids Exec while a Query result is still open on the tx.
		for promotionID, identities := range seen {
			rows, queryErr := tx.Query(ctx, `SELECT id,COALESCE(source_sku,0),COALESCE(source_product_id,0),COALESCE(source_offer_id,'')FROM promotion_sku_memberships WHERE promotion_id=$1 AND known_to IS NULL`, promotionID)
			if queryErr != nil {
				return queryErr
			}
			var closeIDs []int64
			for rows.Next() {
				var id, sku, product int64
				var offer string
				if err = rows.Scan(&id, &sku, &product, &offer); err != nil {
					rows.Close()
					return err
				}
				identity := fmt.Sprintf("%d|%d|%s", sku, product, offer)
				if !identities[identity] {
					closeIDs = append(closeIDs, id)
				}
			}
			if err = rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()
			for _, id := range closeIDs {
				if _, err = tx.Exec(ctx, `UPDATE promotion_sku_memberships SET known_to=$2 WHERE id=$1`, id, at); err != nil {
					return err
				}
				result.MembershipIntervalsClosed++
			}
		}
		for _, snapshot := range result.Eligibility {
			promotionID := ids[PromotionKey{Origin: snapshot.Origin, ProviderActionID: snapshot.ProviderActionID}]
			if promotionID == 0 {
				continue
			}
			var snapshotID int64
			err = tx.QueryRow(ctx, `INSERT INTO promotion_eligibility_snapshot_runs(seller_account_id,promotion_id,collection_domain_run_id,observed_at,completeness_status,rows_received,source_payload_id)VALUES($1,$2,$3,$4,$5,$6,$7)ON CONFLICT(promotion_id,collection_domain_run_id)DO UPDATE SET completeness_status=EXCLUDED.completeness_status,rows_received=EXCLUDED.rows_received RETURNING id`, seller, promotionID, run, at, snapshot.CompletenessStatus, len(snapshot.Rows), snapshot.SourcePayloadID).Scan(&snapshotID)
			if err != nil {
				return err
			}
			for _, candidate := range snapshot.Rows {
				_, err = tx.Exec(ctx, `INSERT INTO promotion_sku_eligibility_snapshots(seller_account_id,snapshot_run_id,canonical_sku_id,source_product_id,source_offer_id,source_sku,sku_resolution_status,eligibility_state,action_price,action_stock,provider_fields_json,source_payload_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,'CANDIDATE',$8,$9,$10,$11)`, seller, snapshotID, candidate.CanonicalSKUID, candidate.SourceProductID, candidate.SourceOfferID, candidate.SourceSKU, candidate.SKUResolutionStatus, decimalString(candidate.ActionPrice), candidate.ActionStock, jsonOrEmpty(candidate.ProviderFields), candidate.SourcePayloadID)
				if err != nil {
					return err
				}
				result.EligibilityRows++
			}
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" {
			status = "PARTIAL"
		}
		cmd, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$5,pages_received=$6,completeness_status=$7,finished_at=$8,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, len(result.Definitions)+len(result.Participants)+result.EligibilityRows, result.VersionsWritten+result.MembershipIntervalsWritten+result.EligibilityRows, result.DefinitionRequests+result.ParticipantPages+result.CandidatePages, result.CompletenessStatus, result.CompletedAt)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale PROMOTIONS domain transition")
		}
		_, err = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'PROMOTIONS',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, result.CompletedAt)
		return err
	})
}

func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:PROMOTIONS:%d", seller)); err != nil {
		return false, err
	}
	var status, domain string
	if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); err != nil {
		return false, err
	}
	if domain != "PROMOTIONS" {
		return false, errors.New("expected PROMOTIONS domain")
	}
	var head int64
	err := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='PROMOTIONS'`, seller).Scan(&head)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if (status == "COMPLETED" || status == "PARTIAL") && err == nil && head >= run {
		return true, nil
	}
	if status != "RUNNING" {
		return false, fmt.Errorf("PROMOTIONS domain must be RUNNING, got %s", status)
	}
	return false, nil
}

func definitionHash(d DefinitionObservation) string {
	return factualapi.StableHash(d.Origin, strconv.FormatInt(d.ProviderActionID, 10), d.Title, d.ProviderState, optionalTime(d.StartsAt), optionalTime(d.EndsAt), d.DiscountType, optionalDecimal(d.DiscountValue), d.CurrencyCode, string(d.ProviderFields))
}
func membershipHash(x SKUObservation) string {
	return factualapi.StableHash(skuIdentity(x), optionalInt64(x.CanonicalSKUID), x.SKUResolutionStatus, optionalDecimal(x.ActionPrice), optionalInt64(x.ActionStock))
}
func skuIdentity(x SKUObservation) string {
	var sku, product int64
	if x.SourceSKU != nil {
		sku = *x.SourceSKU
	}
	if x.SourceProductID != nil {
		product = *x.SourceProductID
	}
	return fmt.Sprintf("%d|%d|%s", sku, product, x.SourceOfferID)
}
func optionalInt64(v *int64) string {
	if v == nil {
		return "NULL"
	}
	return strconv.FormatInt(*v, 10)
}
func optionalTime(v *time.Time) string {
	if v == nil {
		return "NULL"
	}
	return v.UTC().Format(time.RFC3339Nano)
}
func optionalDecimal(v *sales.Decimal) string {
	if v == nil {
		return "NULL"
	}
	return v.String()
}
func decimalString(v *sales.Decimal) any {
	if v == nil {
		return nil
	}
	return v.String()
}
func marshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
