package factual

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ratingScanner interface{ Scan(...any) error }

func scanRating(row ratingScanner) (factualapi.AccountRating, error) {
	var out factualapi.AccountRating
	var current, previous pgtype.Text
	var history pgtype.Bool
	if err := row.Scan(&out.ID, &out.ProviderRatingKey, &out.VersionNo, &out.CanonicalMetricKey, &out.MappingStatus, &out.ProviderGroupName, &out.ProviderDisplayName, &out.ValueType, &out.RatingDirection, &out.CanonicalFamily, &out.KnownCalculationWindow, &history, &out.ValueUnit, &current, &previous, &out.ProviderStatus, &out.ProviderChangeDirection, &out.ProviderChangeMeaning, &out.ObservedAt); err != nil {
		return out, err
	}
	if history.Valid {
		out.HistorySupported = &history.Bool
	}
	out.CurrentValue = decimalValue(textValue(current), "RATING_VALUE_UNAVAILABLE")
	out.ProviderPreviousValue = decimalValue(textValue(previous), "PROVIDER_PREVIOUS_VALUE_UNAVAILABLE")
	return out, nil
}

const currentRatingsSelect = `WITH snapshot AS(
 SELECT * FROM account_health_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY observed_at DESC,id DESC LIMIT 1
)SELECT m.id,m.provider_rating_key,v.version_no,v.canonical_metric_key,v.mapping_status,COALESCE(v.provider_group_name,''),COALESCE(v.provider_display_name,''),COALESCE(v.value_type,''),COALESCE(v.rating_direction,''),COALESCE(v.canonical_family,''),COALESCE(v.known_calculation_window,''),v.history_supported,COALESCE(v.value_unit,''),trim_scale(s.provider_value)::text,trim_scale(s.provider_previous_value)::text,COALESCE(s.provider_status,''),COALESCE(s.provider_change_direction,''),COALESCE(s.provider_change_meaning,''),snapshot.observed_at
FROM snapshot JOIN account_rating_snapshots s ON s.snapshot_run_id=snapshot.id
JOIN account_health_metrics m ON m.id=s.account_health_metric_id
JOIN LATERAL(SELECT * FROM account_health_metric_versions x WHERE x.account_health_metric_id=m.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true`

