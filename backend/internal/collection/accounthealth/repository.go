package accounthealth

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

func (r *Repository) ResolvePosting(ctx context.Context, seller int64, postingNumber string, cutoff time.Time) (*int64, string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM sales_postings WHERE seller_account_id=$1 AND posting_number=$2 AND first_observed_at<=$3 ORDER BY id`, seller, strings.TrimSpace(postingNumber), cutoff)
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
		return errors.New("account health finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, err := lockRun(ctx, tx, seller, run)
		if err != nil || already {
			return err
		}
		at := result.StartedAt.UTC()
		if at.IsZero() {
			return errors.New("account health observation time is required")
		}
		metricIDs := map[string]int64{}
		ensureMetric := func(key string, sourcePayload int64, definition *MetricObservation) (int64, error) {
			if id := metricIDs[key]; id != 0 {
				return id, nil
			}
			var id int64
			var created bool
			err := tx.QueryRow(ctx, `INSERT INTO account_health_metrics(seller_account_id,provider,provider_rating_key,first_observed_at,last_observed_at)VALUES($1,'OZON',$2,$3,$3)ON CONFLICT(seller_account_id,provider,provider_rating_key)DO UPDATE SET last_observed_at=GREATEST(account_health_metrics.last_observed_at,EXCLUDED.last_observed_at)RETURNING id,(xmax=0)`, seller, key, at).Scan(&id, &created)
			if err != nil {
				return 0, err
			}
			metricIDs[key] = id
			if created {
				result.MetricsCreated++
			}
			if definition == nil {
				definition = &MetricObservation{ProviderRatingKey: key, CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", SourcePayloadID: sourcePayload}
			}
			hash := metricHash(*definition)
			var version int64
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM account_health_metric_versions WHERE account_health_metric_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&version, &latestHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return 0, err
			}
			if latestHash != hash {
				_, err = tx.Exec(ctx, `INSERT INTO account_health_metric_versions(seller_account_id,account_health_metric_id,version_no,canonical_metric_key,mapping_status,provider_group_name,provider_display_name,value_type,rating_direction,canonical_family,known_calculation_window,history_supported,value_unit,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,NULLIF($13,''),$14,$15,$16,$17,$18)`, seller, id, version+1, definition.CanonicalMetricKey, definition.MappingStatus, definition.GroupName, definition.DisplayName, definition.ValueType, definition.RatingDirection, definition.CanonicalFamily, definition.KnownCalculationWindow, definition.HistorySupported, definition.Unit, jsonOrEmpty(definition.ProviderFields), hash, at, sourcePayload, run)
				if err != nil {
					return 0, err
				}
				result.MetricVersionsWritten++
			}
			return id, nil
		}

		var snapshotID int64
		if result.SummaryPayloadID > 0 {
			err = tx.QueryRow(ctx, `INSERT INTO account_health_snapshot_runs(seller_account_id,collection_domain_run_id,observed_at,completeness_status,rows_received,source_payload_id)VALUES($1,$2,$3,$4,$5,$6)RETURNING id`, seller, run, at, result.CompletenessStatus, len(result.Metrics), result.SummaryPayloadID).Scan(&snapshotID)
			if err != nil {
				return err
			}
			result.SnapshotsWritten++
		}
		for i := range result.Metrics {
			metric := &result.Metrics[i]
			id, metricErr := ensureMetric(metric.ProviderRatingKey, metric.SourcePayloadID, metric)
			if metricErr != nil {
				return metricErr
			}
			if snapshotID > 0 {
				_, err = tx.Exec(ctx, `INSERT INTO account_rating_snapshots(seller_account_id,snapshot_run_id,account_health_metric_id,provider_value,provider_previous_value,provider_status,provider_change_direction,provider_change_meaning,penalty_amount,penalty_currency_code,provider_fields_json,source_payload_id)VALUES($1,$2,$3,NULLIF($4,'')::numeric,NULLIF($5,'')::numeric,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,'')::numeric,NULLIF($10,''),$11,$12)`, seller, snapshotID, id, metric.Value, metric.ProviderPreviousValue, metric.ProviderStatus, metric.ProviderChangeDirection, metric.ProviderChangeMeaning, metric.PenaltyAmount, metric.PenaltyCurrency, jsonOrEmpty(metric.ProviderFields), metric.SourcePayloadID)
				if err != nil {
					return err
				}
			}
		}
		for _, fact := range result.History {
			id, metricErr := ensureMetric(fact.ProviderRatingKey, fact.SourcePayloadID, nil)
			if metricErr != nil {
				return metricErr
			}
			hash := factualapi.StableHash(fact.ProviderRatingKey, fact.PeriodFrom.Format("2006-01-02"), fact.PeriodTo.Format("2006-01-02"), fact.Value, fact.ProviderStatus, string(fact.ProviderFields))
			var revision int64
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT revision_no,state_hash FROM account_rating_period_facts WHERE account_health_metric_id=$1 AND period_from=$2 AND period_to=$3 ORDER BY revision_no DESC LIMIT 1`, id, fact.PeriodFrom, fact.PeriodTo).Scan(&revision, &latestHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash {
				continue
			}
			_, err = tx.Exec(ctx, `INSERT INTO account_rating_period_facts(seller_account_id,account_health_metric_id,period_from,period_to,revision_no,provider_value,provider_status,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::numeric,NULLIF($7,''),$8,$9,$10,$11)`, seller, id, fact.PeriodFrom, fact.PeriodTo, revision+1, fact.Value, fact.ProviderStatus, hash, at, fact.SourcePayloadID, run)
			if err != nil {
				return err
			}
			result.HistoryRevisionsWritten++
		}
		for _, threshold := range result.Thresholds {
			id, metricErr := ensureMetric(threshold.ProviderRatingKey, threshold.SourcePayloadID, nil)
			if metricErr != nil {
				return metricErr
			}
			hash := factualapi.StableHash(threshold.ProviderStatus, threshold.From, threshold.To, string(threshold.ProviderFields))
			var version int64
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT version_no,state_hash FROM account_rating_threshold_observations WHERE account_health_metric_id=$1 AND provider_status=$2 ORDER BY version_no DESC LIMIT 1`, id, threshold.ProviderStatus).Scan(&version, &latestHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash {
				continue
			}
			_, err = tx.Exec(ctx, `INSERT INTO account_rating_threshold_observations(seller_account_id,account_health_metric_id,version_no,provider_status,threshold_from,threshold_to,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,NULLIF($5,'')::numeric,NULLIF($6,'')::numeric,$7,$8,$9,$10,$11)`, seller, id, version+1, threshold.ProviderStatus, threshold.From, threshold.To, jsonOrEmpty(threshold.ProviderFields), hash, at, threshold.SourcePayloadID, run)
			if err != nil {
				return err
			}
			result.ThresholdVersionsWritten++
		}
		for _, penalty := range result.Penalties {
			var metricID *int64
			if penalty.ProviderRatingKey != "" {
				id, metricErr := ensureMetric(penalty.ProviderRatingKey, penalty.SourcePayloadID, nil)
				if metricErr != nil {
					return metricErr
				}
				metricID = &id
			}
			hash := penaltyHash(penalty, metricID)
			var revision int64
			var latestHash string
			queryErr := tx.QueryRow(ctx, `SELECT revision_no,state_hash FROM account_penalty_event_facts WHERE seller_account_id=$1 AND provider_event_key=$2 ORDER BY revision_no DESC LIMIT 1`, seller, penalty.ProviderEventKey).Scan(&revision, &latestHash)
			if queryErr != nil && !errors.Is(queryErr, pgx.ErrNoRows) {
				return queryErr
			}
			if latestHash == hash {
				continue
			}
			command, execErr := tx.Exec(ctx, `INSERT INTO account_penalty_event_facts(seller_account_id,provider_event_key,revision_no,account_health_metric_id,occurred_at,penalty_amount,currency_code,description,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::numeric,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,$12,$13)`, seller, penalty.ProviderEventKey, revision+1, metricID, penalty.OccurredAt, penalty.Amount, penalty.Currency, penalty.Description, jsonOrEmpty(penalty.ProviderFields), hash, at, penalty.SourcePayloadID, run)
			if execErr != nil {
				return execErr
			}
			result.PenaltiesWritten += int(command.RowsAffected())
		}
		if result.FBSIndex != nil {
			_, err = tx.Exec(ctx, `INSERT INTO fbs_error_index_facts(seller_account_id,observed_at,provider_period_from,provider_period_to,index_value,provider_status,provider_fields_json,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,NULLIF($5,'')::numeric,NULLIF($6,''),$7,$8,$9)`, seller, at, result.FBSIndex.PeriodFrom, result.FBSIndex.PeriodTo, result.FBSIndex.Value, result.FBSIndex.ProviderStatus, jsonOrEmpty(result.FBSIndex.ProviderFields), result.FBSIndex.SourcePayloadID, run)
			if err != nil {
				return err
			}
			result.FBSIndexWritten++
		}
		for _, event := range result.FBSErrors {
			hash := fbsErrorHash(event)
			var revision int64
			var latestHash string
			queryErr := tx.QueryRow(ctx, `SELECT revision_no,state_hash FROM fbs_posting_error_events WHERE seller_account_id=$1 AND provider_event_key=$2 ORDER BY revision_no DESC LIMIT 1`, seller, event.ProviderEventKey).Scan(&revision, &latestHash)
			if queryErr != nil && !errors.Is(queryErr, pgx.ErrNoRows) {
				return queryErr
			}
			if latestHash == hash {
				continue
			}
			command, execErr := tx.Exec(ctx, `INSERT INTO fbs_posting_error_events(seller_account_id,provider_event_key,revision_no,canonical_posting_id,source_posting_number,posting_resolution_status,error_code,error_message,occurred_at,provider_fields_json,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,$12,$13,$14)`, seller, event.ProviderEventKey, revision+1, event.CanonicalPostingID, event.SourcePostingNumber, event.ResolutionStatus, event.ErrorCode, event.ErrorMessage, event.OccurredAt, jsonOrEmpty(event.ProviderFields), hash, at, event.SourcePayloadID, run)
			if execErr != nil {
				return execErr
			}
			result.FBSErrorEventsWritten += int(command.RowsAffected())
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" {
			status = "PARTIAL"
		}
		received := len(result.Metrics) + len(result.History) + len(result.Thresholds) + len(result.Penalties) + len(result.FBSErrors)
		if result.FBSIndex != nil {
			received++
		}
		written := result.MetricVersionsWritten + result.SnapshotsWritten + result.HistoryRevisionsWritten + result.ThresholdVersionsWritten + result.PenaltiesWritten + result.FBSIndexWritten + result.FBSErrorEventsWritten
		command, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$5,pages_received=$6,completeness_status=$7,finished_at=$8,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, received, written, result.SummaryRequests+result.HistoryRequests+result.IndexRequests+result.ErrorPages, result.CompletenessStatus, result.CompletedAt)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return errors.New("stale ACCOUNT_HEALTH domain transition")
		}
		_, err = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'ACCOUNT_HEALTH',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, result.CompletedAt)
		return err
	})
}

func lockRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:ACCOUNT_HEALTH:%d", seller)); err != nil {
		return false, err
	}
	var status, domain string
	if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); err != nil {
		return false, err
	}
	if domain != "ACCOUNT_HEALTH" {
		return false, errors.New("expected ACCOUNT_HEALTH domain")
	}
	var head int64
	err := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='ACCOUNT_HEALTH'`, seller).Scan(&head)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if (status == "COMPLETED" || status == "PARTIAL") && err == nil && head >= run {
		return true, nil
	}
	if status != "RUNNING" {
		return false, fmt.Errorf("ACCOUNT_HEALTH domain must be RUNNING, got %s", status)
	}
	return false, nil
}

