package alerts

import (
	"fmt"
)

func evaluateFBOSurplusStorageRisk(input StockRuleEvaluationInput, ctx FBOStorageRiskSKUContext) (RuleResult, bool) {
	eval := evaluateFBOStorageRiskSignals(ctx)
	if !eval.Trigger {
		return RuleResult{}, false
	}

	entityID := fmt.Sprintf("%d", ctx.OzonProductID)
	if ctx.OzonProductID <= 0 {
		entityID = fmt.Sprintf("%d", ctx.SKU)
	}
	sku := ctx.SKU
	title := "Риск избыточного FBO-хранения по SKU"
	message := fmt.Sprintf(
		"По товару %d на FBO накопился избыточный остаток: запаса может хватить примерно на %s дней",
		ctx.SKU,
		formatIDCDays(ctx.IDC),
	)
	if len(eval.Reasons) > 0 {
		message += ". Проверьте необходимость акции, скидки, вывоза или сокращения будущих поставок."
	}

	evidence := BuildStockEvidence(idcFloat(ctx.IDC), int64(ctx.AvailableStockCount), adsFloat(ctx.ADS), EvidencePayload{
		"metric":                      "fbo_surplus_storage_risk",
		"source_method":               fboStorageRiskSourceMethod,
		"metric_quality":              fboStorageRiskMetricQuality,
		"sku":                         ctx.SKU,
		"offer_id":                    ctx.OfferID,
		"product_name":                ctx.ProductName,
		"product_id":                  ctx.OzonProductID,
		"ozon_product_id":             ctx.OzonProductID,
		"available_stock_count":       ctx.AvailableStockCount,
		"excess_stock_count":          ctx.ExcessStockCount,
		"excess_share":                evidenceExcessShare(ctx),
		"ads":                         evidenceFloatPtr(ctx.ADS),
		"idc":                         evidenceInt32Ptr(ctx.IDC),
		"turnover_grade":              ctx.TurnoverGrade,
		"turnover_grade_cluster":      ctx.TurnoverGradeCluster,
		"days_without_sales":          evidenceInt32Ptr(ctx.DaysWithoutSales),
		"requested_stock_count":       ctx.RequestedStockCount,
		"transit_stock_count":         ctx.TransitStockCount,
		"valid_stock_count":           ctx.ValidStockCount,
		"waiting_docs_stock_count":    ctx.WaitingDocsStockCount,
		"warehouse_names":             ctx.WarehouseNames,
		"warehouse_ids":               ctx.WarehouseIDs,
		"threshold_idc_days":          fboStorageRiskIDCThresholdDays,
		"threshold_high_idc_days":     fboStorageRiskIDCHighDays,
		"threshold_critical_idc_days": fboStorageRiskIDCCriticalDays,
		"reason":                      primaryReason(eval.Reasons),
		"reasons":                     eval.Reasons,
		"as_of_date":                  input.AsOfDate,
	})

	return RuleResult{
		AlertType:       AlertTypeFBOSurplusStorageRisk,
		AlertGroup:      AlertGroupStock,
		EntityType:      EntityTypeSKU,
		EntityID:        &entityID,
		EntitySKU:       &sku,
		EntityOfferID:   ctx.OfferID,
		Title:           title,
		Message:         message,
		Severity:        eval.Severity,
		Urgency:         eval.Urgency,
		EvidencePayload: evidence,
	}, true
}

func formatIDCDays(idc *int32) string {
	if idc == nil {
		return "неизвестное количество"
	}
	return fmt.Sprintf("%d", *idc)
}

func idcFloat(idc *int32) float64 {
	if idc == nil {
		return 0
	}
	return float64(*idc)
}

func adsFloat(ads *float64) float64 {
	if ads == nil {
		return 0
	}
	return *ads
}

func evidenceExcessShare(ctx FBOStorageRiskSKUContext) any {
	if !ctx.HasExcessShare {
		return nil
	}
	return ctx.ExcessShare
}

func evidenceInt32Ptr(v *int32) any {
	if v == nil {
		return nil
	}
	return *v
}

func primaryReason(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	return reasons[0]
}
