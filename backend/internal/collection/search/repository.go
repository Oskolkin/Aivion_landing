package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// ResolveSearchUniverse loads verified ozon_sku identities only (no ProductID fallback).
func (r *Repository) ResolveSearchUniverse(ctx context.Context, seller int64, cutoff time.Time) ([]FrozenSKU, error) {
	rows, err := r.pool.Query(ctx, `SELECT cs.id,cs.ozon_sku FROM canonical_skus cs JOIN LATERAL(SELECT catalog_state FROM canonical_sku_versions v WHERE v.canonical_sku_id=cs.id AND v.observed_at<=$2 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1)v ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND cs.ozon_sku IS NOT NULL AND cs.ozon_sku>0 AND v.catalog_state<>'UNOBSERVED' ORDER BY cs.ozon_sku`, seller, cutoff)
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
		return errors.New("search finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:SEARCH:%d", seller)); err != nil {
			return err
		}
		var status, domain string
		if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); err != nil {
			return err
		}
		if domain != "SEARCH" || status != "RUNNING" {
			return fmt.Errorf("expected running SEARCH domain")
		}
		at := r.now().UTC()
		written := 0
		for i := range result.SKUSummaries {
			x := &result.SKUSummaries[i]
			hash := skuSummaryHash(*x)
			var latestHash string
			var latestRevision int64
			err := tx.QueryRow(ctx, `SELECT state_hash,revision_no FROM search_sku_summaries WHERE seller_account_id=$1 AND source_sku=$2 AND period_start=$3 AND period_end=$4 ORDER BY revision_no DESC,id DESC LIMIT 1`, seller, x.SourceSKU, x.PeriodStart, x.PeriodEnd).Scan(&latestHash, &latestRevision)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash {
				continue
			}
			agg := x.UniqueUsersAggregation
			if agg == "" {
				agg = "NON_ADDITIVE"
			}
			if _, err = tx.Exec(ctx, `INSERT INTO search_sku_summaries(seller_account_id,canonical_sku_id,source_sku,sku_resolution_status,period_start,period_end,period_kind,provider_search_gmv,orders,unique_search_users,unique_view_users,position,view_conversion,unique_users_aggregation,revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at,source_fragment_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::numeric,NULLIF($11,'')::numeric,NULLIF($12,'')::numeric,NULLIF($13,'')::numeric,$14,$15,$16,$17,$18,$19,$20)`,
				seller, x.CanonicalSKUID, x.SourceSKU, x.SKUResolutionStatus, x.PeriodStart, x.PeriodEnd, x.PeriodKind,
				x.ProviderSearchGMV, x.Orders, x.UniqueSearchUsers, x.UniqueViewUsers, x.Position, x.ViewConversion,
				agg, latestRevision+1, hash, run, x.SourcePayloadID, x.ObservedAt, jsonValue(x.SourceFragment)); err != nil {
				return err
			}
			written++
		}
		for i := range result.Details {
			x := &result.Details[i]
			qid, err := upsertQuery(ctx, tx, x.Identity, run, x.SourcePayloadID, x.ObservedAt)
			if err != nil {
				return err
			}
			if x.CanonicalSKUID == nil {
				var id int64
				if e := tx.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2`, seller, x.SourceSKU).Scan(&id); e == nil {
					x.CanonicalSKUID = &id
					x.SKUResolutionStatus = "RESOLVED"
				} else if !errors.Is(e, pgx.ErrNoRows) {
					return e
				} else if x.SKUResolutionStatus == "" {
					x.SKUResolutionStatus = "UNRESOLVED"
				}
			}
			hash := detailHash(*x, qid)
			var latestHash string
			var latestRevision int64
			err = tx.QueryRow(ctx, `SELECT state_hash,revision_no FROM search_query_details WHERE seller_account_id=$1 AND search_query_id=$2 AND source_sku=$3 AND period_start=$4 AND period_end=$5 ORDER BY revision_no DESC,id DESC LIMIT 1`, seller, qid, x.SourceSKU, x.PeriodStart, x.PeriodEnd).Scan(&latestHash, &latestRevision)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if latestHash == hash {
				continue
			}
			if _, err = tx.Exec(ctx, `INSERT INTO search_query_details(seller_account_id,search_query_id,canonical_sku_id,source_sku,sku_resolution_status,provider_rank,provider_search_gmv,orders,period_start,period_end,retrieval_scope,retrieval_reason,top_n_limit,collection_policy,sort_by,sort_dir,limit_by_sku,page,page_size,revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at,source_fragment_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)`,
				seller, qid, x.CanonicalSKUID, x.SourceSKU, x.SKUResolutionStatus, x.ProviderRank, x.ProviderSearchGMV, x.Orders,
				x.PeriodStart, x.PeriodEnd, x.RetrievalScope, x.RetrievalReason, x.TopNLimit, DetailCollectionPolicyV1,
				defaultString(x.SortBy, DefaultSortBy), defaultString(x.SortDir, DefaultSortDir),
				nonzero(x.LimitBySKU, x.TopNLimit), nonzero(x.Page, 1), nonzero(x.PageSize, DefaultPageSize),
				latestRevision+1, hash, run, x.SourcePayloadID, x.ObservedAt, jsonValue(x.SourceFragment)); err != nil {
				return err
			}
			written++
		}
		for _, x := range result.MarketFacts {
			qid, err := upsertQuery(ctx, tx, x.Identity, run, x.SourcePayloadID, x.ObservedAt)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO search_market_facts(seller_account_id,search_query_id,provider_rank,period_start,period_end,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8)ON CONFLICT DO NOTHING`, seller, qid, x.ProviderRank, x.PeriodStart, x.PeriodEnd, run, x.SourcePayloadID, x.ObservedAt); err != nil {
				return err
			}
		}
		runStatus := "COMPLETED"
		if result.CompletenessStatus != "COMPLETE" {
			runStatus = "PARTIAL"
		}
		if _, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$4,pages_received=$5,completeness_status=$6,finished_at=$7,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, runStatus, written, result.PagesReceived, result.CompletenessStatus, at); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'SEARCH',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, at)
		return err
	})
}

