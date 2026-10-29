package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/financeregistry"
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

func (r *Repository) Finalize(ctx context.Context, seller, run int64, result *Result) error {
	if r == nil || r.pool == nil || result == nil {
		return errors.New("finance finalization requires repository and result")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		already, err := lockFinanceRun(ctx, tx, seller, run)
		if err != nil || already {
			return err
		}
		at := r.now().UTC()
		typeIDs, err := syncProviderTypes(ctx, tx, result.ProviderTypes, result.DictionaryHash, at)
		if err != nil {
			return err
		}
		written := 0
		for _, day := range result.Days {
			var latestHash string
			err = tx.QueryRow(ctx, `SELECT state_hash FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date=$2 ORDER BY revision_no DESC,id DESC LIMIT 1`, seller, day.BusinessDate).Scan(&latestHash)
			if err == nil && latestHash == day.StateHash {
				continue
			}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			var revisionNo, revisionID int64
			err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision_no),0)+1 FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date=$2`, seller, day.BusinessDate).Scan(&revisionNo)
			if err != nil {
				return err
			}
			err = tx.QueryRow(ctx, `INSERT INTO finance_accrual_day_revisions(seller_account_id,business_date,revision_no,collection_domain_run_id,source_payload_id,completeness_status,state_hash,observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8)RETURNING id`, seller, day.BusinessDate, revisionNo, run, day.SourcePayloadID, day.CompletenessStatus, day.StateHash, at).Scan(&revisionID)
			if err != nil {
				return err
			}
			for _, record := range day.Records {
				providerTypeID, ok := typeIDs[record.ProviderTypeID]
				if !ok {
					providerTypeID, err = ensureUnknownProviderType(ctx, tx, record.ProviderTypeID, at)
					if err != nil {
						return err
					}
					typeIDs[record.ProviderTypeID] = providerTypeID
				}
				var recordID int64
				err = tx.QueryRow(ctx, `INSERT INTO finance_accrual_records(seller_account_id,day_revision_id,row_ordinal,total_amount_signed,currency_code,finance_provider_type_id,provider_type_id,provider_accrued_category,unit_number,source_fragment_json)VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),$10)RETURNING id`, seller, revisionID, record.RowOrdinal, record.TotalAmountSigned, record.CurrencyCode, providerTypeID, record.ProviderTypeID, record.ProviderAccruedCategory, record.UnitNumber, jsonValue(record.SourceFragment)).Scan(&recordID)
				if err != nil {
					return err
				}
				for _, line := range record.Lines {
					_, err = tx.Exec(ctx, `INSERT INTO finance_accrual_lines(seller_account_id,finance_accrual_record_id,line_ordinal,source_component_path,signed_amount,mapping_status,canonical_sku_id,canonical_order_id,canonical_posting_id,source_ozon_sku,source_product_id,source_fragment_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, seller, recordID, line.Ordinal, line.SourceComponentPath, line.SignedAmount, line.MappingStatus, line.CanonicalSKUID, line.CanonicalOrderID, line.CanonicalPostingID, line.SourceOzonSKU, line.SourceProductID, jsonValue(line.SourceFragment))
					if err != nil {
						return err
					}
				}
				written++
			}
		}
		status := "COMPLETED"
		if result.CompletenessStatus == "PARTIAL" {
			status = "PARTIAL"
		}
		cmd, err := tx.Exec(ctx, `UPDATE collection_domain_runs SET status=$3,records_received=$4,records_written=$4,pages_received=$5,completeness_status=$6,finished_at=$7,error_code=NULL,error_message=NULL,updated_at=NOW()WHERE id=$1 AND seller_account_id=$2 AND status='RUNNING'`, run, seller, status, written, result.PagesReceived, result.CompletenessStatus, at)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return errors.New("stale finance domain transition")
		}
		_, err = tx.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'FINANCE',$2,$3)ON CONFLICT(seller_account_id,domain)DO UPDATE SET collection_domain_run_id=EXCLUDED.collection_domain_run_id,finalized_at=EXCLUDED.finalized_at,updated_at=NOW()WHERE collection_authoritative_heads.collection_domain_run_id<=EXCLUDED.collection_domain_run_id`, seller, run, at)
		return err
	})
}

func syncProviderTypes(ctx context.Context, tx pgx.Tx, types []financeregistry.ProviderType, hash string, at time.Time) (map[string]int64, error) {
	out := make(map[string]int64, len(types))
	for _, item := range types {
		item = financeregistry.Normalize(item)
		var id int64
		err := tx.QueryRow(ctx, `INSERT INTO finance_provider_types(provider,provider_type_id,first_observed_at,last_observed_at)VALUES('OZON',$1,$2,$2)ON CONFLICT(provider,provider_type_id)DO UPDATE SET last_observed_at=GREATEST(finance_provider_types.last_observed_at,EXCLUDED.last_observed_at)RETURNING id`, item.ProviderTypeID, at).Scan(&id)
		if err != nil {
			return nil, err
		}
		out[item.ProviderTypeID] = id
		var currentHash string
		err = tx.QueryRow(ctx, `SELECT source_dictionary_hash FROM finance_provider_type_versions WHERE finance_provider_type_id=$1 AND valid_to IS NULL`, id).Scan(&currentHash)
		if err == nil && currentHash == hash {
			continue
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE finance_provider_type_versions SET valid_to=$2 WHERE finance_provider_type_id=$1 AND valid_to IS NULL`, id, at); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO finance_provider_type_versions(finance_provider_type_id,version_no,name,description,canonical_group,canonical_subtype,economic_role,default_scope,cross_domain_owner,mapping_status,source_dictionary_hash,valid_from,observed_at)VALUES($1,(SELECT COALESCE(max(version_no),0)+1 FROM finance_provider_type_versions WHERE finance_provider_type_id=$1),$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),$9,$10,$11,$11)`, id, item.Name, item.Description, item.CanonicalGroup, item.CanonicalSubtype, item.EconomicRole, item.DefaultScope, item.CrossDomainOwner, item.MappingStatus, hash, at)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func ensureUnknownProviderType(ctx context.Context, tx pgx.Tx, providerType string, at time.Time) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO finance_provider_types(provider,provider_type_id,first_observed_at,last_observed_at)VALUES('OZON',$1,$2,$2)ON CONFLICT(provider,provider_type_id)DO UPDATE SET last_observed_at=GREATEST(finance_provider_types.last_observed_at,EXCLUDED.last_observed_at)RETURNING id`, providerType, at).Scan(&id)
	return id, err
}

func lockFinanceRun(ctx context.Context, tx pgx.Tx, seller, run int64) (bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("canonical:FINANCE:%d", seller)); err != nil {
		return false, err
	}
	var status, domain string
	if err := tx.QueryRow(ctx, `SELECT status,domain FROM collection_domain_runs WHERE id=$1 AND seller_account_id=$2 FOR UPDATE`, run, seller).Scan(&status, &domain); err != nil {
		return false, err
	}
	if domain != "FINANCE" {
		return false, errors.New("expected FINANCE domain")
	}
	if status != "RUNNING" {
		var head int64
		err := tx.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='FINANCE'`, seller).Scan(&head)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("FINANCE domain must be RUNNING, got %s", status)
		}
		return err == nil && head >= run, err
	}
	return false, nil
}

func jsonValue(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
