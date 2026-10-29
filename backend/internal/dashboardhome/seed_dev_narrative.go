package dashboardhome

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UpsertDevAdEfficiencyNarrative stores a dev-only dashboard narrative row with ad_efficiency_narrative fixture.
// Not for production use.
func UpsertDevAdEfficiencyNarrative(
	ctx context.Context,
	cfg *config.Config,
	pool *pgxpool.Pool,
	sellerAccountID int64,
	metricDate time.Time,
	adNarrative *AdEfficiencyNarrative,
) error {
	if adNarrative == nil {
		return fmt.Errorf("ad efficiency narrative fixture is required")
	}
	q := dbgen.New(pool)
	svc, err := BuildNarrativeService(cfg, pool)
	if err != nil {
		return fmt.Errorf("build narrative service: %w", err)
	}
	build, _, _, err := svc.buildNarrativeContextForSeller(ctx, sellerAccountID, metricDate)
	if err != nil {
		return fmt.Errorf("build narrative context: %w", err)
	}

	payload := minimalDevDashboardNarrativePayload(build)
	payload.AdEfficiencyNarrative = cloneAdEfficiencyNarrative(adNarrative)
	accepted, repaired, repair, hard := acceptGeneratedDashboardNarrative(payload, build)
	if !accepted || len(hard) > 0 {
		return fmt.Errorf("dev ad efficiency narrative payload rejected: hard=%v repair=%v", hard, repair.HardErrors)
	}
	validatedBytes, err := json.Marshal(repaired)
	if err != nil {
		return fmt.Errorf("marshal validated payload: %w", err)
	}

	if err := q.DeleteAIDashboardNarrativesBySellerMetricDate(ctx, dbgen.DeleteAIDashboardNarrativesBySellerMetricDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(metricDate),
	}); err != nil {
		return fmt.Errorf("delete existing narrative: %w", err)
	}

	row, err := q.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(metricDate),
		Status:          "running",
		HealthScore:     pgtype.Int4{Int32: build.Context.HealthScore, Valid: true},
		Severity:        pgtype.Text{String: build.Context.Severity, Valid: build.Context.Severity != ""},
		RiskDriversHash: build.ContextHash,
		ContextHash:     build.FactualContextHash,
		PromptVersion:   DashboardNarrativePromptVersion,
		Model:           pgtype.Text{String: "dev_fixture", Valid: true},
		StartedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert dev narrative row: %w", err)
	}

	_, err = q.CompleteAIDashboardNarrative(ctx, dbgen.CompleteAIDashboardNarrativeParams{
		ID:               row.ID,
		SellerAccountID:  sellerAccountID,
		Status:           "completed",
		RawAiResponse:    pgtype.Text{String: `{"seeded_by":"dev_fixture_ad_efficiency"}`, Valid: true},
		ValidatedPayload: validatedBytes,
		ValidationErrors: []byte("[]"),
		FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("complete dev narrative row: %w", err)
	}
	return nil
}

// DeleteDevDashboardNarrative removes stored dashboard narrative for a seller/date (dev reset / pending scenarios).
func DeleteDevDashboardNarrative(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, metricDate time.Time) error {
	return q.DeleteAIDashboardNarrativesBySellerMetricDate(ctx, dbgen.DeleteAIDashboardNarrativesBySellerMetricDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(metricDate),
	})
}

func minimalDevDashboardNarrativePayload(build dashboardNarrativeBuildResult) *dashboardNarrativePayload {
	payload := &dashboardNarrativePayload{
		Headline:           "Главный риск дня в рекламе.",
		ExecutiveSummary:   "Сильнее всего индекс давит реклама с низкой отдачей.",
		ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
		WhatChanged:        buildFallbackWhatChangedPayload(build.Context.WhatChangedContext),
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Остановите неэффективную рекламу",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу.",
				ExpectedEffect:    "Снизить риск рекламного перерасхода.",
			},
		},
	}
	if build.SellerPulseRequired {
		payload.SellerPulseNarrative = devSellerPulseNarrativeFixture()
	}
	return payload
}

func devSellerPulseNarrativeFixture() *SellerPulseNarrative {
	return &SellerPulseNarrative{
		Tone:     "stable",
		Headline: "Ключевые показатели в пределах ожидаемой динамики.",
		Summary:  "Продолжайте мониторинг продаж и рисков.",
		ImportantNotes: []SellerPulseImportantNote{
			{Text: "Критичных ухудшений по доступным метрикам не видно."},
		},
		QuickActions: []SellerPulseQuickAction{
			{Label: "Продолжить мониторинг ключевых SKU", Kind: "monitoring", Href: "/app/products"},
		},
	}
}

// DevAdEfficiencyNarrativeFixture returns the default dev fixture narrative for visual testing.
func DevAdEfficiencyNarrativeFixture() *AdEfficiencyNarrative {
	return &AdEfficiencyNarrative{
		Status:  "available",
		Summary: "Рост ДРР связан с несколькими кампаниями с расходом выше выручки и кампаниями, которые получили расход без заказов.",
		Checklist: []AdEfficiencyCheckItem{
			{
				Title:    "Проверить кампании с ДРР выше 100%",
				Text:     "Сравните ставки, релевантность запросов и страницы входа по кампаниям из списка.",
				Priority: "high",
				Target:   "high_drr_campaigns",
			},
			{
				Title:    "Разобрать кампании без заказов",
				Text:     "Проверьте показы, клики, CTR и соответствие посадочных страниц ожиданиям покупателя.",
				Priority: "high",
				Target:   "no_order_campaigns",
			},
			{
				Title:    "Сверить рекламу с конверсией карточек",
				Text:     "Если трафик есть, но заказов мало, проверьте цену, остатки, контент и отзывы.",
				Priority: "medium",
				Target:   "advertising_campaigns",
			},
			{
				Title:    "Проверить дневные лимиты и расписание показов",
				Text:     "Сверьте лимиты бюджета с часами, где конверсия стабильно выше, чтобы сократить лишний расход.",
				Priority: "medium",
				Target:   "advertising_campaigns",
			},
		},
		Limitations: []string{"Рекламный анализ построен на уровне кампаний."},
	}
}
