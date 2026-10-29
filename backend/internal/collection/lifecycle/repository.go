package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
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
	return &Repository{pool: pool, now: func() time.Time { return time.Now().UTC() }}
}
func NewRepositoryWithClock(pool *pgxpool.Pool, now func() time.Time) *Repository {
	return &Repository{pool: pool, now: now}
}

type SupersededRunError struct {
	Domain        string
	RunID, HeadID int64
}

func (e *SupersededRunError) Error() string {
	return fmt.Sprintf("%s domain run %d superseded by %d", e.Domain, e.RunID, e.HeadID)
}

func (r *Repository) FinalizePostings(ctx context.Context, seller, run int64, result *PostingsResult) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("postings finalization requires repository and result")
	}
	var superseded int64
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		stale, already, head, err := lockDomain(ctx, tx, seller, run, "POSTINGS")
		if err != nil {
			return err
		}
		if already {
			return nil
		}
		if stale {
			superseded = head
			return cancelSuperseded(ctx, tx, seller, run, head)
		}
		accept := r.now().UTC()
		limited := false
		for _, c := range result.postings {
			orderID := int64(0)
			var err error
			if c.OrderID > 0 {
				var created bool
				orderID, created, err = upsertOrder(ctx, tx, seller, c, accept)
				if err != nil {
					return err
				}
				if created {
					result.OrdersCreated++
				} else {
					result.OrdersReused++
				}
			} else {
				limited = true
			}
			postingID, newPosting, err := upsertPosting(ctx, tx, seller, orderID, c, accept)
			if err != nil {
				return err
			}
			if newPosting {
				result.PostingIdentitiesCreated++
			}
			if c.Scheme == "UNKNOWN" {
				limited = true
			}
			var previousHash, previousStatus string
			var previousObserved *time.Time
			var latestVersion int64
			err = tx.QueryRow(ctx, `SELECT version_no,state_hash,provider_status,observed_at FROM sales_posting_versions WHERE canonical_posting_id=$1 ORDER BY version_no DESC LIMIT 1`, postingID).Scan(&latestVersion, &previousHash, &previousStatus, &previousObserved)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil && previousHash == c.StateHash {
				result.VersionsReused++
				continue
			}
			var versionID int64
			orderResolution := "UNRESOLVED"
			if orderID > 0 {
				orderResolution = "RESOLVED"
			}
			err = tx.QueryRow(ctx, `INSERT INTO sales_posting_versions(seller_account_id,canonical_posting_id,version_no,collection_domain_run_id,source_payload_id,source_received_at,observed_at,provider_status,provider_substatus,provider_created_at,in_process_at,delivering_date,shipment_date,cancel_reason_id,cancel_reason,cancellation_type,cancellation_initiator,cancelled_after_ship,warehouse_id,provider_id,state_hash,canonical_order_id,provider_order_number,fulfillment_scheme,order_resolution_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12,$13,$14,NULLIF($15,''),NULLIF($16,''),NULLIF($17,''),$18,$19,$20,$21,$22,NULLIF($23,''),$24,$25) RETURNING id`, seller, postingID, latestVersion+1, run, c.PayloadID, c.SourceReceivedAt, accept, c.Status, c.Substatus, c.ProviderCreatedAt, c.InProcessAt, dateValue(c.DeliveringDate), dateValue(c.ShipmentDate), c.CancelReasonID, c.CancelReason, c.CancellationType, c.CancellationInitiator, c.CancelledAfterShip, c.WarehouseID, c.ProviderID, c.StateHash, nullablePositive(orderID), c.OrderNumber, c.Scheme, orderResolution).Scan(&versionID)
			if err != nil {
				return err
			}
			result.VersionsWritten++
			for i, p := range c.Products {
				var skuID *int64
				var id int64
				if p.SourceSKU > 0 {
					if e := tx.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2`, seller, p.SourceSKU).Scan(&id); e == nil {
						skuID = &id
						result.ResolvedSKUs++
					} else if !errors.Is(e, pgx.ErrNoRows) {
						return e
					} else {
						result.UnresolvedSKUs++
						limited = true
					}
				} else {
					result.UnresolvedSKUs++
					limited = true
				}
				resolution := "UNRESOLVED"
				if skuID != nil {
					resolution = "RESOLVED"
				}
				_, e := tx.Exec(ctx, `INSERT INTO sales_posting_products(seller_account_id,posting_version_id,line_no,canonical_sku_id,source_sku,sku_resolution_status,offer_id,quantity,unit_price,currency_code,product_name,source_payload_id) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,NULLIF($11,''),$12)`, seller, versionID, i+1, skuID, nullablePositive(p.SourceSKU), resolution, p.OfferID, p.Quantity, p.UnitPrice.String(), p.Currency, p.Name, p.PayloadID)
				if e != nil {
					return e
				}
				result.ProductLines++
			}
			if newPosting {
				if err = insertPostingCreated(ctx, tx, seller, orderID, postingID, versionID, c, run, accept); err != nil {
					return err
				}
				result.Events++
				if event := statusEvent(c.Status); event != "" {
					wrote, e := insertInitialStatusEvent(ctx, tx, seller, orderID, postingID, versionID, event, c, run, accept)
					if e != nil {
						return e
					}
					if wrote {
						result.Events++
					}
				}
			} else if previousStatus != c.Status {
				wrote, e := insertStatusEvent(ctx, tx, seller, orderID, postingID, versionID, latestVersion, previousHash, c, previousStatus, previousObserved, run, accept)
				if e != nil {
					return e
				}
				if wrote {
					result.Events++
				}
			}
			if c.ParentPostingNumber != "" {
				wrote, unresolved, e := upsertSplit(ctx, tx, seller, postingID, c, run, accept)
				if e != nil {
					return e
				}
				if wrote {
					result.Relations++
					result.Events++
				}
				if unresolved {
					limited = true
				}
			}
			if e := resolveWaitingParents(ctx, tx, seller, postingID, c.PostingNumber, run, c.PayloadID, accept); e != nil {
				return e
			}
		}
		status, complete := "COMPLETED", "COMPLETE"
		if limited || !streamFinal(result.FBO.Status) || !streamFinal(result.FBS.Status) {
			status, complete = "PARTIAL", "PARTIAL"
		}
		errorCode := ""
		if result.FBO.ErrorCode != "" {
			errorCode = result.FBO.ErrorCode
		}
		if result.FBS.ErrorCode != "" {
			errorCode = result.FBS.ErrorCode
		}
		from, to := postingCoverage(result.postings)
		records := result.VersionsWritten + result.ProductLines + result.Relations + result.Events
		cmd, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,coverage_from=$4,coverage_to=$5,records_received=$6,records_written=$7,pages_received=$8,completeness_status=$9,finished_at=$10,error_code=NULLIF($11,''),error_message=CASE WHEN $11='' THEN NULL ELSE 'one posting stream completed with an explicit limitation' END,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, from, to, len(result.postings), records, result.FBO.Pages+result.FBS.Pages, complete, accept, errorCode)
		if e != nil {
			return e
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale postings domain transition")
		}
		return advanceHead(ctx, tx, seller, "POSTINGS", run, accept)
	})
	if err != nil {
		return err
	}
	if superseded > 0 {
		return &SupersededRunError{"POSTINGS", run, superseded}
	}
	return nil
}

func (r *Repository) FinalizeReturns(ctx context.Context, seller, run int64, result *ReturnsResult) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("returns finalization requires repository and result")
	}
	var superseded int64
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		stale, already, head, err := lockDomain(ctx, tx, seller, run, "RETURNS")
		if err != nil {
			return err
		}
		if already {
			return nil
		}
		if stale {
			superseded = head
			return cancelSuperseded(ctx, tx, seller, run, head)
		}
		accept := r.now().UTC()
		limited := false
		for _, c := range result.returns {
			var returnID int64
			var created bool
			err = tx.QueryRow(ctx, `INSERT INTO sales_returns(seller_account_id,provider_return_id,first_observed_at,last_observed_at) VALUES($1,$2,$3,$3) ON CONFLICT(seller_account_id,provider_return_id) DO UPDATE SET last_observed_at=EXCLUDED.last_observed_at,updated_at=NOW() RETURNING id,(xmax=0)`, seller, c.ProviderReturnID, accept).Scan(&returnID, &created)
			if err != nil {
				return err
			}
			if created {
				result.ReturnsCreated++
			}
			orderID, orderStatus, err := resolveOrder(ctx, tx, seller, c.SourceOrderID)
			if err != nil {
				return err
			}
			postingID, postingStatus, err := resolvePosting(ctx, tx, seller, c.SourcePostingNumber)
			if err != nil {
				return err
			}
			skuID, skuStatus, err := resolveSKU(ctx, tx, seller, c.SourceSKU)
			if err != nil {
				return err
			}
			for _, s := range []string{orderStatus, postingStatus, skuStatus} {
				if s == "RESOLVED" {
					continue
				}
				if s == "UNRESOLVED" {
					result.UnresolvedLinks++
					limited = true
				}
			}
			if orderStatus == "RESOLVED" {
				result.ResolvedOrders++
			}
			if postingStatus == "RESOLVED" {
				result.ResolvedPostings++
			}
			if skuStatus == "RESOLVED" {
				result.ResolvedSKUs++
			}
			var latest int64
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM sales_return_events WHERE sales_return_id=$1 ORDER BY version_no DESC LIMIT 1`, returnID).Scan(&latest, &latestHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil && latestHash == c.StateHash {
				result.VersionsReused++
				continue
			}
			var eventID int64
			err = tx.QueryRow(ctx, `INSERT INTO sales_return_events(sales_return_id,seller_account_id,version_no,canonical_order_id,canonical_posting_id,canonical_sku_id,source_order_id,source_posting_number,source_sku,order_resolution_status,posting_resolution_status,sku_resolution_status,provider_type,provider_schema,return_reason_name,provider_source_id,quantity,unit_price,currency_code,provider_return_date,provider_visual_status,provider_visual_status_sys_name,provider_visual_change_at,technical_return_moment,final_moment,cancelled_with_compensation_moment,compensation_status,state_hash,source_received_at,observed_at,source_payload_id,collection_domain_run_id) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),$16,$17,$18,$19,$20,NULLIF($21,''),NULLIF($22,''),$23,$24,$25,$26,NULLIF($27,''),$28,$29,$30,$31,$32) RETURNING id`, returnID, seller, latest+1, orderID, postingID, skuID, nullablePositive(c.SourceOrderID), c.SourcePostingNumber, nullablePositive(c.SourceSKU), orderStatus, postingStatus, skuStatus, c.ProviderType, c.ProviderSchema, c.Reason, nullablePositive(c.SourceID), c.Quantity, c.UnitPrice.String(), c.Currency, c.ProviderReturnDate, c.VisualStatus, c.VisualStatusSysName, c.VisualChangeAt, c.TechnicalReturnMoment, c.FinalMoment, c.CancelledWithCompensationMoment, c.CompensationStatus, c.StateHash, c.SourceReceivedAt, accept, c.PayloadID, run).Scan(&eventID)
			if err != nil {
				return err
			}
			result.VersionsWritten++
			if latest == 0 {
				dedupe := hash(fmt.Sprint(seller), "RETURN_CREATED", fmt.Sprint(returnID), fmt.Sprint(eventID))
				_, err = tx.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_order_id,canonical_posting_id,canonical_sku_id,canonical_return_id,event_type,quantity,unit_amount,amount,currency_code,effective_at,business_date,time_quality,source_type,source_return_event_id,source_payload_id,collection_domain_run_id,dedupe_key,observed_at) VALUES($1,$2,$3,$4,$5,'RETURN_CREATED',$6,$7,$8,$9,NULL,NULL,'UNKNOWN','RETURN_OBSERVATION',$10,$11,$12,$13,$14) ON CONFLICT(seller_account_id,dedupe_key) DO NOTHING`, seller, orderID, postingID, skuID, returnID, c.Quantity, c.UnitPrice.String(), multiplyAmount(c.UnitPrice.String(), c.Quantity), c.Currency, eventID, c.PayloadID, run, dedupe, accept)
				if err != nil {
					return err
				}
				result.Events++
			}
		}
		status, complete := "COMPLETED", "COMPLETE"
		if limited {
			status, complete = "PARTIAL", "PARTIAL"
		}
		from, to := returnCoverage(result.returns)
		records := result.VersionsWritten + result.Events
		cmd, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,coverage_from=$4,coverage_to=$5,records_received=$6,records_written=$7,pages_received=$8,completeness_status=$9,finished_at=$10,error_code=NULL,error_message=NULL,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, from, to, len(result.returns), records, result.Stream.Pages, complete, accept)
		if e != nil {
			return e
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale returns domain transition")
		}
		return advanceHead(ctx, tx, seller, "RETURNS", run, accept)
	})
	if err != nil {
		return err
	}
	if superseded > 0 {
		return &SupersededRunError{"RETURNS", run, superseded}
	}
	return nil
}

func lockDomain(ctx context.Context, tx pgx.Tx, seller, run int64, domain string) (bool, bool, int64, error) {
	if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical-lifecycle:%s:%d", domain, seller)); e != nil {
		return false, false, 0, e
	}
	var status, actual string
	if e := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &actual); e != nil {
		return false, false, 0, e
	}
	if actual != domain {
		return false, false, 0, fmt.Errorf("expected %s domain", domain)
	}
	var head int64
	e := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain=$2`, seller, domain).Scan(&head)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return false, false, 0, e
	}
	if (status == "COMPLETED" || status == "PARTIAL") && e == nil && head >= run {
		return false, true, head, nil
	}
	if status != "RUNNING" {
		return false, false, 0, fmt.Errorf("%s domain must be RUNNING, got %s", domain, status)
	}
	return e == nil && head > run, false, head, nil
}
func cancelSuperseded(ctx context.Context, tx pgx.Tx, seller, run, head int64) error {
	_, e := tx.Exec(ctx, `UPDATE collection_domain_runs SET status='CANCELLED',finished_at=NOW(),error_code='SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN',error_message=$3,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, run, seller, fmt.Sprintf("newer domain run %d finalized", head))
	return e
}
func advanceHead(ctx context.Context, tx pgx.Tx, seller int64, domain string, run int64, at time.Time) error {
	_, e := tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at) VALUES($1,$2,$3,$4) ON CONFLICT(seller_account_id,domain) DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW() WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, domain, run, at)
	return e
}
func upsertOrder(ctx context.Context, tx pgx.Tx, seller int64, c PostingCandidate, at time.Time) (int64, bool, error) {
	var id int64
	var number *string
	err := tx.QueryRow(ctx, `SELECT id,order_number FROM sales_orders WHERE seller_account_id=$1 AND ozon_order_id=$2 FOR UPDATE`, seller, c.OrderID).Scan(&id, &number)
	if err == nil {
		if number != nil && *number != "" && c.OrderNumber != "" && *number != c.OrderNumber {
			return 0, false, fmt.Errorf("%w: order_id=%d", ErrOrderIdentityConflict, c.OrderID)
		}
		_, err = tx.Exec(ctx, `UPDATE sales_orders SET order_number=COALESCE(order_number,NULLIF($3,'')),last_observed_at=$4,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, id, seller, c.OrderNumber, at)
		return id, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO sales_orders(seller_account_id,ozon_order_id,order_number,first_observed_at,last_observed_at) VALUES($1,$2,NULLIF($3,''),$4,$4) RETURNING id`, seller, c.OrderID, c.OrderNumber, at).Scan(&id)
	return id, true, err
}
func upsertPosting(ctx context.Context, tx pgx.Tx, seller, orderID int64, c PostingCandidate, at time.Time) (int64, bool, error) {
	var id, existingOrder int64
	var scheme string
	err := tx.QueryRow(ctx, `SELECT id,COALESCE(canonical_order_id,0),fulfillment_scheme FROM sales_postings WHERE seller_account_id=$1 AND posting_number=$2 FOR UPDATE`, seller, c.PostingNumber).Scan(&id, &existingOrder, &scheme)
	if err == nil {
		if existingOrder > 0 && orderID > 0 && existingOrder != orderID {
			return 0, false, fmt.Errorf("%w: posting order changed", ErrOrderIdentityConflict)
		}
		newScheme := scheme
		if c.Scheme != "UNKNOWN" {
			newScheme = c.Scheme
		}
		_, err = tx.Exec(ctx, `UPDATE sales_postings SET canonical_order_id=COALESCE($3,canonical_order_id),order_resolution_status=CASE WHEN COALESCE($3,canonical_order_id)IS NULL THEN 'UNRESOLVED' ELSE 'RESOLVED' END,fulfillment_scheme=$4,last_observed_at=$5,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, id, seller, nullablePositive(orderID), newScheme, at)
		return id, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	resolution := "UNRESOLVED"
	if orderID > 0 {
		resolution = "RESOLVED"
	}
	err = tx.QueryRow(ctx, `INSERT INTO sales_postings(seller_account_id,canonical_order_id,posting_number,fulfillment_scheme,order_resolution_status,first_observed_at,last_observed_at) VALUES($1,$2,$3,$4,$5,$6,$6) RETURNING id`, seller, nullablePositive(orderID), c.PostingNumber, c.Scheme, resolution, at).Scan(&id)
	return id, true, err
}
func insertPostingCreated(ctx context.Context, tx pgx.Tx, seller, order, posting, version int64, c PostingCandidate, run int64, at time.Time) error {
	quality := "UNKNOWN"
	var effective any
	var business any
	if c.ProviderCreatedAt != nil {
		quality = "PROVIDER_EXACT"
		effective = *c.ProviderCreatedAt
		d := businessDay(*c.ProviderCreatedAt)
		business = d
	}
	dedupe := hash(fmt.Sprint(seller), "POSTING_CREATED", fmt.Sprint(posting), fmt.Sprint(version))
	_, e := tx.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_order_id,canonical_posting_id,event_type,effective_at,business_date,time_quality,source_type,source_posting_version_id,source_payload_id,collection_domain_run_id,dedupe_key,observed_at) VALUES($1,$2,$3,'POSTING_CREATED',$4,$5,$6,'POSTING_VERSION',$7,$8,$9,$10,$11) ON CONFLICT(seller_account_id,dedupe_key) DO NOTHING`, seller, nullablePositive(order), posting, effective, business, quality, version, c.PayloadID, run, dedupe, at)
	return e
}
func insertStatusEvent(ctx context.Context, tx pgx.Tx, seller, order, posting, newVersion, previousVersion int64, previousHash string, c PostingCandidate, previousStatus string, previousObserved *time.Time, run int64, at time.Time) (bool, error) {
	event := statusEvent(c.Status)
	if event == "" {
		return false, nil
	}
	quality := "OBSERVED_APPROX"
	var from any
	if previousObserved != nil {
		from = *previousObserved
	}
	business := approxBusinessDate(previousObserved, at)
	if previousVersion == 0 {
		quality = "UNKNOWN"
		from = nil
		business = nil
	}
	dedupe := hash(fmt.Sprint(seller), event, fmt.Sprint(posting), fmt.Sprint(previousVersion), fmt.Sprint(newVersion), previousHash, c.StateHash)
	cmd, e := tx.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_order_id,canonical_posting_id,event_type,effective_at,effective_window_from_at,effective_window_to_at,business_date,time_quality,source_type,source_posting_version_id,source_payload_id,collection_domain_run_id,dedupe_key,observed_at) VALUES($1,$2,$3,$4,NULL,$5,$6,$7,$8,'POSTING_VERSION',$9,$10,$11,$12,$13) ON CONFLICT(seller_account_id,dedupe_key) DO NOTHING`, seller, nullablePositive(order), posting, event, from, at, business, quality, newVersion, c.PayloadID, run, dedupe, at)
	return cmd.RowsAffected() > 0, e
}