func metricHash(metric MetricObservation) string {
	return factualapi.StableHash(metric.ProviderRatingKey, metric.CanonicalMetricKey, metric.MappingStatus, metric.GroupName, metric.DisplayName, metric.Unit, metric.ValueType, metric.RatingDirection, metric.CanonicalFamily, metric.KnownCalculationWindow, optionalBool(metric.HistorySupported))
}

func penaltyHash(penalty PenaltyEvent, metricID *int64) string {
	return factualapi.StableHash(penalty.ProviderEventKey, optionalInt64(metricID), optionalTime(penalty.OccurredAt), penalty.Amount, penalty.Currency, penalty.Description, string(penalty.ProviderFields))
}

func fbsErrorHash(event FBSErrorEvent) string {
	return factualapi.StableHash(event.ProviderEventKey, optionalInt64(event.CanonicalPostingID), event.SourcePostingNumber, event.ResolutionStatus, event.ErrorCode, event.ErrorMessage, optionalTime(event.OccurredAt), string(event.ProviderFields))
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

func optionalBool(v *bool) string {
	if v == nil {
		return "NULL"
	}
	return strconv.FormatBool(*v)
}

func jsonOrEmpty(v json.RawMessage) string {
	if len(v) == 0 {
		return "{}"
	}
	return string(v)
}
