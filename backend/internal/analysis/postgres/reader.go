package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type PublicationRow struct {
	ID, SellerID, Generation, RunID int64
	UniverseID                      string
	UniverseHash, ContentHash       string
	AnalysisDate                    time.Time
	CutoffAt                        time.Time
	PublishedAt                     time.Time
}

func (r *Repository) GetPublication(ctx context.Context, id, sellerID int64) (PublicationRow, error) {
	var p PublicationRow
	err := r.pool.QueryRow(ctx, `SELECT id,seller_account_id,publication_generation,analysis_run_id,factual_universe_id::text,factual_universe_hash,publication_content_hash,
		analysis_date,analysis_cutoff_at,published_at
		FROM analysis_publications WHERE id=$1 AND seller_account_id=$2`, id, sellerID).
		Scan(&p.ID, &p.SellerID, &p.Generation, &p.RunID, &p.UniverseID, &p.UniverseHash, &p.ContentHash, &p.AnalysisDate, &p.CutoffAt, &p.PublishedAt)
	return p, err
}

func (r *Repository) GetCurrentPublication(ctx context.Context, sellerID int64) (PublicationRow, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1`, sellerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicationRow{}, err
	}
	if err != nil {
		return PublicationRow{}, err
	}
	return r.GetPublication(ctx, id, sellerID)
}

func (r *Repository) GetProjection(ctx context.Context, publicationID int64, key string) ([]byte, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT payload_json FROM publication_projections WHERE publication_id=$1 AND projection_key=$2`, publicationID, key).Scan(&raw)
	return raw, err
}

type ProjectionRow struct {
	ID   string
	JSON []byte
}

func (r *Repository) ListPinnedRows(ctx context.Context, sellerID, publicationID int64, table, idCol, afterID, bandFilter string, noSales *bool, limit int) ([]ProjectionRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE id=$1 AND seller_account_id=$2`, publicationID, sellerID).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, pgx.ErrNoRows
	}
	q := `SELECT ` + idCol + `, row_json FROM ` + table + ` WHERE publication_id=$1`
	args := []any{publicationID}
	i := 2
	if afterID != "" {
		q += ` AND ` + idCol + ` > $` + strconv.Itoa(i)
		args = append(args, afterID)
		i++
	}
	if bandFilter != "" {
		q += ` AND row_json->>'coverage_band' = $` + strconv.Itoa(i)
		args = append(args, bandFilter)
		i++
	}
	if noSales != nil && *noSales {
		q += ` AND (row_json->>'no_sales')::boolean = true`
	}
	q += ` ORDER BY ` + idCol + ` LIMIT $` + strconv.Itoa(i)
	args = append(args, limit)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectionRow
	for rows.Next() {
		var rrow ProjectionRow
		if err := rows.Scan(&rrow.ID, &rrow.JSON); err != nil {
			return nil, err
		}
		out = append(out, rrow)
	}
	return out, rows.Err()
}
