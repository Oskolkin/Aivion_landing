package recommendations

import (
	"encoding/json"
	"fmt"
)

// ContextBuildLimits caps recommendation AI context size (MVP: item counts + JSON byte budget).
// Zero values mean defaults (see DefaultContextBuildLimits).
type ContextBuildLimits struct {
	MaxItemsPerList int
	MaxContextBytes int
}

func DefaultContextBuildLimits() ContextBuildLimits {
	return ContextBuildLimits{
		MaxItemsPerList: defaultTopAlerts, // same cap as legacy per-list defaults
		MaxContextBytes: 480 * 1024,
	}
}

func (l ContextBuildLimits) effectiveMaxItems() int {
	if l.MaxItemsPerList > 0 {
		return l.MaxItemsPerList
	}
	return defaultTopAlerts
}

func (l ContextBuildLimits) effectiveMaxBytes() int {
	if l.MaxContextBytes > 0 {
		return l.MaxContextBytes
	}
	return 480 * 1024
}

// ApplyRecommendationContextBudget clamps lists and shrinks until marshaled JSON fits max bytes.
// Sets ContextTruncated / ContextTruncationReason / ContextApproxUncompressedBytes on ctx.
func ApplyRecommendationContextBudget(ctx *AIRecommendationContext, limits ContextBuildLimits) {
	if ctx == nil {
		return
	}
	maxItems := limits.effectiveMaxItems()
	maxBytes := limits.effectiveMaxBytes()

	ctx.Alerts.TopOpen = clamp(ctx.Alerts.TopOpen, maxItems)
	ctx.Recommendations.TopOpen = clamp(ctx.Recommendations.TopOpen, maxItems)
	ctx.Merchandising.TopRevenueSKUs = clamp(ctx.Merchandising.TopRevenueSKUs, maxItems)
	ctx.Merchandising.LowStockSKUs = clamp(ctx.Merchandising.LowStockSKUs, maxItems)
	ctx.Advertising.TopCampaigns = clamp(ctx.Advertising.TopCampaigns, maxItems)
	ctx.Pricing.TopConstrainedSKUs = clamp(ctx.Pricing.TopConstrainedSKUs, maxItems)

	reasons := []string{}
	if maxItems < defaultTopAlerts {
		reasons = append(reasons, fmt.Sprintf("item_cap=%d", maxItems))
	}

	for step := 0; step < 24; step++ {
		raw, err := json.Marshal(ctx)
		if err != nil {
			ctx.ContextTruncated = true
			ctx.ContextTruncationReason = "marshal_error:" + err.Error()
			break
		}
		ctx.ContextApproxUncompressedBytes = len(raw)
		if len(raw) <= maxBytes {
			if len(reasons) > 0 {
				ctx.ContextTruncated = true
				ctx.ContextTruncationReason = joinReasons(reasons)
			}
			return
		}
		ctx.ContextTruncated = true
		if shrinkContextOneStep(ctx) {
			reasons = append(reasons, fmt.Sprintf("byte_shrink_step=%d", step+1))
			continue
		}
		ctx.ContextTruncationReason = joinReasons(append(reasons, "byte_budget_still_exceeded_after_minimum_slices"))
		return
	}
	ctx.ContextTruncationReason = joinReasons(append(reasons, "byte_budget_max_steps"))
}

func joinReasons(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}

func shrinkContextOneStep(ctx *AIRecommendationContext) bool {
	if len(ctx.Recommendations.TopOpen) > 1 {
		n := len(ctx.Recommendations.TopOpen) / 2
		if n < 1 {
			n = 1
		}
		ctx.Recommendations.TopOpen = ctx.Recommendations.TopOpen[:n]
		return true
	}
	if len(ctx.Merchandising.TopRevenueSKUs) > 1 {
		n := len(ctx.Merchandising.TopRevenueSKUs) / 2
		if n < 1 {
			n = 1
		}
		ctx.Merchandising.TopRevenueSKUs = ctx.Merchandising.TopRevenueSKUs[:n]
		return true
	}
	if len(ctx.Merchandising.LowStockSKUs) > 1 {
		n := len(ctx.Merchandising.LowStockSKUs) / 2
		if n < 1 {
			n = 1
		}
		ctx.Merchandising.LowStockSKUs = ctx.Merchandising.LowStockSKUs[:n]
		return true
	}
	if len(ctx.Advertising.TopCampaigns) > 1 {
		n := len(ctx.Advertising.TopCampaigns) / 2
		if n < 1 {
			n = 1
		}
		ctx.Advertising.TopCampaigns = ctx.Advertising.TopCampaigns[:n]
		return true
	}
	if len(ctx.Pricing.TopConstrainedSKUs) > 1 {
		n := len(ctx.Pricing.TopConstrainedSKUs) / 2
		if n < 1 {
			n = 1
		}
		ctx.Pricing.TopConstrainedSKUs = ctx.Pricing.TopConstrainedSKUs[:n]
		return true
	}
	if len(ctx.Alerts.TopOpen) > 1 {
		n := len(ctx.Alerts.TopOpen) / 2
		if n < 1 {
			n = 1
		}
		ctx.Alerts.TopOpen = ctx.Alerts.TopOpen[:n]
		return true
	}
	return false
}
