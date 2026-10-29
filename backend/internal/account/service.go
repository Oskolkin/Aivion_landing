package account

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	queries *dbgen.Queries
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(db),
	}
}

func (s *Service) GetByUserID(ctx context.Context, userID int64) (dbgen.SellerAccount, error) {
	account, err := s.queries.GetSellerAccountByUserID(ctx, userID)
	if err != nil {
		return dbgen.SellerAccount{}, fmt.Errorf("get seller account by user id: %w", err)
	}
	return account, nil
}