func insertInitialStatusEvent(ctx context.Context, tx pgx.Tx, seller, order, posting, version int64, event string, c PostingCandidate, run int64, at time.Time) (bool, error) {
	dedupe := hash(fmt.Sprint(seller), event, fmt.Sprint(posting), "initial", fmt.Sprint(version))
	cmd, err := tx.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_order_id,canonical_posting_id,event_type,effective_at,business_date,time_quality,source_type,source_posting_version_id,source_payload_id,collection_domain_run_id,dedupe_key,observed_at) VALUES($1,$2,$3,$4,NULL,NULL,'UNKNOWN','POSTING_VERSION',$5,$6,$7,$8,$9) ON CONFLICT(seller_account_id,dedupe_key) DO NOTHING`, seller, nullablePositive(order), posting, event, version, c.PayloadID, run, dedupe, at)
	return cmd.RowsAffected() > 0, err
}
func statusEvent(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "cancelled", "canceled":
		return "POSTING_CANCELLED_FULL"
	case "delivered":
		return "POSTING_DELIVERED"
	case "delivering":
		return "POSTING_DELIVERING"
	}
	return ""
}
func upsertSplit(ctx context.Context, tx pgx.Tx, seller, child int64, c PostingCandidate, run int64, at time.Time) (bool, bool, error) {
	if c.ParentPostingNumber == c.PostingNumber {
		return false, false, ErrPostingLineageCycle
	}
	var parent int64
	err := tx.QueryRow(ctx, `SELECT id FROM sales_postings WHERE seller_account_id=$1 AND posting_number=$2`, seller, c.ParentPostingNumber).Scan(&parent)
	unresolved := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !unresolved {
		return false, false, err
	}
	if !unresolved {
		cycle, e := wouldCycle(ctx, tx, seller, parent, child)
		if e != nil {
			return false, false, e
		}
		if cycle {
			return false, false, ErrPostingLineageCycle
		}
	}
	var relationID int64
	err = tx.QueryRow(ctx, `INSERT INTO sales_posting_relations(seller_account_id,parent_posting_id,parent_posting_number,child_posting_id,relation_type,resolution_status,first_observed_at,source_payload_id,collection_domain_run_id) VALUES($1,$2,$3,$4,'SPLIT',$5,$6,$7,$8) ON CONFLICT(seller_account_id,parent_posting_number,child_posting_id,relation_type) DO UPDATE SET parent_posting_id=COALESCE(sales_posting_relations.parent_posting_id,EXCLUDED.parent_posting_id),resolution_status=CASE WHEN EXCLUDED.parent_posting_id IS NULL THEN sales_posting_relations.resolution_status ELSE 'RESOLVED' END,updated_at=NOW() RETURNING id`, seller, nullablePositive(parent), c.ParentPostingNumber, child, map[bool]string{true: "UNRESOLVED", false: "RESOLVED"}[unresolved], at, c.PayloadID, run).Scan(&relationID)
	if err != nil {
		return false, false, err
	}
	if err = appendRelationVersion(ctx, tx, seller, relationID, nullablePositive(parent), map[bool]string{true: "UNRESOLVED", false: "RESOLVED"}[unresolved], run, c.PayloadID, at); err != nil {
		return false, false, err
	}
	dedupe := hash(fmt.Sprint(seller), "POSTING_SPLIT", fmt.Sprint(relationID))
	cmd, e := tx.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_posting_id,event_type,time_quality,source_type,source_payload_id,collection_domain_run_id,dedupe_key,observed_at) VALUES($1,$2,'POSTING_SPLIT','UNKNOWN','POSTING_RELATION',$3,$4,$5,$6) ON CONFLICT(seller_account_id,dedupe_key) DO NOTHING`, seller, child, c.PayloadID, run, dedupe, at)
	return cmd.RowsAffected() > 0, unresolved, e
}
func wouldCycle(ctx context.Context, tx pgx.Tx, seller, parent, child int64) (bool, error) {
	var cycle bool
	e := tx.QueryRow(ctx, `WITH RECURSIVE descendants(id) AS (SELECT child_posting_id FROM sales_posting_relations WHERE seller_account_id=$1 AND parent_posting_id=$2 UNION SELECT r.child_posting_id FROM sales_posting_relations r JOIN descendants d ON r.parent_posting_id=d.id WHERE r.seller_account_id=$1) SELECT EXISTS(SELECT 1 FROM descendants WHERE id=$3)`, seller, child, parent).Scan(&cycle)
	return cycle, e
}
func resolveWaitingParents(ctx context.Context, tx pgx.Tx, seller, parent int64, number string, run, payload int64, at time.Time) error {
	rows, e := tx.Query(ctx, `SELECT id,child_posting_id FROM sales_posting_relations WHERE seller_account_id=$1 AND parent_posting_number=$2 AND parent_posting_id IS NULL FOR UPDATE`, seller, number)
	if e != nil {
		return e
	}
	defer rows.Close()
	type item struct{ id, child int64 }
	var items []item
	for rows.Next() {
		var i item
		if e = rows.Scan(&i.id, &i.child); e != nil {
			return e
		}
		items = append(items, i)
	}
	for _, i := range items {
		cycle, e := wouldCycle(ctx, tx, seller, parent, i.child)
		if e != nil {
			return e
		}
		if cycle {
			return ErrPostingLineageCycle
		}
		if _, e = tx.Exec(ctx, `UPDATE sales_posting_relations SET parent_posting_id=$2,resolution_status='RESOLVED',updated_at=NOW() WHERE id=$1`, i.id, parent); e != nil {
			return e
		}
		if e = appendRelationVersion(ctx, tx, seller, i.id, parent, "RESOLVED", run, payload, at); e != nil {
			return e
		}
	}
	return nil
}

