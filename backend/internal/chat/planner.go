package chat

import (
	"context"
	"time"
)

type Planner interface {
	Plan(ctx context.Context, input PlannerInput) (*ToolPlan, error)
}

type PlannerInput struct {
	Question        string
	SellerAccountID int64
	AsOfDate        *time.Time
	AllowedTools    []ToolDefinition
}
