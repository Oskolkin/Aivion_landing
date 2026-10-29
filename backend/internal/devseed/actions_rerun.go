package devseed

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RerunActionsOptions struct {
	SellerAccountID int64
	MetricDate      *time.Time
	AIEnabled       *bool
	Force           bool
	DryRun          bool
}

func RerunActions(ctx context.Context, pool *pgxpool.Pool, ai actions.ActionAIClient, opts RerunActionsOptions) (*actions.RerunActionsResponse, error) {
	if err := AssertDevActionsRerunAllowed(); err != nil {
		return nil, err
	}
	if opts.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller-id must be > 0")
	}
	aiFirstRaw := strings.TrimSpace(strings.ToLower(os.Getenv("ACTIONS_AI_FIRST")))
	if ai == nil && (aiFirstRaw == "true" || aiFirstRaw == "1" || aiFirstRaw == "yes") {
		ai = actions.NewDeterministicAIFirstFakeClient()
	}
	svc := actions.NewRerunService(actions.NewGenerationServiceFromPool(pool, ai))
	return svc.RerunActions(ctx, actions.RerunActionsRequest{
		SellerAccountID: opts.SellerAccountID,
		MetricDate:      opts.MetricDate,
		AIEnabled:       opts.AIEnabled,
		Force:           opts.Force,
		DryRun:          opts.DryRun,
	})
}
