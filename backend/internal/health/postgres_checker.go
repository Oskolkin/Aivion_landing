package health

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresChecker struct {
	pool *pgxpool.Pool
}

func NewPostgresChecker(pool *pgxpool.Pool) *PostgresChecker {
	return &PostgresChecker{
		pool: pool,
	}
}

func (c *PostgresChecker) Check(ctx context.Context) error {
	return c.pool.Ping(ctx)
}