func upsertQuery(ctx context.Context, tx pgx.Tx, x QueryIdentity, run, payload int64, at time.Time) (int64, error) {
	var id int64
	var firstDisplay string
	var created bool
	err := tx.QueryRow(ctx, `INSERT INTO search_queries(provider,market,normalized_query_key,display_query)VALUES($1,$2,$3,$4)ON CONFLICT(provider,market,normalized_query_key)DO UPDATE SET display_query=search_queries.display_query RETURNING id,display_query,(xmax=0)`, x.Provider, x.Market, x.NormalizedQueryKey, x.QueryText).Scan(&id, &firstDisplay, &created)
	if err != nil {
		return 0, err
	}
	if created {
		_, err = tx.Exec(ctx, `INSERT INTO search_query_versions(search_query_id,display_query,version_no,observed_at,collection_domain_run_id,source_payload_id)VALUES($1,$2,1,$3,$4,$5)ON CONFLICT DO NOTHING`, id, firstDisplay, at, run, payload)
		return id, err
	}
	var latestDisplay string
	var latestVersion int64
	err = tx.QueryRow(ctx, `SELECT display_query,version_no FROM search_query_versions WHERE search_query_id=$1 ORDER BY version_no DESC LIMIT 1`, id).Scan(&latestDisplay, &latestVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO search_query_versions(search_query_id,display_query,version_no,observed_at,collection_domain_run_id,source_payload_id)VALUES($1,$2,1,$3,$4,$5)`, id, firstDisplay, at, run, payload)
		if err != nil {
			return 0, err
		}
		latestDisplay, latestVersion = firstDisplay, 1
	} else if err != nil {
		return 0, err
	}
	if x.QueryText != "" && x.QueryText != latestDisplay {
		_, err = tx.Exec(ctx, `INSERT INTO search_query_versions(search_query_id,display_query,version_no,observed_at,collection_domain_run_id,source_payload_id)VALUES($1,$2,$3,$4,$5,$6)`, id, x.QueryText, latestVersion+1, at, run, payload)
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func skuSummaryHash(x SKUSummary) string {
	return factualapi.StableHash(
		optionalInt64(x.CanonicalSKUID), strconv.FormatInt(x.SourceSKU, 10), x.SKUResolutionStatus,
		x.PeriodStart.Format("2006-01-02"), x.PeriodEnd.Format("2006-01-02"), x.PeriodKind,
		x.ProviderSearchGMV, x.Orders, x.UniqueSearchUsers, x.UniqueViewUsers,
		x.Position, x.ViewConversion, x.UniqueUsersAggregation,
	)
}

func detailHash(x Detail, qid int64) string {
	return factualapi.StableHash(
		strconv.FormatInt(qid, 10), strconv.FormatInt(x.SourceSKU, 10), strconv.Itoa(x.ProviderRank),
		optionalInt64(x.CanonicalSKUID), x.SKUResolutionStatus,
		x.PeriodStart.Format("2006-01-02"), x.PeriodEnd.Format("2006-01-02"), x.ProviderSearchGMV, x.Orders,
		x.RetrievalScope, x.RetrievalReason, strconv.Itoa(nonzero(x.TopNLimit, x.LimitBySKU)),
		x.SortBy, x.SortDir, strconv.Itoa(x.LimitBySKU), strconv.Itoa(x.Page), strconv.Itoa(x.PageSize),
	)
}

func optionalInt64(v *int64) string {
	if v == nil {
		return "NULL"
	}
	return strconv.FormatInt(*v, 10)
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func nonzero(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func jsonValue(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