func (s *Service) GetCurrentRatings(ctx context.Context, c factualapi.FactualReadContext) ([]factualapi.AccountRating, error) {
	if err := s.validate(c); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, currentRatingsSelect+` ORDER BY m.id`, c.SellerAccountID(), c.CutoffAt())
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.AccountRating
	for rows.Next() {
		item, scanErr := scanRating(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		item.Meta, _ = s.meta(c, "HEALTH.CURRENT_RATINGS", "ACCOUNT_RATING", item.ID, nil, factHash(item))
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) GetRating(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.AccountRating, error) {
	var out factualapi.AccountRating
	if err := s.validate(c); err != nil {
		return out, err
	}
	out, err := scanRating(s.pool.QueryRow(ctx, currentRatingsSelect+` WHERE m.id=$3`, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("account rating")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.Meta, err = s.meta(c, "HEALTH.CURRENT_RATINGS", "ACCOUNT_RATING", out.ID, nil, factHash(out))
	return out, err
}

func (s *Service) GetAccountHealthSummary(ctx context.Context, c factualapi.FactualReadContext) (factualapi.AccountHealthSummary, error) {
	var out factualapi.AccountHealthSummary
	if err := s.validate(c); err != nil {
		return out, err
	}
	err := s.pool.QueryRow(ctx, `SELECT id,observed_at,completeness_status FROM account_health_snapshot_runs WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), c.CutoffAt()).Scan(&out.SnapshotID, &out.ObservedAt, &out.CompletenessStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("account health summary")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.TimeQuality = "OBSERVED_APPROX"
	out.Ratings, err = s.GetCurrentRatings(ctx, c)
	if err != nil {
		return out, err
	}
	rows, queryErr := s.pool.Query(ctx, `SELECT DISTINCT ON(provider_event_key)id,revision_no,provider_event_key,account_health_metric_id,occurred_at,penalty_amount::text,COALESCE(currency_code,''),COALESCE(description,''),observed_at FROM account_penalty_event_facts WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY provider_event_key,observed_at DESC,revision_no DESC,id DESC`, c.SellerAccountID(), c.CutoffAt())
	if queryErr != nil {
		return out, dbError(queryErr)
	}
	for rows.Next() {
		var item factualapi.AccountPenaltyEvent
		var rating pgtype.Int8
		var occurred pgtype.Timestamptz
		var amount pgtype.Text
		var currency string
		if err = rows.Scan(&item.ID, &item.RevisionNo, &item.ProviderEventKey, &rating, &occurred, &amount, &currency, &item.Description, &item.ObservedAt); err != nil {
			rows.Close()
			return out, err
		}
		if rating.Valid {
			item.RatingID = &rating.Int64
		}
		if occurred.Valid {
			item.OccurredAt = &occurred.Time
		}
		item.Amount = moneyValue(amount, currency, "PENALTY_AMOUNT_UNAVAILABLE")
		item.Meta, _ = s.meta(c, "HEALTH.PENALTY_EVENTS", "ACCOUNT_PENALTY_EVENT", item.ID, nil, factHash(item))
		out.PenaltyEvents = append(out.PenaltyEvents, item)
	}
	rows.Close()
	out.Meta, err = s.meta(c, "HEALTH.CURRENT_RATINGS", "ACCOUNT_HEALTH_SUMMARY", out.SnapshotID, nil, factHash(out))
	return out, err
}

func (s *Service) GetRatingHistory(ctx context.Context, c factualapi.FactualReadContext, ratingID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.RatingPeriodFact], error) {
	var out factualapi.Page[factualapi.RatingPeriodFact]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "GetRatingHistory", ratingID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(
 SELECT DISTINCT ON(period_from,period_to)* FROM account_rating_period_facts
 WHERE seller_account_id=$1 AND account_health_metric_id=$3 AND observed_at<=$2
 ORDER BY period_from,period_to,observed_at DESC,revision_no DESC,id DESC
)SELECT id,account_health_metric_id,revision_no,period_from,period_to,trim_scale(provider_value)::text,COALESCE(provider_status,''),observed_at FROM latest WHERE id>$4 ORDER BY id LIMIT $5`, c.SellerAccountID(), c.CutoffAt(), ratingID, last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item factualapi.RatingPeriodFact
		var value pgtype.Text
		if err = rows.Scan(&item.ID, &item.RatingID, &item.RevisionNo, &item.Period.From, &item.Period.To, &value, &item.ProviderStatus, &item.ObservedAt); err != nil {
			return out, err
		}
		item.Value = decimalValue(textValue(value), "RATING_VALUE_UNAVAILABLE")
		item.Meta, _ = s.meta(c, "HEALTH.RATING_HISTORY", "RATING_PERIOD", item.ID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "GetRatingHistory", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) GetRatingThresholds(ctx context.Context, c factualapi.FactualReadContext, ratingID int64) ([]factualapi.RatingThreshold, error) {
	if err := s.validate(c); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON(provider_status)id,account_health_metric_id,version_no,provider_status,threshold_from::text,threshold_to::text,observed_at FROM account_rating_threshold_observations WHERE seller_account_id=$1 AND account_health_metric_id=$3 AND observed_at<=$2 ORDER BY provider_status,observed_at DESC,version_no DESC,id DESC`, c.SellerAccountID(), c.CutoffAt(), ratingID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []factualapi.RatingThreshold
	for rows.Next() {
		var item factualapi.RatingThreshold
		var from, to pgtype.Text
		if err = rows.Scan(&item.ID, &item.RatingID, &item.VersionNo, &item.ProviderStatus, &from, &to, &item.ObservedAt); err != nil {
			return nil, err
		}
		item.ThresholdFrom = decimalValue(textValue(from), "THRESHOLD_FROM_UNAVAILABLE")
		item.ThresholdTo = decimalValue(textValue(to), "THRESHOLD_TO_UNAVAILABLE")
		item.Meta, _ = s.meta(c, "HEALTH.THRESHOLDS", "RATING_THRESHOLD", item.ID, nil, factHash(item))
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) GetFBSErrorIndex(ctx context.Context, c factualapi.FactualReadContext) (factualapi.FBSErrorIndex, error) {
	var out factualapi.FBSErrorIndex
	if err := s.validate(c); err != nil {
		return out, err
	}
	var value pgtype.Text
	var from, to pgtype.Date
	err := s.pool.QueryRow(ctx, `SELECT id,provider_period_from,provider_period_to,index_value::text,COALESCE(provider_status,''),observed_at FROM fbs_error_index_facts WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), c.CutoffAt()).Scan(&out.ID, &from, &to, &value, &out.ProviderStatus, &out.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("FBS error index")
	}
	if err != nil {
		return out, dbError(err)
	}
	if from.Valid && to.Valid {
		out.ProviderPeriod = &factualapi.Period{From: from.Time, To: to.Time}
	}
	out.Value = decimalValue(textValue(value), "FBS_ERROR_INDEX_UNAVAILABLE")
	out.Meta, err = s.meta(c, "HEALTH.FBS_ERROR_INDEX", "FBS_ERROR_INDEX", out.ID, nil, factHash(out))
	return out, err
}

func (s *Service) ListFBSRatingPostingErrors(ctx context.Context, c factualapi.FactualReadContext, page factualapi.PageRequest) (factualapi.Page[factualapi.FBSRatingPostingError], error) {
	var out factualapi.Page[factualapi.FBSRatingPostingError]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListFBSRatingPostingErrors", nil, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS(SELECT DISTINCT ON(provider_event_key)* FROM fbs_posting_error_events WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY provider_event_key,observed_at DESC,revision_no DESC,id DESC)SELECT id,revision_no,provider_event_key,canonical_posting_id,COALESCE(source_posting_number,''),posting_resolution_status,COALESCE(error_code,''),COALESCE(error_message,''),occurred_at,observed_at FROM latest WHERE id>$3 ORDER BY id LIMIT $4`, c.SellerAccountID(), c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item factualapi.FBSRatingPostingError
		var posting pgtype.Int8
		var occurred pgtype.Timestamptz
		if err = rows.Scan(&item.ID, &item.RevisionNo, &item.ProviderEventKey, &posting, &item.SourcePostingNumber, &item.PostingResolutionStatus, &item.ErrorCode, &item.ErrorMessage, &occurred, &item.ObservedAt); err != nil {
			return out, err
		}
		if posting.Valid {
			item.CanonicalPostingID = &posting.Int64
		}
		if occurred.Valid {
			item.OccurredAt = &occurred.Time
		}
		item.Meta, _ = s.meta(c, "HEALTH.FBS_POSTING_ERRORS", "FBS_RATING_POSTING_ERROR", item.ID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListFBSRatingPostingErrors", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}
