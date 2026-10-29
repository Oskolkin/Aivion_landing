package syncstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CursorTypeProductsLastID   = "last_id"
	CursorTypeOrdersSince      = "since"
	CursorTypeStocksSnapshotAt = "snapshot_at"
	CursorTypeAdsSince         = "since"
)

type SyncCursorService struct {
	queries *dbgen.Queries
}

func NewSyncCursorService(db *pgxpool.Pool) *SyncCursorService {
	return &SyncCursorService{
		queries: dbgen.New(db),
	}
}

func DefaultCursorTypeForDomain(domain string) (string, error) {
	switch domain {
	case "products":
		return CursorTypeProductsLastID, nil
	case "orders":
		return CursorTypeOrdersSince, nil
	case "stocks":
		return CursorTypeStocksSnapshotAt, nil
	case "ads":
		return CursorTypeAdsSince, nil
	default:
		return "", fmt.Errorf("unsupported cursor domain: %s", domain)
	}
}

func (s *SyncCursorService) ResolveSourceCursor(
	ctx context.Context,
	sellerAccountID int64,
	domain string,
) (pgtype.Text, error) {
	cursorType, err := DefaultCursorTypeForDomain(domain)
	if err != nil {
		return pgtype.Text{}, err
	}

	row, err := s.queries.GetSyncCursorBySellerAccountDomainAndType(
		ctx,
		dbgen.GetSyncCursorBySellerAccountDomainAndTypeParams{
			SellerAccountID: sellerAccountID,
			Domain:          domain,
			CursorType:      cursorType,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.Text{Valid: false}, nil
		}
		return pgtype.Text{}, fmt.Errorf("get sync cursor: %w", err)
	}

	return row.CursorValue, nil
}

func (s *SyncCursorService) AdvanceCursor(
	ctx context.Context,
	sellerAccountID int64,
	domain string,
	cursorValue string,
) (dbgen.SyncCursor, error) {
	cursorType, err := DefaultCursorTypeForDomain(domain)
	if err != nil {
		return dbgen.SyncCursor{}, err
	}

	row, err := s.queries.UpsertSyncCursor(ctx, dbgen.UpsertSyncCursorParams{
		SellerAccountID: sellerAccountID,
		Domain:          domain,
		CursorType:      cursorType,
		CursorValue:     nullableCursorText(cursorValue),
	})
	if err != nil {
		return dbgen.SyncCursor{}, fmt.Errorf("upsert sync cursor: %w", err)
	}

	return row, nil
}

func (s *SyncCursorService) ResetCursor(
	ctx context.Context,
	sellerAccountID int64,
	domain string,
	cursorType string,
	cursorValue *string,
) (dbgen.SyncCursor, error) {
	if domain == "" {
		return dbgen.SyncCursor{}, fmt.Errorf("domain is required")
	}
	if cursorType == "" {
		return dbgen.SyncCursor{}, fmt.Errorf("cursor_type is required")
	}

	value := pgtype.Text{Valid: false}
	if cursorValue != nil && *cursorValue != "" {
		value = pgtype.Text{String: *cursorValue, Valid: true}
	}

	row, err := s.queries.UpsertSyncCursor(ctx, dbgen.UpsertSyncCursorParams{
		SellerAccountID: sellerAccountID,
		Domain:          domain,
		CursorType:      cursorType,
		CursorValue:     value,
	})
	if err != nil {
		return dbgen.SyncCursor{}, fmt.Errorf("reset sync cursor: %w", err)
	}
	return row, nil
}

func nullableCursorText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}

	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}