func appendRelationVersion(ctx context.Context, tx pgx.Tx, seller, relation int64, parent any, status string, run, payload int64, at time.Time) error {
	var revision int64
	var lastParent *int64
	var lastStatus string
	err := tx.QueryRow(ctx, `SELECT revision_no,parent_posting_id,resolution_status FROM sales_posting_relation_versions WHERE posting_relation_id=$1 ORDER BY revision_no DESC LIMIT 1`, relation).Scan(&revision, &lastParent, &lastStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var parentID *int64
	switch v := parent.(type) {
	case int64:
		parentID = &v
	case *int64:
		parentID = v
	}
	if err == nil && lastStatus == status && ((lastParent == nil && parentID == nil) || (lastParent != nil && parentID != nil && *lastParent == *parentID)) {
		return nil
	}
	state := hash(fmt.Sprint(relation), fmt.Sprint(parentID), status)
	_, err = tx.Exec(ctx, `INSERT INTO sales_posting_relation_versions(seller_account_id,posting_relation_id,revision_no,parent_posting_id,resolution_status,observed_at,source_payload_id,collection_domain_run_id,state_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, seller, relation, revision+1, parentID, status, at, payload, run, state)
	return err
}

func streamFinal(status string) bool { return status == "COMPLETE" || status == "EMPTY_VALID" }
func resolveOrder(ctx context.Context, tx pgx.Tx, seller, source int64) (*int64, string, error) {
	if source <= 0 {
		return nil, "NOT_APPLICABLE", nil
	}
	var id int64
	e := tx.QueryRow(ctx, `SELECT id FROM sales_orders WHERE seller_account_id=$1 AND ozon_order_id=$2`, seller, source).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, "UNRESOLVED", nil
	}
	return &id, "RESOLVED", e
}
func resolvePosting(ctx context.Context, tx pgx.Tx, seller int64, source string) (*int64, string, error) {
	if strings.TrimSpace(source) == "" {
		return nil, "NOT_APPLICABLE", nil
	}
	var id int64
	e := tx.QueryRow(ctx, `SELECT id FROM sales_postings WHERE seller_account_id=$1 AND posting_number=$2`, seller, source).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, "UNRESOLVED", nil
	}
	return &id, "RESOLVED", e
}
func resolveSKU(ctx context.Context, tx pgx.Tx, seller, source int64) (*int64, string, error) {
	if source <= 0 {
		return nil, "UNRESOLVED", nil
	}
	var id int64
	e := tx.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2`, seller, source).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, "UNRESOLVED", nil
	}
	return &id, "RESOLVED", e
}
func postingCoverage(rows []PostingCandidate) (*time.Time, *time.Time) {
	var from, to *time.Time
	for _, r := range rows {
		if r.ProviderCreatedAt == nil {
			continue
		}
		d := businessDay(*r.ProviderCreatedAt)
		if from == nil || d.Before(*from) {
			v := d
			from = &v
		}
		if to == nil || d.After(*to) {
			v := d
			to = &v
		}
	}
	return from, to
}
func returnCoverage(rows []ReturnCandidate) (*time.Time, *time.Time) {
	var from, to *time.Time
	for _, r := range rows {
		if r.VisualChangeAt == nil {
			continue
		}
		d := businessDay(*r.VisualChangeAt)
		if from == nil || d.Before(*from) {
			v := d
			from = &v
		}
		if to == nil || d.After(*to) {
			v := d
			to = &v
		}
	}
	return from, to
}
func businessDay(v time.Time) time.Time {
	loc, _ := time.LoadLocation("Europe/Moscow")
	x := v.In(loc)
	return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, time.UTC)
}
func approxBusinessDate(from *time.Time, to time.Time) any {
	if from == nil {
		return nil
	}
	a, b := businessDay(*from), businessDay(to)
	if a.Equal(b) {
		return a
	}
	return nil
}
func dateValue(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullablePositive(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func multiplyAmount(amount string, quantity int64) string {
	v, ok := new(big.Rat).SetString(amount)
	if !ok {
		return "0"
	}
	v.Mul(v, new(big.Rat).SetInt64(quantity))
	return v.FloatString(6)
}
