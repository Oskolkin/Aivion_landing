package funnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
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

func (r *Repository) Finalize(ctx context.Context, sellerID, domainRunID int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("funnel finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:FUNNEL:%d", sellerID)); err != nil {
			return err
		}
		var status, domain string
		if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, domainRunID, sellerID).Scan(&status, &domain); err != nil {
			return err
		}
		if domain != "FUNNEL" {
			return fmt.Errorf("expected FUNNEL domain")
		}
		if status != "RUNNING" {
			return fmt.Errorf("funnel domain must be RUNNING, got %s", status)
		}
		accepted := r.now().UTC()
		sort.Slice(result.Observations, func(i, j int) bool {
			a, b := result.Observations[i], result.Observations[j]
			return observationGrain(a) < observationGrain(b)
		})
		for i := range result.Observations {
			o := &result.Observations[i]
			var canonicalID *int64
			if o.ScopeKind == "SKU" {
				if o.SourceSKU == nil || *o.SourceSKU <= 0 {
					return errors.New("SKU funnel observation requires provider SKU")
				}
				var id int64
				err := tx.QueryRow(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=$2`, sellerID, *o.SourceSKU).Scan(&id)
				if err == nil {
					canonicalID = &id
					o.CanonicalSKUID = &id
					o.SKUResolutionStatus = "RESOLVED"
				} else if errors.Is(err, pgx.ErrNoRows) {
					o.CanonicalSKUID = nil
					o.SKUResolutionStatus = "UNRESOLVED"
					result.UnresolvedSKUs++
				} else {
					return err
				}
			} else {
				o.SourceSKU, o.CanonicalSKUID = nil, nil
				o.SKUResolutionStatus = "NOT_APPLICABLE"
			}
			var latestHash string
			var latestRevision int64
			source := int64(0)
			if o.SourceSKU != nil {
				source = *o.SourceSKU
			}
			err := tx.QueryRow(ctx, `SELECT state_hash,revision_no FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind=$2 AND COALESCE(source_sku,0)=$3 AND metric_key=$4 AND period_start=$5 AND period_end=$6 ORDER BY revision_no DESC,id DESC LIMIT 1`, sellerID, o.ScopeKind, source, o.MetricKey, o.PeriodStart, o.PeriodEnd).Scan(&latestHash, &latestRevision)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			stateHash := observationStateHash(*o, canonicalID)
			if latestHash == stateHash {
				continue
			}
			if _, err = tx.Exec(ctx, `INSERT INTO funnel_observations(seller_account_id,scope_kind,canonical_sku_id,source_sku,sku_resolution_status,metric_key,period_start,period_end,value_numeric,unit,temporal_semantics,entity_aggregation_semantics,surface,provider_metric_name,registry_version,source_contract_hash,revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::numeric,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`, sellerID, o.ScopeKind, canonicalID, o.SourceSKU, o.SKUResolutionStatus, o.MetricKey, o.PeriodStart, o.PeriodEnd, o.ValueNumeric, o.Unit, o.TemporalSemantics, o.EntityAggregationSemantics, o.Surface, o.ProviderMetricName, o.RegistryVersion, o.SourceContractHash, latestRevision+1, stateHash, domainRunID, o.SourcePayloadID, accepted); err != nil {
				return err
			}
			result.RevisionsWritten++
		}
		finalStatus, completeness := "COMPLETED", "COMPLETE"
		if result.UnresolvedSKUs > 0 {
			finalStatus, completeness = "PARTIAL", "PARTIAL"
		}
		result.RecordsWritten = result.RevisionsWritten
		cmd, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,coverage_from=$4,coverage_to=$5,records_received=$6,records_written=$7,pages_received=$8,completeness_status=$9,finished_at=$10,error_code=NULL,error_message=NULL,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, domainRunID, sellerID, finalStatus, result.RequestedFrom, result.RequestedTo, result.RecordsReceived, result.RecordsWritten, result.Pages, completeness, accepted)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale funnel domain transition")
		}
		_, err = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'FUNNEL',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, sellerID, domainRunID, accepted)
		return err
	})
}

func observationGrain(o Observation) string {
	source := int64(0)
	if o.SourceSKU != nil {
		source = *o.SourceSKU
	}
	return fmt.Sprintf("%s/%d/%s/%s/%s", o.ScopeKind, source, o.MetricKey, o.PeriodStart.Format("2006-01-02"), o.PeriodEnd.Format("2006-01-02"))
}

// observationStateHash is semantic only: no revision_no, observed_at, or DB row ids.
func observationStateHash(o Observation, canonicalID *int64) string {
	valueState := strings.Join([]string{
		o.ValueNumeric, o.Unit, o.TemporalSemantics, o.EntityAggregationSemantics, o.Surface,
		o.ProviderMetricName, o.RegistryVersion, o.SourceContractHash, optionalInt64(canonicalID), o.SKUResolutionStatus,
	}, "|")
	return hash(valueState)
}

func optionalInt64(v *int64) string {
	if v == nil {
		return "NULL"
	}
	return strconv.FormatInt(*v, 10)
}

func hash(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
