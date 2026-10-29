// Package dashboardhome builds GET /api/v1/dashboard/home.
//
// Deterministic metrics (seller pulse KPIs, stock coverage, ad efficiency, funnel, risk counts)
// are sourced from analytics tables — not from AI daily summaries. AI summaries are an
// interpretive layer (headlines, diagnostics text, recommended actions) merged via dedup.
package dashboardhome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	skuFocusLimit        = 6
	categoryPreviewLimit = 8
	topActionsLimit      = 3
	actionsFeedLimit     = 12
)

type Service struct {
	pool           *pgxpool.Pool
	q              *dbgen.Queries
	dashboard      *analytics.DashboardService
	criticalSKUs   *analytics.CriticalSKUService
	replenishment  *analytics.ReplenishmentService
	advertising    *analytics.AdvertisingService
	alerts         *alerts.Service
	resolver       publishedsnapshot.Resolver
	sourceResolver analysissource.Resolver
}

func NewService(
	pool *pgxpool.Pool,
	dashboard *analytics.DashboardService,
	criticalSKUs *analytics.CriticalSKUService,
	replenishment *analytics.ReplenishmentService,
	advertising *analytics.AdvertisingService,
	alertsSvc *alerts.Service,
) *Service {
	q := dbgen.New(pool)
	return &Service{
		pool:           pool,
		q:              q,
		dashboard:      dashboard,
		criticalSKUs:   criticalSKUs,
		replenishment:  replenishment,
		advertising:    advertising,
		alerts:         alertsSvc,
		resolver:       publishedsnapshot.NewSQLCResolver(q),
		sourceResolver: analysissource.NewResolver(q, analysissource.GlobalFlagsFromEnv()),
	}
}

func newDashboardResponseShellForMetrics() Response {
	return Response{
		Status:           StatusReady,
		DataQualityNotes: []string{},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("AI-сводка появится после обработки последних метрик.")},
			WhatChanged: WhatChangedWidget{
				WidgetBase: pendingWidget(""),
				Groups:     []WhatChangedGroup{},
			},
			TopActions:        []TopActionItem{},
			CategorySummaries: []CategorySummaryCard{},
			SalesFunnel:       SalesFunnelWidget{WidgetBase: funnelUnavailableWidget("")},
			FunnelAnalysis: FunnelAnalysisWidget{
				WidgetBase: WidgetBase{Status: WidgetNotEnoughData},
				AIAnalysis: pendingFunnelAIAnalysisWidget(),
			},
			StockCoverage:    StockCoverageWidget{WidgetBase: pendingWidget("")},
			AdEfficiency:     AdEfficiencyWidget{WidgetBase: pendingWidget("")},
			RiskDistribution: RiskDistributionWidget{WidgetBase: pendingWidget("")},
		},
		SellerPulse: SellerPulse{KPIs: []KPICard{}, Trend7D: []TrendDayPoint{}},
		SKUFocus: SKUFocus{
			OrderDrop:        []SKUFocusCard{},
			StockRisk:        []SKUFocusCard{},
			NoSalesWithStock: []SKUFocusCard{},
			Surplus:          []SKUFocusCard{},
			SlowTurnover:     []SKUFocusCard{},
			KeySKUs:          []KeySkuCard{},
		},
		RecommendedActions: []ActionItem{},
	}
}

// populateResponseForNarrativeHashes runs the same dashboard blocks as BuildHome
// so narrative context hashes match GET /dashboard/home freshness checks.
func (s *Service) populateResponseForNarrativeHashes(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	dashboardDTO analytics.DashboardMetricsDTO,
	resp *Response,
) {
	partial := &partialTracker{}
	if !dashboardAIOnlyEnabled() {
		s.runBlock("ai_diagnostics", func() { s.applyAIDiagnostics(ctx, sellerAccountID, metricDate, resp) })
	}
	s.runBlock("seller_pulse", func() { s.applySellerPulse(ctx, dashboardDTO, metricDate, resp) })
	s.runBlock("stock_coverage", func() { s.applyStockCoverage(dashboardDTO, resp) })
	s.runBlock("ad_efficiency", func() {
		s.applyAdEfficiency(ctx, sellerAccountID, metricDate, resp, partial)
	})
	s.runBlock("risk_distribution", func() {
		s.applyRiskDistribution(ctx, sellerAccountID, resp, partial)
	})
	s.runBlock("sales_funnel", func() {
		s.applySalesFunnel(ctx, sellerAccountID, metricDate, resp, partial, dashboardDTO)
	})
	s.runBlock("sku_focus", func() {
		s.applySKUFocus(ctx, sellerAccountID, metricDate, dashboardDTO, resp, partial)
	})
	s.reconcileAccountSummaryWithAds(resp)
	reconcileSurplusStockCoverageLabelsV2(resp)
	enrichSKUFocusSlowTurnover(resp)
	s.applyPartialNotes(resp, partial)
}

func (s *Service) buildNarrativeBuildResult(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	dashboardDTO analytics.DashboardMetricsDTO,
	resp *Response,
) (dashboardNarrativeBuildResult, int32, string, error) {
	recommendationsRows, err := s.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           20,
		Offset:          0,
	})
	if err != nil {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("open recommendations: %w", err)
	}
	alertRows, err := s.q.ListAlertsFiltered(ctx, dbgen.ListAlertsFilteredParams{
		SellerAccountID: sellerAccountID,
		Status:          pgtype.Text{String: "open", Valid: true},
		Severity:        pgtype.Text{},
		AlertGroup:      pgtype.Text{},
		EntityType:      pgtype.Text{},
		Limit:           30,
		Offset:          0,
	})
	if err != nil {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("open alerts: %w", err)
	}

	drivers := buildRiskDrivers(metricDate, dashboardDTO, resp, recommendationsRows, alertRows)
	score := computeHealthScoreBreakdown(drivers).FinalScore
	severity := computeHealthSeverity(score)
	actionPool := buildTopActionPool(nil, recommendationsRows, alertRows, resp.AIDiagnostics.AdEfficiency.Points, actionsFeedLimit)

	var previousSnapshot *accountMetricsSnapshot
	from := metricDate.UTC().AddDate(0, 0, -14)
	accRows, metricsErr := s.q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})
	if metricsErr == nil {
		previousSnapshot = findPreviousAccountSnapshot(accRows, metricDate)
	}

	build := buildDashboardNarrativeContext(metricDate, dashboardDTO, resp, drivers, score, severity, alertRows, actionPool, recommendationsRows, previousSnapshot)
	return build, score, severity, nil
}

func (s *Service) BuildHome(ctx context.Context, sellerAccountID int64) (Response, error) {
	resp := Response{
		Status:           StatusNotEnoughData,
		DataQualityNotes: []string{},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("AI-сводка появится после обработки последних метрик.")},
			WhatChanged: WhatChangedWidget{
				WidgetBase: pendingWidget(""),
				Groups:     []WhatChangedGroup{},
			},
			TopActions:        []TopActionItem{},
			CategorySummaries: []CategorySummaryCard{},
			SalesFunnel:       SalesFunnelWidget{WidgetBase: funnelUnavailableWidget("")},
			FunnelAnalysis: FunnelAnalysisWidget{
				WidgetBase: WidgetBase{Status: WidgetNotEnoughData},
				AIAnalysis: pendingFunnelAIAnalysisWidget(),
			},
			StockCoverage:    StockCoverageWidget{WidgetBase: pendingWidget("")},
			AdEfficiency:     AdEfficiencyWidget{WidgetBase: pendingWidget("")},
			RiskDistribution: RiskDistributionWidget{WidgetBase: pendingWidget("")},
		},
		SellerPulse: SellerPulse{KPIs: []KPICard{}, Trend7D: []TrendDayPoint{}},
		SKUFocus: SKUFocus{
			OrderDrop:        []SKUFocusCard{},
			StockRisk:        []SKUFocusCard{},
			NoSalesWithStock: []SKUFocusCard{},
			Surplus:          []SKUFocusCard{},
			SlowTurnover:     []SKUFocusCard{},
			KeySKUs:          []KeySkuCard{},
		},
		RecommendedActions: []ActionItem{},
	}

	source, err := s.sourceResolver.ResolveCurrent(ctx, sellerAccountID)
	if err != nil {
		slog.Error("dashboard home: resolve analysis source", "seller_account_id", sellerAccountID, "error", err)
		resp.DataQualityNotes = append(resp.DataQualityNotes, "Не удалось определить источник опубликованного анализа.")
		resp.AnalysisSource = analysissource.SourceNone
		resp.DashboardContentStatus = string(StatusNotEnoughData)
		return resp, nil
	}
	if source.Source == analysissource.SourceNone || source.PipelineRunID <= 0 {
		resp.AnalysisSource = analysissource.SourceNone
		resp.FallbackUsed = source.FallbackUsed
		resp.FallbackReason = source.FallbackReason
		resp.PublicationID = source.PublicationID
		resp.MetricDate = source.MetricDate
		resp.DashboardContentStatus = string(StatusAIGenerationRequired)
		resp.DataQualityNotes = append(resp.DataQualityNotes,
			"Опубликованный анализ ещё не готов. Запустите Unified Analysis или дождитесь публикации.")
		slog.Info("dashboard home source selected",
			"seller_id", sellerAccountID,
			"selected_source", analysissource.SourceNone,
			"publication_id", source.PublicationID,
			"fallback_used", source.FallbackUsed,
			"fallback_reason", source.FallbackReason,
		)
		return resp, nil
	}

	snapshot, err := s.resolver.GetPublishedSnapshotByPipelineID(ctx, sellerAccountID, source.PipelineRunID)
	if err != nil {
		slog.Error("dashboard home: published snapshot by pipeline",
			"seller_account_id", sellerAccountID,
			"pipeline_run_id", source.PipelineRunID,
			"error", err,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			resp.AnalysisSource = source.Source
			resp.PublicationID = source.PublicationID
			resp.FallbackUsed = source.FallbackUsed
			resp.FallbackReason = joinNonEmpty(source.FallbackReason, "published_snapshot_missing")
			resp.DashboardContentStatus = "error"
			resp.DataQualityNotes = append(resp.DataQualityNotes, "Не удалось загрузить опубликованный анализ.")
			return resp, nil
		}
		resp.DataQualityNotes = append(resp.DataQualityNotes, "Не удалось определить дату метрик для главной страницы.")
		resp.DashboardContentStatus = string(StatusNotEnoughData)
		return resp, nil
	}
	if err := publishedsnapshot.ValidateSnapshot(snapshot, sellerAccountID); err != nil {
		resp.AnalysisSource = source.Source
		resp.PublicationID = source.PublicationID
		resp.FallbackUsed = source.FallbackUsed
		resp.FallbackReason = joinNonEmpty(source.FallbackReason, "published_snapshot_incomplete")
		resp.DashboardContentStatus = "error"
		resp.DataQualityNotes = append(resp.DataQualityNotes, "Не удалось загрузить опубликованный анализ.")
		return resp, nil
	}
	metricDate := snapshot.MetricDate.UTC()
	resp.MetricDate = metricDate.Format("2006-01-02")
	if source.MetricDate != "" {
		resp.MetricDate = source.MetricDate
	}

	frozen, _, ferr := s.loadFrozenDashboardSnapshot(ctx, snapshot.PipelineRunID)
	if ferr != nil {
		slog.Error("dashboard home: frozen dashboard snapshot", "seller_account_id", sellerAccountID, "pipeline_run_id", snapshot.PipelineRunID, "error", ferr)
		resp.AnalysisSource = source.Source
		resp.PublicationID = source.PublicationID
		resp.FallbackUsed = source.FallbackUsed
		resp.FallbackReason = source.FallbackReason
		resp.DashboardContentStatus = "error"
		resp.DataQualityNotes = append(resp.DataQualityNotes, "Опубликованный снимок дашборда повреждён или неполон.")
		return resp, nil
	}
	if err := applyFrozenForAnalysisSource(&resp, frozen, source.Source); err != nil {
		resp.AnalysisSource = source.Source
		resp.PublicationID = source.PublicationID
		resp.FallbackUsed = source.FallbackUsed
		resp.FallbackReason = source.FallbackReason
		resp.DashboardContentStatus = "error"
		resp.DataQualityNotes = append(resp.DataQualityNotes, "Не удалось применить опубликованный снимок дашборда.")
		return resp, nil
	}
	s.enrichFrozenSKUFocusImages(ctx, sellerAccountID, &resp)
	dashboardDTO := frozen.DashboardMetrics

	s.runBlock("dashboard_narrative_apply", func() {
		s.applyStoredDashboardNarrativeToHome(ctx, sellerAccountID, metricDate, dashboardDTO, snapshot, &resp)
	})
	s.runBlock("seller_pulse_narrative", func() {
		finalizeSellerPulseKPITrends(&resp)
		applySellerPulseNarrativeToResponse(&resp, resp.AIDiagnostics.AccountSummary.narrativeBuild)
		finalizeSellerPulseUIContract(&resp)
	})
	finalizeSellerFacingDashboardResponse(&resp)
	enforceAIOnlyGenerationRequired(&resp)
	if shouldExposeNarrativeStaleDebug() {
		issues := ValidateDashboardHomeResponseConsistency(&resp)
		if dashboardAIOnlyEnabled() && resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" {
			var payload *dashboardNarrativePayload
			if stored := resp.AIDiagnostics.AccountSummary.storedNarrative; stored != nil {
				payload = stored.Payload
			}
			issues = append(issues, ValidateDashboardAIOnlyContent(payload, DashboardSynthesisInput{}, &resp)...)
		}
		resp.DashboardNarrativeDebug = buildDashboardNarrativeDebug(&resp, issues)
	}

	finalizeResponseTopActions(&resp)
	s.applyAnalysisSource(&resp, source, snapshot)
	resp.DashboardContentStatus = string(resp.Status)
	return resp, nil
}

func (s *Service) applyAnalysisSource(resp *Response, source analysissource.CurrentAnalysisSource, snapshot publishedsnapshot.Snapshot) {
	if resp == nil {
		return
	}
	resp.AnalysisSource = source.Source
	resp.PublicationID = source.PublicationID
	resp.FallbackUsed = source.FallbackUsed
	resp.FallbackReason = source.FallbackReason
	if resp.Pipeline == nil {
		resp.Pipeline = &DashboardPipelineInfo{}
	}
	if resp.Pipeline.PipelineRunID == 0 {
		resp.Pipeline.PipelineRunID = snapshot.PipelineRunID
	}
	if resp.Pipeline.ActionsRunID == 0 {
		resp.Pipeline.ActionsRunID = snapshot.ActionsRunID
	}
	if resp.Pipeline.DashboardRunID == "" {
		resp.Pipeline.DashboardRunID = snapshot.DashboardRunID
	}
	if resp.Pipeline.DataPackHash == "" {
		resp.Pipeline.DataPackHash = snapshot.DataPackHash
	}
	if resp.Pipeline.GlobalContextHash == "" {
		resp.Pipeline.GlobalContextHash = snapshot.GlobalContextHash
	}
	slog.Info("dashboard home source selected",
		"seller_id", snapshot.SellerAccountID,
		"analysis_mode", source.Source,
		"selected_source", source.Source,
		"publication_id", source.PublicationID,
		"run_id", snapshot.PipelineRunID,
		"metric_date", resp.MetricDate,
		"dashboard_content_status", resp.Status,
		"kpi_count", len(resp.SellerPulse.KPIs),
		"trend_count", len(resp.SellerPulse.Trend7D),
		"actions_count", len(resp.AIDiagnostics.TopActions),
		"fallback_used", source.FallbackUsed,
		"fallback_reason", source.FallbackReason,
	)
}

func joinNonEmpty(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func enforceAIOnlyGenerationRequired(resp *Response) {
	if !dashboardAIOnlyEnabled() || resp == nil {
		return
	}
	// Unified publication projections are authoritative once applied. Do not
	// replace them with the legacy v9 "AI dashboard not generated" placeholder.
	// StatusReady must still reflect merged factual content — never force ready
	// solely because a unified narrative row exists.
	if IsUnifiedPublicationPromptVersion(resp.AIDiagnostics.AccountSummary.PromptVersion) &&
		resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" {
		reconcileUnifiedPublicationStatus(resp)
		return
	}
	topActionsSource := ""
	if resp.Pipeline != nil {
		topActionsSource = strings.TrimSpace(resp.Pipeline.TopActionsSource)
	}
	topActionsReady := len(resp.AIDiagnostics.TopActions) >= 3
	switch topActionsSource {
	case "actions_final":
		topActionsReady = len(resp.AIDiagnostics.TopActions) > 0
	case "actions_empty":
		topActionsReady = true
	}
	ready := resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" &&
		resp.AIDiagnostics.AccountSummary.ValidationStatus == "completed" &&
		resp.AIDiagnostics.AccountSummary.HealthScore != nil &&
		len(resp.AIDiagnostics.AccountSummary.RiskDrivers) > 0 &&
		resp.SellerPulse.Narrative != nil && resp.SellerPulse.Narrative.Status == "available" &&
		resp.AIDiagnostics.AdEfficiency.Narrative != nil && resp.AIDiagnostics.AdEfficiency.Narrative.Status == "available" &&
		resp.AIDiagnostics.StockCoverage.Narrative != nil && resp.AIDiagnostics.StockCoverage.Narrative.Status == "available" &&
		resp.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status == WidgetAvailable &&
		len(resp.AIDiagnostics.CategorySummaries) > 0 &&
		topActionsReady
	if ready {
		return
	}
	resp.Status = StatusAIGenerationRequired
	resp.DataQualityNotes = []string{
		"AI dashboard not generated for current metric date. Run dashboard_narrative_v9_ai_only_chunks generation.",
	}
	resp.AIDiagnostics.AccountSummary.NarrativeSource = ""
	resp.AIDiagnostics.AccountSummary.ValidationStatus = "failed"
	if topActionsSource != "actions_final" && topActionsSource != "actions_empty" {
		resp.AIDiagnostics.TopActions = []TopActionItem{}
		resp.TopActions = []TopActionItem{}
	}
	resp.AIDiagnostics.CategorySummaries = []CategorySummaryCard{}
	resp.RecommendedActions = []ActionItem{}
	resp.SellerPulse.Narrative = &SellerPulseNarrative{Status: string(WidgetSourceNotAvailable), Headline: "", Summary: ""}
	resp.AIDiagnostics.AdEfficiency.Narrative = &AdEfficiencyNarrative{Status: string(WidgetSourceNotAvailable), Summary: "AI narrative is not ready yet."}
	resp.AIDiagnostics.StockCoverage.Narrative = &StockCoverageNarrative{Status: string(WidgetSourceNotAvailable), Summary: "AI narrative is not ready yet."}
	resp.AIDiagnostics.FunnelAnalysis.AIAnalysis = FunnelAIAnalysisWidget{
		WidgetBase:     WidgetBase{Status: WidgetSourceNotAvailable},
		MainConclusion: "AI funnel analysis is not ready yet.",
	}
	resp.AIDiagnostics.RiskDistribution = RiskDistributionWidget{WidgetBase: WidgetBase{Status: WidgetSourceNotAvailable}}
	resp.AIDiagnostics.WhatChanged = WhatChangedWidget{WidgetBase: WidgetBase{Status: WidgetSourceNotAvailable}, Groups: []WhatChangedGroup{}}
}

// reconcileUnifiedPublicationStatus sets Status from merged factual + AI content.
// A unified narrative alone must not force StatusReady when KPI/factual blocks are empty.
func reconcileUnifiedPublicationStatus(resp *Response) {
	if resp == nil {
		return
	}
	hasFactualKPIs := countSellerPulseFactualKPIs(resp.SellerPulse.KPIs) > 0
	hasTrend := len(resp.SellerPulse.Trend7D) > 0
	hasHeadline := strings.TrimSpace(resp.AIDiagnostics.AccountSummary.Headline) != ""
	hasSummary := strings.TrimSpace(resp.AIDiagnostics.AccountSummary.ExecutiveSummary) != ""
	switch {
	case hasFactualKPIs && (hasTrend || hasHeadline):
		if resp.Status != StatusPartial {
			resp.Status = StatusReady
		}
	case hasFactualKPIs || hasHeadline || hasSummary:
		resp.Status = StatusPartial
	case resp.Status == StatusReady:
		// Do not keep a ready mask over an empty factual layer.
		resp.Status = StatusNotEnoughData
	}
	_ = hasSummary
}

func countSellerPulseFactualKPIs(kpis []KPICard) int {
	n := 0
	for _, kpi := range kpis {
		key := strings.ToLower(strings.TrimSpace(kpi.Key))
		switch key {
		case "revenue", "orders", "average_order_value", "ad_spend_share_total_revenue", "ctr", "drr":
			if strings.TrimSpace(kpi.Value) != "" && !strings.EqualFold(kpi.Value, "—") && !strings.EqualFold(kpi.Value, "-") {
				n++
			}
		}
	}
	return n
}

func (s *Service) applyPartialNotes(resp *Response, partial *partialTracker) {
	if partial == nil || !partial.any() {
		return
	}
	resp.Status = StatusPartial
	resp.DataQualityNotes = append(resp.DataQualityNotes, buildPartialDataQualityNotes(partial.reasons)...)
}

func pendingWidget(msg string) WidgetBase {
	if msg == "" {
		return WidgetBase{Status: WidgetNotEnoughData}
	}
	return WidgetBase{Status: WidgetSourcePending, Message: msg}
}

func funnelUnavailableWidget(msg string) WidgetBase {
	if msg == "" {
		msg = "Данные для воронки пока не подключены. Для построения воронки нужны метрики показов, переходов, корзин и заказов из Ozon analytics/data."
	}
	return WidgetBase{Status: WidgetSourceNotAvailable, Message: msg}
}

// applyAIDiagnostics merges category cards and funnel AI text from ai_daily_summaries.
// Block ai_diagnostics.what_changed is NOT sourced from daily summaries; it is filled in
// applyUnifiedAccountHealth from the validated AI dashboard narrative (or deterministic fallback).
func (s *Service) applyAIDiagnostics(ctx context.Context, sellerID int64, metricDate time.Time, resp *Response) {
	dashboardDate := dateValue(metricDate)
	rows, err := s.q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      dashboardDate,
	})
	if err != nil || len(rows) == 0 {
		if latest, lerr := s.q.GetLatestAIDailySummaryMetricDate(ctx, sellerID); lerr == nil && latest.Valid && latest.Time.Year() > 2000 {
			latestKey := latest.Time.UTC().Format("2006-01-02")
			dashboardKey := metricDate.UTC().Format("2006-01-02")
			if latestKey != dashboardKey {
				resp.AIDiagnostics.AccountSummary = AccountSummaryWidget{
					WidgetBase: pendingWidget(
						fmt.Sprintf("AI-сводка за %s ещё не готова (последняя доступная: %s). Ориентируйтесь на метрики и рекламу ниже.", dashboardKey, latestKey),
					),
					Freshness:  "stale_metric_date",
					MetricDate: latestKey,
				}
			}
		}
		return
	}

	var debug *AISummariesDebug
	if shouldExposeAISummariesDebug() {
		debug = &AISummariesDebug{}
		if run, rerr := s.q.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
			SellerAccountID: sellerID,
			MetricDate:      dashboardDate,
		}); rerr == nil {
			if run.ID.Valid {
				debug.RunID = run.ID.String()
			}
			debug.RunStatus = run.Status
		}
	}

	for _, row := range rows {
		if row.SummaryType != "account_daily_summary" {
			continue
		}
		stored := storedRowFromDB(row)
		payload, outcome, ok := acceptStoredAccountSummaryPayload(stored)
		if debug != nil {
			debug.AccountSummaryID = stored.ID
			debug.AccountStatus = stored.Status
			if ok {
				debug.AccountValidation = outcome.ValidationStatus
				debug.FunnelRepairable = outcome.Repair.RepairableErrors
			}
		}
		if ok {
			widget, fallbackReason := funnelAIAnalysisFromAcceptedPayload(payload, outcome)
			resp.AIDiagnostics.FunnelAnalysis.AIAnalysis = widget
			if debug != nil {
				debug.FunnelFallback = fallbackReason
			}
		} else {
			var raw aisummaries.SummaryPayload
			if len(stored.SummaryJSON) > 0 {
				_ = json.Unmarshal(stored.SummaryJSON, &raw)
			}
			resp.AIDiagnostics.FunnelAnalysis.AIAnalysis = funnelAIAnalysisWidgetFromPayload(raw)
			if debug != nil {
				debug.FunnelFallback = firstNonEmpty(strings.Join(outcome.Repair.HardErrors, ","), "account_summary_not_accepted")
			}
		}
	}

	cats := make([]CategorySummaryCard, 0)
	catDebug := make([]CategoryDebugRow, 0)
	for _, row := range rows {
		if row.SummaryType != "category_daily_summary" {
			continue
		}
		stored := storedRowFromDB(row)
		fallbackName := ""
		if row.ScopeName.Valid {
			fallbackName = row.ScopeName.String
		}
		card, rowDebug := buildCategorySummaryCard(stored, fallbackName)
		cats = append(cats, card)
		if debug != nil {
			catDebug = append(catDebug, CategoryDebugRow{
				ScopeID:          rowDebug.ScopeID,
				CategoryName:     rowDebug.CategoryName,
				Status:           rowDebug.Status,
				ValidationStatus: rowDebug.ValidationStatus,
				RepairableErrors: rowDebug.RepairableErrors,
				FallbackReason:   rowDebug.FallbackReason,
				Accepted:         rowDebug.Accepted,
			})
		}
	}
	sortCategoryCards(cats)
	if len(cats) > categoryPreviewLimit {
		cats = cats[:categoryPreviewLimit]
	}
	resp.AIDiagnostics.CategorySummaries = cats
	if debug != nil {
		debug.CategoryRows = catDebug
		resp.AIDiagnostics.AISummariesDebug = debug
	}
}

func buildAIActionsFromPayload(payload aisummaries.SummaryPayload, limit int) []ActionItem {
	out := make([]ActionItem, 0, limit)
	for _, a := range payload.RecommendedActions {
		if strings.TrimSpace(a.Action) == "" {
			continue
		}
		out = append(out, ActionItem{
			Priority:          a.Priority,
			UrgencyGroup:      urgencyGroupForPriority(a.Priority),
			Action:            a.Action,
			Reason:            a.Reason,
			ExpectedEffect:    a.ExpectedEffect,
			RelatedEntityType: a.RelatedEntityType,
			RelatedEntityID:   a.RelatedEntityID,
			Source:            "ai_summary",
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func pendingFunnelAIAnalysisWidget() FunnelAIAnalysisWidget {
	return FunnelAIAnalysisWidget{
		WidgetBase:       pendingWidget("AI-анализ воронки появится после генерации дневной AI-сводки."),
		RelatedSignals:   []string{},
		RecommendedFocus: []string{},
	}
}

func missingFunnelAIAnalysisWidget() FunnelAIAnalysisWidget {
	return FunnelAIAnalysisWidget{
		WidgetBase: WidgetBase{
			Status:  WidgetNotEnoughData,
			Message: "AI-анализ воронки пока не сформирован. Запустите генерацию дневной AI-сводки.",
		},
		RelatedSignals:   []string{},
		RecommendedFocus: []string{},
	}
}

func funnelAIAnalysisWidgetFromPayload(payload aisummaries.SummaryPayload) FunnelAIAnalysisWidget {
	if payload.FunnelAIAnalysis == nil {
		return missingFunnelAIAnalysisWidget()
	}

	status := widgetStatusOrDefault(payload.FunnelAIAnalysis.Status, WidgetAvailable)
	widget := FunnelAIAnalysisWidget{
		WidgetBase:       WidgetBase{Status: status},
		ShortSummary:     strings.TrimSpace(payload.FunnelAIAnalysis.ShortSummary),
		Summary:          strings.TrimSpace(payload.FunnelAIAnalysis.Summary),
		TrendVsYesterday: strings.TrimSpace(payload.FunnelAIAnalysis.TrendVsYesterday),
		TrendVs7D:        strings.TrimSpace(payload.FunnelAIAnalysis.TrendVs7D),
		RelatedSignals:   stringSliceOrEmpty(payload.FunnelAIAnalysis.RelatedSignals),
		RecommendedFocus: stringSliceOrEmpty(payload.FunnelAIAnalysis.RecommendedFocus),
		FullAnalysis:     strings.TrimSpace(payload.FunnelAIAnalysis.FullAnalysis),
		Details:          strings.TrimSpace(payload.FunnelAIAnalysis.Details),
		MainConclusion:   strings.TrimSpace(payload.FunnelAIAnalysis.MainConclusion),
		ActionItems:      stringSliceOrEmpty(payload.FunnelAIAnalysis.ActionItems),
		DropFactors:      mapFunnelDropFactors(payload.FunnelAIAnalysis.DropFactors),
		CheckDirections:  mapFunnelTextItems(payload.FunnelAIAnalysis.CheckDirections),
		DetailsSections:  mapFunnelDetailsSections(payload.FunnelAIAnalysis.DetailsSections),
		InfluenceFactors: mapFunnelTextItems(payload.FunnelAIAnalysis.InfluenceFactors),
	}
	if payload.FunnelAIAnalysis.ComparisonsCommentary != nil {
		widget.ComparisonsCommentary = FunnelComparisonsCommentary{
			PreviousPeriod:  strings.TrimSpace(payload.FunnelAIAnalysis.ComparisonsCommentary.PreviousPeriod),
			SevenDayAverage: strings.TrimSpace(payload.FunnelAIAnalysis.ComparisonsCommentary.SevenDayAverage),
		}
	}
	if payload.FunnelAIAnalysis.WeakestStage != nil {
		ws := payload.FunnelAIAnalysis.WeakestStage
		explanation := strings.TrimSpace(ws.Explanation)
		if explanation == "" {
			explanation = strings.TrimSpace(ws.Reason)
		}
		widget.WeakestStage = FunnelWeakestStage{
			Status:        strings.TrimSpace(ws.Status),
			Key:           strings.TrimSpace(ws.Key),
			Label:         strings.TrimSpace(ws.Label),
			Metric:        strings.TrimSpace(ws.Metric),
			Value:         strings.TrimSpace(ws.Value),
			Explanation:   explanation,
			Reason:        explanation,
			DeltaPrevious: strings.TrimSpace(ws.DeltaPrevious),
			Delta7D:       strings.TrimSpace(ws.Delta7D),
			Tone:          strings.TrimSpace(ws.Tone),
		}
	}
	if payload.FunnelAIAnalysis.Comparisons != nil {
		widget.Comparisons = FunnelComparisons{
			PreviousPeriod:    mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.PreviousPeriod),
			SevenDayAverage:   mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.SevenDayAverage),
			StageMetrics:      mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.StageMetrics),
			ConversionMetrics: mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.ConversionMetrics),
			BusinessMetrics:   mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.BusinessMetrics),
			ContextSignals:    mapFunnelComparisonItems(payload.FunnelAIAnalysis.Comparisons.ContextSignals),
		}
	}
	if status == WidgetAvailable && !funnelWidgetHasPrimaryAIContent(&widget) {
		return missingFunnelAIAnalysisWidget()
	}
	if status != WidgetAvailable {
		widget.Message = firstNonEmpty(
			widget.FullAnalysis,
			widget.ShortSummary,
			defaultFunnelAIAnalysisMessage(status),
		)
	}
	return widget
}

func defaultFunnelAIAnalysisMessage(status WidgetStatus) string {
	switch status {
	case WidgetSourcePending:
		return "AI-анализ воронки ещё формируется."
	case WidgetSourceNotAvailable:
		return "AI-анализ воронки недоступен, потому что нет исходных данных по воронке."
	case WidgetError:
		return "Не удалось подготовить AI-анализ воронки."
	default:
		return "AI-анализ воронки пока недоступен."
	}
}

func widgetStatusOrDefault(raw string, fallback WidgetStatus) WidgetStatus {
	switch WidgetStatus(strings.TrimSpace(raw)) {
	case WidgetAvailable, WidgetNotEnoughData, WidgetSourceNotAvailable, WidgetSourceAccessDenied, WidgetPremiumRequired, WidgetSourcePending, WidgetError:
		return WidgetStatus(strings.TrimSpace(raw))
	default:
		return fallback
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func sortCategoryCards(cards []CategorySummaryCard) {
	sevRank := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sort.SliceStable(cards, func(i, j int) bool {
		ri := sevRank[strings.ToLower(cards[i].Severity)]
		rj := sevRank[strings.ToLower(cards[j].Severity)]
		if ri != rj {
			return ri < rj
		}
		hi := int32(0)
		hj := int32(0)
		if cards[i].HealthScore != nil {
			hi = *cards[i].HealthScore
		}
		if cards[j].HealthScore != nil {
			hj = *cards[j].HealthScore
		}
		return hi < hj
	})
}

func (s *Service) reconcileAccountSummaryWithAds(resp *Response) {
	w := &resp.AIDiagnostics.AccountSummary
	if w.Status != WidgetAvailable || w.Freshness != "current" {
		return
	}
	ads := resp.AIDiagnostics.AdEfficiency
	if ads.Status != WidgetAvailable || !aiSummaryContradictsAdEfficiency(w.Headline, w.ExecutiveSummary, ads.Points) {
		return
	}
	w.Freshness = "stale_relative_to_ads"
	w.Headline = ""
	w.ExecutiveSummary = ""
	w.Message = "Текст AI-сводки не совпадает с актуальными рекламными метриками ниже. Ориентируйтесь на блок «Эффективность рекламы»."
}

func (s *Service) applySellerPulse(ctx context.Context, dto analytics.DashboardMetricsDTO, metricDate time.Time, resp *Response) {
	acc := dto.Account
	from := metricDate.AddDate(0, 0, -6)
	rows, _ := s.q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: dto.SellerAccountID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})

	accountDays := make([]accountMetricDay, 0, len(rows))
	revByDate := make(map[string]float64)
	ordByDate := make(map[string]float64)
	for _, row := range rows {
		d := row.MetricDate.Time.Format("2006-01-02")
		rev := numericToFloat(row.Revenue)
		ord := row.OrdersCount
		accountDays = append(accountDays, accountMetricDay{Date: d, Revenue: rev, Orders: ord})
		revByDate[d] = rev
		ordByDate[d] = float64(ord)
	}
	trend := buildAccountTrend7D(from, metricDate, accountDays)
	sparkRevenue := buildCalendarSparklineSeries(from, metricDate, revByDate)
	sparkOrders := buildCalendarSparklineSeries(from, metricDate, ordByDate)
	metricsByDate := make(map[string]sparklineDayMetrics, len(accountDays))
	for _, day := range accountDays {
		metricsByDate[day.Date] = sparklineDayMetrics{
			orders:     float64(day.Orders),
			revenue:    day.Revenue,
			hasOrders:  true,
			hasRevenue: true,
		}
	}
	sparkDRR := s.buildAccountDRRSparkline(ctx, dto.SellerAccountID, from, metricDate, revByDate)
	sparkCTR := s.loadAccountCTRSparkline(ctx, dto.SellerAccountID, from, metricDate)

	ordersToday := acc.OrdersToday
	ordersYesterday := acc.OrdersYesterday
	if ord, ok := accountOrdersForDate(accountDays, metricDate); ok {
		ordersToday = ord
	}
	yesterday := metricDate.AddDate(0, 0, -1)
	if ord, ok := accountOrdersForDate(accountDays, yesterday); ok {
		ordersYesterday = ord
	}

	drr := acc.AccountAdSpendToRevenue

	var revenueDeltaPct *float64
	if acc.RevenueDayDelta.Pct != nil {
		v := ratioToPercent(acc.RevenueDayDelta.Pct)
		revenueDeltaPct = &v
	}
	ordersDeltaPct := intChangePercent(int32(ordersToday), int32(ordersYesterday))
	ordersDeltaPctPtr := &ordersDeltaPct

	kpis := []KPICard{
		{
			Key: "revenue", Label: "Выручка", Value: formatMoney(acc.RevenueToday),
			Delta: &KPICardDelta{
				Abs: acc.RevenueDayDelta.Abs, Pct: revenueDeltaPct, Label: "к вчера",
			},
			Sparkline7D: sparkRevenue, Hint: "За отчётный день",
		},
		{
			Key: "orders", Label: "Заказы", Value: formatInt(int64(ordersToday)),
			Delta: &KPICardDelta{
				Abs: float64(ordersToday - ordersYesterday), Pct: ordersDeltaPctPtr, Label: "к вчера",
			},
			Sparkline7D: sparkOrders, Hint: "За отчётный день",
		},
	}
	if ctrRow, ctrErr := s.q.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: dto.SellerAccountID,
		MetricDate:      dateValue(metricDate),
		ScopeType:       "account",
		ScopeID:         "",
	}); ctrErr == nil {
		prevCTRRow := loadPreviousAvailableProductAnalyticsRow(ctx, s.q, dto.SellerAccountID, metricDate)
		if ctrKPI, ok := buildCTRKPI(ctrRow, prevCTRRow, sparkCTR); ok {
			kpis = append(kpis, ctrKPI)
		}
	}
	if drr != nil {
		pct := *drr * 100
		kpis = append(kpis, KPICard{
			Key: "drr", Label: "ДРР в общей выручке", Value: fmt.Sprintf("%.1f%%", pct),
			Sparkline7D: sparkDRR, Hint: "Рекламные расходы / общая выручка кабинета",
		})
	}

	alertSummary, err := s.alerts.GetSummary(ctx, dto.SellerAccountID)
	if err == nil {
		critHigh := int64(0)
		for _, sc := range alertSummary.BySeverity {
			if sc.Severity == alerts.SeverityCritical || sc.Severity == alerts.SeverityHigh {
				critHigh += sc.Count
			}
		}
		kpis = append(kpis, KPICard{
			Key: "risks", Label: "Критичные риски", Value: formatInt(critHigh),
			Hint: "Открытые алерты critical/high",
		})
	}

	resp.SellerPulse = SellerPulse{KPIs: kpis, Trend7D: trend}
}

func (s *Service) buildAccountDRRSparkline(
	ctx context.Context,
	sellerID int64,
	from, to time.Time,
	revenueByDate map[string]float64,
) []SparklinePoint {
	adRows, err := s.q.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        dateValue(from),
		DateTo:          dateValue(to),
	})
	if err != nil || len(adRows) == 0 {
		return []SparklinePoint{}
	}
	spendByDate := make(map[string]float64, len(adRows))
	for _, row := range adRows {
		d := row.MetricDate.Time.Format("2006-01-02")
		spendByDate[d] += numericToFloat(row.Spend)
	}
	ratioByDate := make(map[string]dailyRatioPoint, 7)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		rev, hasRev := revenueByDate[key]
		spend, hasSpend := spendByDate[key]
		if hasRev && hasSpend && rev > 0 {
			v := spend / rev * 100
			ratioByDate[key] = dailyRatioPoint{value: v, hasData: true}
		}
	}
	return buildRatioSparklineSeries(from, to, ratioByDate)
}

func (s *Service) applyStockCoverage(dto analytics.DashboardMetricsDTO, resp *Response) {
	buckets := map[string]*StockCoverageSegment{
		"critical_0_3":   {Key: "critical_0_3", Label: "0–3 дня", Severity: "critical"},
		"shortage_4_7":   {Key: "shortage_4_7", Label: "4–7 дней", Severity: "high"},
		"planning_8_14":  {Key: "planning_8_14", Label: "8–14 дней", Severity: "medium"},
		"healthy_14plus": {Key: "healthy_14plus", Label: ">14 дней", Severity: "low"},
		"surplus":        {Key: "surplus", Label: "SURPLUS (Ozon оборачиваемость)", Severity: "high"},
	}
	total := 0
	for _, sku := range dto.SKUs {
		total++
		key := StockCoverageBucketKey(sku)
		if seg, ok := buckets[key]; ok {
			seg.SKUCount++
		}
	}
	segments := make([]StockCoverageSegment, 0, 5)
	for _, k := range []string{"critical_0_3", "shortage_4_7", "planning_8_14", "healthy_14plus", "surplus"} {
		segments = append(segments, *buckets[k])
	}
	status := WidgetAvailable
	msg := ""
	if total == 0 {
		status = WidgetNotEnoughData
		msg = "Нет SKU-метрик для оценки покрытия остатков."
	}
	resp.AIDiagnostics.StockCoverage = StockCoverageWidget{
		WidgetBase: WidgetBase{Status: status, Message: msg},
		Segments:   segments,
		TotalSKUs:  total,
	}
}

func (s *Service) applyAdEfficiency(ctx context.Context, sellerID int64, metricDate time.Time, resp *Response, partial *partialTracker) {
	dto, err := s.advertising.BuildAdEfficiencyDashboard(ctx, sellerID, analytics.AdvertisingQuery{}, metricDate)
	if err != nil {
		partial.mark(partialReasonAdvertising)
		resp.AIDiagnostics.AdEfficiency = errorAdEfficiencyWidget("")
		resp.AIDiagnostics.AdEfficiency.Narrative = &AdEfficiencyNarrative{Status: "unavailable"}
		return
	}
	if !dto.HasData {
		partial.mark(partialReasonAdvertising)
		resp.AIDiagnostics.AdEfficiency = emptyAdEfficiencyWidget("")
		resp.AIDiagnostics.AdEfficiency.Narrative = &AdEfficiencyNarrative{Status: "not_enough_data"}
		return
	}
	resp.AIDiagnostics.AdEfficiency = mapAdEfficiencyDashboardDTO(dto)
}

func (s *Service) applyRiskDistribution(ctx context.Context, sellerID int64, resp *Response, partial *partialTracker) {
	openAlerts, err := s.q.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerID, Limit: 500, Offset: 0,
	})
	if err != nil {
		partial.mark(partialReasonRiskDistribution)
		resp.AIDiagnostics.RiskDistribution = RiskDistributionWidget{
			WidgetBase: WidgetBase{Status: WidgetError, Message: "Не удалось загрузить распределение рисков."},
			Groups:     []RiskGroupCount{},
		}
		return
	}
	bucketCounts := map[string]int64{
		"stock":        0,
		"surplus":      0,
		"advertising":  0,
		"sales":        0,
		"critical_sku": 0,
		"other":        0,
	}
	bucketMeta := map[string]RiskGroupCount{
		"stock":        {Group: "stock", Label: "Остатки", Severity: "high"},
		"surplus":      {Group: "surplus", Label: "Излишки", Severity: "medium"},
		"advertising":  {Group: "advertising", Label: "Реклама", Severity: "medium"},
		"sales":        {Group: "sales", Label: "Просадка заказов", Severity: "high"},
		"critical_sku": {Group: "critical_sku", Label: "Критические SKU", Severity: "critical"},
		"other":        {Group: "other", Label: "Прочее", Severity: "low"},
	}
	for _, a := range openAlerts {
		if alerts.IsLegacyAlertType(alerts.AlertType(a.AlertType)) {
			continue
		}
		bucket := alerts.AlertRiskBucket(alerts.AlertType(a.AlertType), alerts.AlertGroup(a.AlertGroup))
		bucketCounts[bucket]++
	}
	groups := make([]RiskGroupCount, 0, len(bucketCounts))
	var total int64
	for _, k := range []string{"stock", "surplus", "advertising", "sales", "critical_sku", "other"} {
		if bucketCounts[k] <= 0 {
			continue
		}
		g := bucketMeta[k]
		g.Count = bucketCounts[k]
		groups = append(groups, g)
		total += g.Count
	}
	status := WidgetAvailable
	msg := ""
	if total == 0 {
		status = WidgetNotEnoughData
		msg = "Открытых рисков не обнаружено."
	}
	resp.AIDiagnostics.RiskDistribution = RiskDistributionWidget{
		WidgetBase: WidgetBase{Status: status, Message: msg},
		Groups:     groups,
		Total:      total,
	}
}

func (s *Service) applySalesFunnel(ctx context.Context, sellerID int64, metricDate time.Time, resp *Response, partial *partialTracker, dto analytics.DashboardMetricsDTO) {
	row, err := s.q.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(metricDate),
		ScopeType:       "account",
		ScopeID:         "",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			partial.mark(partialReasonSalesFunnel)
			resp.AIDiagnostics.SalesFunnel = SalesFunnelWidget{WidgetBase: funnelUnavailableWidget("")}
			resp.AIDiagnostics.FunnelAnalysis = FunnelAnalysisWidget{
				WidgetBase: WidgetBase{
					Status:  WidgetSourceNotAvailable,
					Message: "Данных для воронки пока недостаточно. Когда будут доступны показы, переходы и корзины, здесь появится анализ потерь на этапах продаж.",
				},
			}
			return
		}
		partial.mark(partialReasonSalesFunnel)
		resp.AIDiagnostics.SalesFunnel = SalesFunnelWidget{WidgetBase: WidgetBase{Status: WidgetError, Message: "Ошибка загрузки воронки."}}
		return
	}

	prevRow := loadPreviousAvailableProductAnalyticsRow(ctx, s.q, sellerID, metricDate)
	funnelWidget, analysisWidget := funnelWidgetsFromRows(row, prevRow)
	analysisWidget.AIAnalysis = resp.AIDiagnostics.FunnelAnalysis.AIAnalysis
	ev := buildFunnelEvidenceSnapshot(resp, dto)
	historyRows, histErr := s.q.ListProductAnalyticsDailyBySellerAndDateRange(ctx, dbgen.ListProductAnalyticsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(metricDate.AddDate(0, 0, -6)),
		MetricDate_2:    dateValue(metricDate),
	})
	var funnelHist []dbgen.ProductAnalyticsDaily
	if histErr == nil {
		funnelHist = historyRows
	}
	if len(funnelWidget.Stages) > 0 && analysisWidget.Status == WidgetAvailable && !dashboardAIOnlyEnabled() {
		baselineAware := analyzeFunnel(funnelWidget.Stages, funnelHist)
		analysisWidget.PrimaryDropStage = baselineAware.stage
		analysisWidget.PrimaryDropMessage = baselineAware.message
	} else if dashboardAIOnlyEnabled() {
		analysisWidget.PrimaryDropStage = ""
		analysisWidget.PrimaryDropMessage = ""
	}
	analysisWidget.AIAnalysis = applyFunnelAIGuards(analysisWidget.AIAnalysis, funnelWidget.Stages, ev, funnelHist, &row, prevRow, resp.AIDiagnostics.TopActions)
	resp.AIDiagnostics.SalesFunnel = funnelWidget
	resp.AIDiagnostics.FunnelAnalysis = analysisWidget
	if funnelWidget.Status != WidgetAvailable {
		partial.mark(partialReasonSalesFunnel)
	}
}

func funnelWidgetHasPrimaryAIContent(w *FunnelAIAnalysisWidget) bool {
	if w == nil {
		return false
	}
	if strings.TrimSpace(w.WeakestStage.Status) != "" {
		return true
	}
	if strings.TrimSpace(w.MainConclusion) != "" {
		return true
	}
	if len(w.DropFactors) > 0 || len(w.CheckDirections) > 0 || len(w.DetailsSections) > 0 {
		return true
	}
	if len(w.InfluenceFactors) > 0 {
		return true
	}
	return strings.TrimSpace(w.Summary) != "" ||
		strings.TrimSpace(w.ShortSummary) != "" ||
		strings.TrimSpace(w.FullAnalysis) != "" ||
		strings.TrimSpace(w.Details) != ""
}

func mapFunnelDropFactors(items []aisummaries.FunnelDropFactor) []FunnelDropFactor {
	out := make([]FunnelDropFactor, 0, len(items))
	for _, item := range items {
		explanation := strings.TrimSpace(item.Explanation)
		out = append(out, FunnelDropFactor{
			Key: strings.TrimSpace(item.Key), Label: strings.TrimSpace(item.Label),
			Group:         strings.TrimSpace(item.Group),
			EvidenceLevel: strings.TrimSpace(item.EvidenceLevel),
			Metric:        strings.TrimSpace(item.Metric), Delta: strings.TrimSpace(item.Delta), Explanation: explanation,
			Limitation: strings.TrimSpace(item.Limitation),
			Tone:       strings.TrimSpace(item.Tone), EvidenceKey: strings.TrimSpace(item.EvidenceKey),
		})
	}
	return out
}

func mapFunnelDetailsSections(sections []aisummaries.FunnelDetailsSection) []FunnelDetailsSection {
	out := make([]FunnelDetailsSection, 0, len(sections))
	for _, sec := range sections {
		items := make([]FunnelDetailsSectionItem, 0, len(sec.Items))
		for _, it := range sec.Items {
			items = append(items, FunnelDetailsSectionItem{
				Text: strings.TrimSpace(it.Text), Metrics: stringSliceOrEmpty(it.Metrics),
			})
		}
		out = append(out, FunnelDetailsSection{
			Title: strings.TrimSpace(sec.Title), Items: items,
		})
	}
	return out
}

func mapFunnelComparisonItems(items []aisummaries.FunnelComparisonItem) []FunnelComparisonItem {
	out := make([]FunnelComparisonItem, 0, len(items))
	for _, item := range items {
		out = append(out, FunnelComparisonItem{
			Key: strings.TrimSpace(item.Key), Label: strings.TrimSpace(item.Label),
			Value: strings.TrimSpace(item.Value), Delta: strings.TrimSpace(item.Delta), Tone: strings.TrimSpace(item.Tone),
		})
	}
	return out
}

func mapFunnelTextItems(items []aisummaries.FunnelTextItem) []FunnelTextItem {
	out := make([]FunnelTextItem, 0, len(items))
	for _, item := range items {
		out = append(out, FunnelTextItem{
			Key: strings.TrimSpace(item.Key), Label: strings.TrimSpace(item.Label),
			Priority:   strings.TrimSpace(item.Priority),
			DataNeeded: strings.TrimSpace(item.DataNeeded),
			Reason:     strings.TrimSpace(item.Reason),
			Text:       strings.TrimSpace(item.Text), Tone: strings.TrimSpace(item.Tone),
		})
	}
	return out
}

func funnelWidgetsFromRow(row dbgen.ProductAnalyticsDaily) (SalesFunnelWidget, FunnelAnalysisWidget) {
	return funnelWidgetsFromRows(row, nil)
}

func funnelWidgetsFromRows(row dbgen.ProductAnalyticsDaily, prev *dbgen.ProductAnalyticsDaily) (SalesFunnelWidget, FunnelAnalysisWidget) {
	if status, msg, ok := funnelAccessStatusFromRow(row); ok {
		w := SalesFunnelWidget{WidgetBase: WidgetBase{Status: status, Message: msg}}
		return w, FunnelAnalysisWidget{WidgetBase: WidgetBase{Status: status, Message: msg}}
	}

	stages := buildFunnelStages(row, prev)
	if len(stages) == 0 {
		msg := "Данных для воронки пока недостаточно. Когда будут доступны показы, переходы и корзины, здесь появится анализ потерь на этапах продаж."
		if row.SourceMessage.Valid && row.SourceMessage.String != "" {
			msg = row.SourceMessage.String
		}
		status := WidgetNotEnoughData
		if row.SourceStatus.Valid {
			switch row.SourceStatus.String {
			case "source_not_available":
				status = WidgetSourceNotAvailable
			case "not_enough_data":
				status = WidgetNotEnoughData
			}
		}
		return SalesFunnelWidget{WidgetBase: WidgetBase{Status: status, Message: msg}},
			FunnelAnalysisWidget{WidgetBase: WidgetBase{Status: status, Message: msg}}
	}

	analysis := analyzeFunnel(stages, nil)
	return SalesFunnelWidget{
			WidgetBase: WidgetBase{Status: WidgetAvailable},
			Stages:     stages,
		}, FunnelAnalysisWidget{
			WidgetBase:         WidgetBase{Status: WidgetAvailable},
			PrimaryDropStage:   analysis.stage,
			PrimaryDropMessage: analysis.message,
		}
}

func funnelAccessStatusFromRow(row dbgen.ProductAnalyticsDaily) (WidgetStatus, string, bool) {
	if !row.SourceStatus.Valid || row.SourceStatus.String == "" || row.SourceStatus.String == "available" {
		return "", "", false
	}
	msg := ""
	if row.SourceMessage.Valid {
		msg = row.SourceMessage.String
	}
	switch row.SourceStatus.String {
	case "source_access_denied":
		if msg == "" {
			msg = "Данные воронки недоступны для текущего кабинета или тарифа Ozon."
		}
		return WidgetSourceAccessDenied, msg, true
	case "premium_required":
		if msg == "" {
			msg = "Данные воронки доступны на расширенном тарифе Ozon (Premium)."
		}
		return WidgetPremiumRequired, msg, true
	case "not_enough_data":
		if msg == "" {
			msg = "Данных для воронки пока недостаточно."
		}
		return WidgetNotEnoughData, msg, true
	case "error":
		if msg == "" {
			msg = "Не удалось загрузить данные воронки."
		}
		return WidgetError, msg, true
	default:
		return WidgetSourceNotAvailable, msg, true
	}
}

func loadPreviousAvailableProductAnalyticsRow(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate time.Time) *dbgen.ProductAnalyticsDaily {
	prevDate := metricDate.UTC().AddDate(0, 0, -1)
	row, err := q.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(prevDate),
		ScopeType:       "account",
		ScopeID:         "",
	})
	if err != nil {
		return nil
	}
	if status, _, blocked := funnelAccessStatusFromRow(row); blocked && status != WidgetAvailable {
		return nil
	}
	return &row
}

func buildFunnelStages(row dbgen.ProductAnalyticsDaily, prev *dbgen.ProductAnalyticsDaily) []FunnelStage {
	labels := []struct {
		key     string
		label   string
		val     *int64
		prevVal *int64
	}{
		{"impressions", "Показы", pgInt8Ptr(row.Impressions), productAnalyticsMetricPtr(prev, "impressions")},
		{"product_views", "Просмотры карточки", pgInt8Ptr(row.ProductViews), productAnalyticsMetricPtr(prev, "product_views")},
		{"add_to_cart", "Добавления в корзину", pgInt8Ptr(row.AddToCart), productAnalyticsMetricPtr(prev, "add_to_cart")},
		{"orders", "Заказанные единицы", pgInt8Ptr(row.OrdersCount), productAnalyticsMetricPtr(prev, "orders")},
	}
	// delivered_units — confirmed Ozon analyticsMetric (Premium): delivered product units, not buyout rate.
	if row.CompletedOrders.Valid {
		labels = append(labels, struct {
			key     string
			label   string
			val     *int64
			prevVal *int64
		}{"delivered_units", "Доставленные единицы", pgInt8Ptr(row.CompletedOrders), productAnalyticsMetricPtr(prev, "delivered_units")})
	}
	stages := make([]FunnelStage, 0, len(labels))
	var top *int64
	var prevStage *int64
	for _, l := range labels {
		if l.val == nil {
			continue
		}
		stage := FunnelStage{
			Key:              l.key,
			Label:            l.label,
			Value:            l.val,
			PreviousDayValue: l.prevVal,
		}
		if top == nil {
			top = l.val
		}
		if prevStage != nil && *prevStage > 0 {
			c := math.Round((float64(*l.val)/float64(*prevStage))*10000) / 100
			stage.ConversionFromPrevious = &c
		}
		if top != nil && *top > 0 {
			c := math.Round((float64(*l.val)/float64(*top))*10000) / 100
			stage.ConversionFromTop = &c
		}
		stage.ChangeVsPreviousDayPct = computePercentChange(l.val, l.prevVal)
		stages = append(stages, stage)
		prevStage = l.val
	}
	return stages
}

func productAnalyticsMetricPtr(row *dbgen.ProductAnalyticsDaily, key string) *int64 {
	if row == nil {
		return nil
	}
	switch key {
	case "impressions":
		return pgInt8Ptr(row.Impressions)
	case "product_views":
		return pgInt8Ptr(row.ProductViews)
	case "add_to_cart":
		return pgInt8Ptr(row.AddToCart)
	case "orders":
		return pgInt8Ptr(row.OrdersCount)
	case "delivered_units":
		return pgInt8Ptr(row.CompletedOrders)
	default:
		return nil
	}
}

func computePercentChange(today, yesterday *int64) *float64 {
	if today == nil || yesterday == nil || *yesterday == 0 {
		return nil
	}
	delta := (float64(*today) - float64(*yesterday)) / float64(*yesterday) * 100
	rounded := math.Round(delta*100) / 100
	return &rounded
}

type funnelAnalysisResult struct {
	stage   string
	message string
}

func analyzeFunnel(stages []FunnelStage, history []dbgen.ProductAnalyticsDaily) funnelAnalysisResult {
	if len(stages) < 2 {
		return funnelAnalysisResult{
			message: "Недостаточно этапов воронки для анализа.",
		}
	}
	maxDrop := -1.0
	var structuralFrom, structuralTo string
	type baselineCandidate struct {
		fromLabel string
		toLabel   string
		vsPrevPP  *float64
		vs7dPP    *float64
	}
	bestBaseline := baselineCandidate{}
	bestSeverity := 0.0
	hasBaseline := false
	for i := 1; i < len(stages); i++ {
		prev := stages[i-1]
		cur := stages[i]
		if prev.Value == nil || cur.Value == nil || *prev.Value <= 0 {
			continue
		}
		conv := float64(*cur.Value) / float64(*prev.Value)
		drop := 1 - conv
		if drop > maxDrop {
			maxDrop = drop
			structuralFrom = prev.Label
			structuralTo = cur.Label
		}
		var vsPrevPP *float64
		if prev.PreviousDayValue != nil && cur.PreviousDayValue != nil && *prev.PreviousDayValue > 0 {
			pv := (conv*100 - (float64(*cur.PreviousDayValue) / float64(*prev.PreviousDayValue) * 100))
			pv = math.Round(pv*100) / 100
			vsPrevPP = &pv
		}
		var vs7dPP *float64
		if avg := averageTransitionConversion(history, prev.Key, cur.Key); avg != nil {
			v := conv*100 - *avg
			v = math.Round(v*100) / 100
			vs7dPP = &v
		}
		if vsPrevPP == nil && vs7dPP == nil {
			continue
		}
		severity := 0.0
		if vsPrevPP != nil && *vsPrevPP < severity {
			severity = *vsPrevPP
		}
		if vs7dPP != nil && *vs7dPP < severity {
			severity = *vs7dPP
		}
		if !hasBaseline || severity < bestSeverity {
			hasBaseline = true
			bestSeverity = severity
			bestBaseline = baselineCandidate{
				fromLabel: prev.Label,
				toLabel:   cur.Label,
				vsPrevPP:  vsPrevPP,
				vs7dPP:    vs7dPP,
			}
		}
	}
	if structuralFrom == "" {
		return funnelAnalysisResult{message: "Не удалось определить основной этап потерь."}
	}
	if hasBaseline && bestSeverity < 0 {
		details := make([]string, 0, 2)
		if bestBaseline.vsPrevPP != nil {
			details = append(details, fmt.Sprintf("%s ко вчера", formatSignedPP(*bestBaseline.vsPrevPP)))
		}
		if bestBaseline.vs7dPP != nil {
			details = append(details, fmt.Sprintf("%s к среднему за 7 дней", formatSignedPP(*bestBaseline.vs7dPP)))
		}
		return funnelAnalysisResult{
			stage: bestBaseline.fromLabel + " → " + bestBaseline.toLabel,
			message: fmt.Sprintf(
				"Основное ухудшение относительно последних данных заметно на этапе %s → %s: конверсия снизилась на %s.",
				bestBaseline.fromLabel,
				bestBaseline.toLabel,
				strings.Join(details, " / "),
			),
		}
	}
	pct := math.Round(maxDrop*1000) / 10
	return funnelAnalysisResult{
		stage: structuralFrom + " → " + structuralTo,
		message: fmt.Sprintf(
			"Недостаточно истории, чтобы сравнить воронку с обычным уровнем. Показан текущий структурный провал: %s → %s (падение ~%s).",
			structuralFrom,
			structuralTo,
			formatPercent(pct),
		),
	}
}

func averageTransitionConversion(history []dbgen.ProductAnalyticsDaily, fromKey, toKey string) *float64 {
	rows := filterAccountProductAnalyticsForFunnel(history)
	if len(rows) == 0 {
		return nil
	}
	total := 0.0
	count := 0
	for i := range rows {
		fromVal := funnelMetricFromProductRow(&rows[i], fromKey)
		toVal := funnelMetricFromProductRow(&rows[i], toKey)
		if fromVal == nil || toVal == nil || *fromVal <= 0 {
			continue
		}
		total += float64(*toVal) / float64(*fromVal) * 100
		count++
	}
	if count == 0 {
		return nil
	}
	avg := math.Round((total/float64(count))*100) / 100
	return &avg
}

func formatSignedPP(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "−"
	}
	return fmt.Sprintf("%s%.2f п.п.", sign, math.Abs(v))
}

func (s *Service) applySKUFocus(ctx context.Context, sellerID int64, metricDate time.Time, dto analytics.DashboardMetricsDTO, resp *Response, partial *partialTracker) {
	productIDs := make([]int64, 0)
	collectID := func(id int64) {
		productIDs = append(productIDs, id)
	}

	replenish, err2 := s.replenishment.ListReplenishmentForSellerAccount(ctx, sellerID, metricDate)
	if err2 != nil {
		partial.mark(partialReasonSKUFocusData)
	}

	// Order drop: strongest negative orders delta first, then revenue delta.
	orderDropCandidates := make([]analytics.DashboardSKUMetricsItem, 0)
	for _, sku := range dto.SKUs {
		if sku.OrdersDeltaDayToDay >= 0 {
			continue
		}
		orderDropCandidates = append(orderDropCandidates, sku)
	}
	sort.Slice(orderDropCandidates, func(i, j int) bool {
		a, b := orderDropCandidates[i], orderDropCandidates[j]
		if a.OrdersDeltaDayToDay != b.OrdersDeltaDayToDay {
			return a.OrdersDeltaDayToDay < b.OrdersDeltaDayToDay
		}
		if a.RevenueDeltaDayToDay != b.RevenueDeltaDayToDay {
			return a.RevenueDeltaDayToDay < b.RevenueDeltaDayToDay
		}
		nameA := derefStr(a.ProductName, "")
		nameB := derefStr(b.ProductName, "")
		if nameA != nameB {
			return nameA < nameB
		}
		return a.OzonProductID < b.OzonProductID
	})
	orderDrop := make([]SKUFocusCard, 0, minInt(len(orderDropCandidates), skuFocusLimit))
	for i, sku := range orderDropCandidates {
		if i >= skuFocusLimit {
			break
		}
		collectID(sku.OzonProductID)
		prevOrders := sku.OrdersCount - sku.OrdersDeltaDayToDay
		curOrders := sku.OrdersCount
		ordersDelta := sku.OrdersDeltaDayToDay
		revenueDelta := sku.RevenueDeltaDayToDay
		orderDrop = append(orderDrop, SKUFocusCard{
			OzonProductID:        sku.OzonProductID,
			SKU:                  sku.SKU,
			OfferID:              sku.OfferID,
			ProductName:          derefStr(sku.ProductName, "Товар"),
			Problem:              "Просадка заказов",
			KeyMetric:            fmt.Sprintf("Δ заказы: %d", sku.OrdersDeltaDayToDay),
			OrdersDeltaDayToDay:  &ordersDelta,
			RevenueDeltaDayToDay: &revenueDelta,
			PreviousOrders:       &prevOrders,
			CurrentOrders:        &curOrders,
		})
	}

	// Stock risk: lowest coverage first, then revenue at risk; only in-stock SKUs with demand.
	stockProbeIDs := make([]int64, 0)
	if err2 == nil {
		for _, item := range replenish.Items {
			if !isReplenishmentStockFocusCandidate(item) {
				continue
			}
			stockProbeIDs = append(stockProbeIDs, item.OzonProductID)
		}
	}
	for _, sku := range dto.SKUs {
		if !isDashboardStockFocusSKU(sku) {
			continue
		}
		stockProbeIDs = append(stockProbeIDs, sku.OzonProductID)
	}
	stockFocusMetricsByProduct := s.loadStockFocusMetrics(ctx, sellerID, uniqueInt64(stockProbeIDs), metricDate)

	stockRiskCandidates := make([]stockFocusCandidate, 0)
	stockRiskSeen := make(map[int64]struct{})
	appendStockCandidate := func(c stockFocusCandidate) {
		if _, ok := stockRiskSeen[c.ozonProductID]; ok {
			return
		}
		stockRiskSeen[c.ozonProductID] = struct{}{}
		stockRiskCandidates = append(stockRiskCandidates, c)
	}
	if err2 == nil {
		for _, item := range replenish.Items {
			if !isReplenishmentStockFocusCandidate(item) {
				continue
			}
			coverage := *item.DaysOfCover
			metrics := stockFocusMetricsByProduct[item.OzonProductID]
			if c, ok := buildStockFocusCandidate(
				item.OzonProductID,
				item.SKU,
				item.OfferID,
				derefStr(item.ProductName, "Товар"),
				item.CurrentAvailableStock,
				coverage,
				metrics,
			); ok {
				collectID(item.OzonProductID)
				appendStockCandidate(c)
			}
		}
	}
	for _, sku := range dto.SKUs {
		if !isDashboardStockFocusSKU(sku) {
			continue
		}
		if _, ok := stockRiskSeen[sku.OzonProductID]; ok {
			continue
		}
		coverage := *sku.DaysOfCover
		metrics := stockFocusMetricsByProduct[sku.OzonProductID]
		if c, ok := buildStockFocusCandidate(
			sku.OzonProductID,
			sku.SKU,
			sku.OfferID,
			derefStr(sku.ProductName, "Товар"),
			sku.StockAvailable,
			coverage,
			metrics,
		); ok {
			collectID(sku.OzonProductID)
			appendStockCandidate(c)
		}
	}
	sortStockFocusCandidates(stockRiskCandidates)
	stockRisk := make([]SKUFocusCard, 0, minInt(len(stockRiskCandidates), skuFocusLimit))
	for i, candidate := range stockRiskCandidates {
		if i >= skuFocusLimit {
			break
		}
		stockRisk = append(stockRisk, stockFocusCardFromCandidate(candidate))
	}

	// No sales with stock: positive stock and zero orders on last completed metric day.
	noSalesProbeIDs := make([]int64, 0)
	for _, sku := range dto.SKUs {
		if sku.StockAvailable <= 0 || sku.OrdersCount != 0 {
			continue
		}
		noSalesProbeIDs = append(noSalesProbeIDs, sku.OzonProductID)
	}
	noSalesMetricsByProduct := s.loadNoSalesFocusMetrics(ctx, sellerID, uniqueInt64(noSalesProbeIDs), metricDate)
	referencePrices := s.loadProductReferencePrices(ctx, sellerID, uniqueInt64(noSalesProbeIDs))
	noSalesCandidates := buildNoSalesFocusCandidates(dto.SKUs, metricDate, noSalesMetricsByProduct, referencePrices)
	sortNoSalesFocusCandidates(noSalesCandidates)
	totalNoSales := len(noSalesCandidates)
	noSales := make([]SKUFocusCard, 0, minInt(totalNoSales, skuFocusLimit))
	for i, candidate := range noSalesCandidates {
		if i >= skuFocusLimit {
			break
		}
		collectID(candidate.ozonProductID)
		noSales = append(noSales, noSalesFocusCardFromCandidate(candidate))
	}

	// Surplus
	surplus := make([]SKUFocusCard, 0)
	for _, sku := range dto.SKUs {
		if sku.StockStatusBucket == nil || *sku.StockStatusBucket != "surplus_120d" {
			if sku.ExcessStockShare == nil || *sku.ExcessStockShare <= 0.2 {
				continue
			}
		}
		collectID(sku.OzonProductID)
		surplus = append(surplus, SKUFocusCard{
			OzonProductID: sku.OzonProductID,
			SKU:           sku.SKU,
			OfferID:       sku.OfferID,
			ProductName:   derefStr(sku.ProductName, "Товар"),
			Problem:       "Риск излишков / платного хранения",
			KeyMetric:     fmt.Sprintf("Остаток %d", sku.StockAvailable),
		})
		if len(surplus) >= skuFocusLimit {
			break
		}
	}

	// Key SKUs: top revenue contributors covering 80% of store revenue over 7 completed days.
	storeTotalRevenue7d := calcStoreTotalRevenue7dFromDashboardSKUs(dto.SKUs)
	keySkuCandidates := buildKeySkuCandidates(dto.SKUs)
	keySkuSelected := selectKeySkuCandidates(keySkuCandidates, storeTotalRevenue7d)
	totalKeySkus := len(keySkuSelected)
	keySkusTotalRevenue := sumKeySkuSelectedRevenue7d(keySkuSelected)
	keySkuProbeIDs := make([]int64, 0, totalKeySkus)
	for _, c := range keySkuSelected {
		keySkuProbeIDs = append(keySkuProbeIDs, c.ozonProductID)
		collectID(c.ozonProductID)
	}
	keySkuTrends := s.loadKeySkuRevenueTrends(ctx, sellerID, keySkuProbeIDs, metricDate)
	for i := range keySkuSelected {
		if trend, ok := keySkuTrends[keySkuSelected[i].ozonProductID]; ok {
			keySkuSelected[i].revenueTrend = trend
		}
	}
	keySkus := make([]KeySkuCard, 0, minInt(totalKeySkus, skuFocusLimit))
	for i, candidate := range keySkuSelected {
		if i >= skuFocusLimit {
			break
		}
		keySkus = append(keySkus, keySkuCardFromCandidate(candidate))
	}

	enrich := s.loadProductEnrichment(ctx, sellerID, uniqueInt64(productIDs))
	sparklines := s.loadSKUSparklines(ctx, sellerID, uniqueInt64(productIDs), metricDate)

	enrichCards := func(cards []SKUFocusCard) []SKUFocusCard {
		for i := range cards {
			if e, ok := enrich[cards[i].OzonProductID]; ok {
				if e.ImageURL != nil {
					cards[i].ImageURL = e.ImageURL
				}
				if e.ProductName != "" {
					cards[i].ProductName = e.ProductName
				}
			}
			if sp, ok := sparklines[cards[i].OzonProductID]; ok {
				cards[i].Sparkline7D = sp
			}
		}
		return cards
	}

	enrichStockCards := func(cards []SKUFocusCard) []SKUFocusCard {
		for i := range cards {
			if e, ok := enrich[cards[i].OzonProductID]; ok {
				if e.ImageURL != nil {
					cards[i].ImageURL = e.ImageURL
				}
				if e.ProductName != "" {
					cards[i].ProductName = e.ProductName
				}
			}
		}
		return cards
	}

	resp.SKUFocus = SKUFocus{
		OrderDrop:                  enrichCards(orderDrop),
		StockRisk:                  enrichStockCards(stockRisk),
		NoSalesWithStock:           enrichNoSalesCards(noSales, enrich),
		NoSalesWithStockTotalCount: totalNoSales,
		Surplus:                    enrichCards(surplus),
		SlowTurnover:               enrichCards(surplus),
		KeySKUs:                    enrichKeySkuCards(keySkus, enrich),
		KeySKUsTotalCount:          totalKeySkus,
		StoreTotalRevenue7d:        math.Round(storeTotalRevenue7d*100) / 100,
		KeySKUsTotalRevenue7d:      math.Round(keySkusTotalRevenue*100) / 100,
	}
}

type productEnrichment struct {
	ProductName string
	ImageURL    *string
}

func enrichNoSalesCards(cards []SKUFocusCard, enrich map[int64]productEnrichment) []SKUFocusCard {
	for i := range cards {
		if e, ok := enrich[cards[i].OzonProductID]; ok {
			if e.ImageURL != nil {
				cards[i].ImageURL = e.ImageURL
			}
			if e.ProductName != "" {
				cards[i].ProductName = e.ProductName
			}
		}
	}
	return cards
}

func (s *Service) loadNoSalesFocusMetrics(ctx context.Context, sellerID int64, ids []int64, metricDate time.Time) map[int64]noSalesFocusMetrics {
	out := make(map[int64]noSalesFocusMetrics)
	if len(ids) == 0 {
		return out
	}
	from := metricDate.AddDate(0, 0, -noSalesHistoryLookbackDays)
	rows, err := s.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})
	if err != nil {
		return out
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := idSet[row.OzonProductID]; !ok {
			continue
		}
		if !row.MetricDate.Valid {
			continue
		}
		key := row.MetricDate.Time.Format("2006-01-02")
		metrics := out[row.OzonProductID]
		if metrics.ordersByDate == nil {
			metrics.ordersByDate = make(map[string]noSalesDayMetric)
		}
		revenue := 0.0
		if row.Revenue.Valid {
			revenue = numericToFloat(row.Revenue)
		}
		metrics.ordersByDate[key] = noSalesDayMetric{
			orders:  row.OrdersCount,
			revenue: revenue,
			hasData: true,
		}
		out[row.OzonProductID] = metrics
	}
	return out
}

func (s *Service) loadProductReferencePrices(ctx context.Context, sellerID int64, ids []int64) map[int64]float64 {
	out := make(map[int64]float64)
	if len(ids) == 0 {
		return out
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	rows, err := s.q.ListProductPricingBySellerAccountID(ctx, sellerID)
	if err != nil {
		return out
	}
	for _, row := range rows {
		if _, ok := idSet[row.OzonProductID]; !ok {
			continue
		}
		if row.ReferencePrice.Valid {
			price := numericToFloat(row.ReferencePrice)
			if price > 0 {
				out[row.OzonProductID] = price
			}
		}
	}
	return out
}

func (s *Service) loadProductEnrichment(ctx context.Context, sellerID int64, ids []int64) map[int64]productEnrichment {
	out := make(map[int64]productEnrichment)
	if len(ids) == 0 {
		return out
	}
	rows, err := s.q.ListProductCardsByOzonProductIDs(ctx, dbgen.ListProductCardsByOzonProductIDsParams{
		SellerAccountID: sellerID,
		OzonProductIds:  ids,
	})
	if err != nil {
		return out
	}
	for _, r := range rows {
		name := r.Name
		var img *string
		if r.PrimaryImageUrl.Valid && r.PrimaryImageUrl.String != "" {
			v := r.PrimaryImageUrl.String
			img = &v
		}
		out[r.OzonProductID] = productEnrichment{ProductName: name, ImageURL: img}
	}
	return out
}

// enrichFrozenSKUFocusImages fills missing ImageURL on published SKU Focus cards
// from products.primary_image_url. Presentation-only; does not change metrics.
func (s *Service) enrichFrozenSKUFocusImages(ctx context.Context, sellerID int64, resp *Response) {
	if s == nil || resp == nil {
		return
	}
	ids := make([]int64, 0, 64)
	collect := func(id int64) {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	for _, c := range resp.SKUFocus.OrderDrop {
		collect(c.OzonProductID)
	}
	for _, c := range resp.SKUFocus.StockRisk {
		collect(c.OzonProductID)
	}
	for _, c := range resp.SKUFocus.NoSalesWithStock {
		collect(c.OzonProductID)
	}
	for _, c := range resp.SKUFocus.SlowTurnover {
		collect(c.OzonProductID)
	}
	for _, c := range resp.SKUFocus.KeySKUs {
		collect(c.OzonProductID)
	}
	enrich := s.loadProductEnrichment(ctx, sellerID, uniqueInt64(ids))
	apply := func(cards []SKUFocusCard) {
		for i := range cards {
			if cards[i].ImageURL != nil && strings.TrimSpace(*cards[i].ImageURL) != "" {
				continue
			}
			if e, ok := enrich[cards[i].OzonProductID]; ok && e.ImageURL != nil {
				cards[i].ImageURL = e.ImageURL
			}
		}
	}
	apply(resp.SKUFocus.OrderDrop)
	apply(resp.SKUFocus.StockRisk)
	apply(resp.SKUFocus.NoSalesWithStock)
	apply(resp.SKUFocus.SlowTurnover)
	for i := range resp.SKUFocus.KeySKUs {
		if resp.SKUFocus.KeySKUs[i].ImageURL != nil && strings.TrimSpace(*resp.SKUFocus.KeySKUs[i].ImageURL) != "" {
			continue
		}
		if e, ok := enrich[resp.SKUFocus.KeySKUs[i].OzonProductID]; ok && e.ImageURL != nil {
			resp.SKUFocus.KeySKUs[i].ImageURL = e.ImageURL
		}
	}
}

func (s *Service) loadSKUSparklines(ctx context.Context, sellerID int64, ids []int64, metricDate time.Time) map[int64][]SparklinePoint {
	out := make(map[int64][]SparklinePoint)
	if len(ids) == 0 {
		return out
	}
	from := metricDate.AddDate(0, 0, -6)
	rows, err := s.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})
	if err != nil {
		return out
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	byProductDate := make(map[int64]map[string]sparklineDayMetrics)
	for _, row := range rows {
		if _, ok := idSet[row.OzonProductID]; !ok {
			continue
		}
		d := row.MetricDate.Time.Format("2006-01-02")
		if byProductDate[row.OzonProductID] == nil {
			byProductDate[row.OzonProductID] = make(map[string]sparklineDayMetrics)
		}
		entry := byProductDate[row.OzonProductID][d]
		entry.orders = float64(row.OrdersCount)
		entry.hasOrders = true
		if row.Revenue.Valid {
			entry.revenue = numericToFloat(row.Revenue)
			entry.hasRevenue = true
		}
		byProductDate[row.OzonProductID][d] = entry
	}
	byProduct := make(map[int64][]SparklinePoint, len(ids))
	for _, id := range ids {
		values := byProductDate[id]
		if values == nil {
			values = map[string]sparklineDayMetrics{}
		}
		byProduct[id] = buildCalendarOrdersRevenueSparklineSeries(from, metricDate, values)
	}
	return byProduct
}

func (s *Service) loadKeySkuRevenueTrends(ctx context.Context, sellerID int64, ids []int64, metricDate time.Time) map[int64][]KeySkuRevenueTrendPoint {
	out := make(map[int64][]KeySkuRevenueTrendPoint)
	if len(ids) == 0 {
		return out
	}
	from := metricDate.AddDate(0, 0, -6)
	rows, err := s.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})
	if err != nil {
		return out
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	byProductDate := make(map[int64]map[string]sparklineDayMetrics)
	for _, row := range rows {
		if _, ok := idSet[row.OzonProductID]; !ok {
			continue
		}
		d := row.MetricDate.Time.Format("2006-01-02")
		if byProductDate[row.OzonProductID] == nil {
			byProductDate[row.OzonProductID] = make(map[string]sparklineDayMetrics)
		}
		entry := byProductDate[row.OzonProductID][d]
		entry.orders = float64(row.OrdersCount)
		entry.hasOrders = true
		if row.Revenue.Valid {
			entry.revenue = numericToFloat(row.Revenue)
			entry.hasRevenue = true
		}
		byProductDate[row.OzonProductID][d] = entry
	}
	for _, id := range ids {
		values := byProductDate[id]
		if values == nil {
			values = map[string]sparklineDayMetrics{}
		}
		out[id] = buildCalendarKeySkuRevenueTrend(from, metricDate, values)
	}
	return out
}

func enrichKeySkuCards(cards []KeySkuCard, enrich map[int64]productEnrichment) []KeySkuCard {
	for i := range cards {
		if e, ok := enrich[cards[i].OzonProductID]; ok {
			if e.ImageURL != nil {
				cards[i].ImageURL = e.ImageURL
			}
			if e.ProductName != "" {
				cards[i].ProductName = e.ProductName
			}
		}
	}
	return cards
}

func (s *Service) loadStockFocusMetrics(ctx context.Context, sellerID int64, ids []int64, metricDate time.Time) map[int64]stockFocusMetrics {
	out := make(map[int64]stockFocusMetrics)
	if len(ids) == 0 {
		return out
	}
	from := metricDate.AddDate(0, 0, -6)
	yesterdayKey := metricDate.AddDate(0, 0, -1).Format("2006-01-02")
	rows, err := s.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	})
	if err != nil {
		return out
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	byProductDate := make(map[int64]map[string]stockTrendDayMetrics)
	revenueSum := make(map[int64]float64)
	revenueDays := make(map[int64]int)
	yesterdayStock := make(map[int64]int32)
	for _, row := range rows {
		if _, ok := idSet[row.OzonProductID]; !ok {
			continue
		}
		d := row.MetricDate.Time.Format("2006-01-02")
		if byProductDate[row.OzonProductID] == nil {
			byProductDate[row.OzonProductID] = make(map[string]stockTrendDayMetrics)
		}
		var coverage *float64
		if row.DaysOfCover.Valid {
			v := numericToFloat(row.DaysOfCover)
			coverage = &v
		}
		byProductDate[row.OzonProductID][d] = stockTrendDayMetrics{
			stock:        row.StockAvailable,
			coverageDays: coverage,
			hasData:      true,
		}
		if row.Revenue.Valid {
			revenueSum[row.OzonProductID] += numericToFloat(row.Revenue)
			revenueDays[row.OzonProductID]++
		}
		if d == yesterdayKey {
			yesterdayStock[row.OzonProductID] = row.StockAvailable
		}
	}
	for _, id := range ids {
		values := byProductDate[id]
		if values == nil {
			values = map[string]stockTrendDayMetrics{}
		}
		metrics := stockFocusMetrics{
			stockTrend: buildCalendarStockTrendSeries(from, metricDate, values),
		}
		if prev, ok := yesterdayStock[id]; ok {
			prevCopy := prev
			metrics.previousStock = &prevCopy
		}
		if metrics.previousStock == nil {
			metrics.previousStock = previousStockFromTrend(metrics.stockTrend)
		}
		if days := revenueDays[id]; days > 0 {
			avg := math.Round((revenueSum[id]/float64(days))*100) / 100
			metrics.avgDailyRevenue7d = avg
			metrics.hasRevenue7d = true
		}
		out[id] = metrics
	}
	return out
}

func (s *Service) applyStoredDashboardNarrativeToHome(
	ctx context.Context,
	sellerID int64,
	metricDate time.Time,
	dashboardDTO analytics.DashboardMetricsDTO,
	snapshot publishedsnapshot.Snapshot,
	resp *Response,
) {
	if metricDate.IsZero() || resp == nil {
		return
	}
	recs, err := s.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerID, Limit: 20, Offset: 0,
	})
	if err != nil {
		recs = nil
	}
	alertsRows, err := s.q.ListAlertsFiltered(ctx, dbgen.ListAlertsFilteredParams{
		SellerAccountID: sellerID,
		Status:          pgtype.Text{String: "open", Valid: true},
		Severity:        pgtype.Text{},
		AlertGroup:      pgtype.Text{},
		EntityType:      pgtype.Text{},
		Limit:           30,
		Offset:          0,
	})
	if err != nil {
		alertsRows = nil
	}
	actionPool := buildTopActionPool(nil, recs, alertsRows, resp.AIDiagnostics.AdEfficiency.Points, actionsFeedLimit)
	resp.AIDiagnostics.AccountSummary.storedNarrative = s.loadStoredDashboardNarrativeBySnapshot(ctx, sellerID, snapshot)

	var previousSnapshot *accountMetricsSnapshot
	from := metricDate.UTC().AddDate(0, 0, -14)
	if accRows, err := s.q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	}); err == nil {
		previousSnapshot = findPreviousAccountSnapshot(accRows, metricDate)
	}
	var narrativeBuildOverride *dashboardNarrativeBuildResult
	if shouldUseDataPackNarrativeBuild(resp.AIDiagnostics.AccountSummary.storedNarrative) {
		if build, buildErr := s.buildDataPackNarrativeBuildForApply(
			ctx, sellerID, metricDate, dashboardDTO, resp, recs, alertsRows, previousSnapshot,
		); buildErr == nil {
			narrativeBuildOverride = &build
		} else {
			slog.Warn("dashboard home: datapack narrative build for apply failed",
				"seller_account_id", sellerID,
				"metric_date", metricDate.UTC().Format("2006-01-02"),
				"error", buildErr.Error())
		}
	}
	applyUnifiedAccountHealth(
		metricDate, dashboardDTO, resp, recs, alertsRows, actionPool, previousSnapshot,
		narrativeBuildOverride,
	)
	// Unified Top 3 is frozen at publication from validated top_action_ids.
	// Do not re-rank or replace from the actions pipeline on GET.
	if shouldOverrideTopActionsFromFinal(resp.AIDiagnostics.AccountSummary.PromptVersion) {
		s.overrideTopActionsFromFinalActions(ctx, sellerID, metricDate, resp)
	}
	if dashboardAIOnlyEnabled() {
		resp.RecommendedActions = []ActionItem{}
	}
}

func (s *Service) applyRecommendedActions(ctx context.Context, sellerID int64, metricDate time.Time, dashboardDTO analytics.DashboardMetricsDTO, resp *Response) {
	aiOnly := dashboardAIOnlyEnabled()
	var recItems []ActionItem
	recs, err := s.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerID, Limit: 20, Offset: 0,
	})
	if err == nil {
		for _, r := range recs {
			id := r.ID
			recItems = append(recItems, ActionItem{
				Priority:               r.PriorityLevel,
				UrgencyGroup:           urgencyGroupForPriority(r.PriorityLevel),
				Action:                 r.RecommendedAction,
				Reason:                 r.Title,
				ExpectedEffect:         derefText(r.ExpectedEffect),
				RelatedEntityType:      r.EntityType,
				RelatedEntityID:        textPtr(r.EntityID),
				Source:                 "recommendation",
				LinkedRecommendationID: &id,
			})
		}
	}

	var aiItems []ActionItem
	if !aiOnly && resp.MetricDate != "" {
		if md, err := time.Parse("2006-01-02", resp.MetricDate); err == nil {
			rows, err := s.q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
				SellerAccountID: sellerID, MetricDate: dateValue(md),
			})
			if err == nil {
				for _, row := range rows {
					if row.SummaryType != "account_daily_summary" {
						continue
					}
					stored := storedRowFromDB(row)
					payload, outcome, ok := acceptStoredAccountSummaryPayload(stored)
					if !ok {
						continue
					}
					items, recDebug := buildAIActionsFromAcceptedPayload(payload, actionsFeedLimit)
					aiItems = append(aiItems, items...)
					if shouldExposeAISummariesDebug() {
						if resp.AIDiagnostics.AISummariesDebug == nil {
							resp.AIDiagnostics.AISummariesDebug = &AISummariesDebug{}
						}
						resp.AIDiagnostics.AISummariesDebug.RecommendedActions = RecommendedDebug{
							AIReceived:   recDebug.AIReceived,
							AIAccepted:   recDebug.AIAccepted,
							Dropped:      recDebug.Dropped,
							FallbackNote: outcome.ValidationStatus,
						}
					}
				}
			}
		}
	}

	var alertItems []ActionItem
	alertsRows, err := s.q.ListAlertsFiltered(ctx, dbgen.ListAlertsFilteredParams{
		SellerAccountID: sellerID,
		Status:          pgtype.Text{String: "open", Valid: true},
		Severity:        pgtype.Text{},
		AlertGroup:      pgtype.Text{},
		EntityType:      pgtype.Text{},
		Limit:           30,
		Offset:          0,
	})
	if err == nil {
		for _, a := range alertsRows {
			if a.Severity != "critical" && a.Severity != "high" {
				continue
			}
			if alerts.IsLegacyAlertType(alerts.AlertType(a.AlertType)) {
				continue
			}
			id := a.ID
			alertItems = append(alertItems, ActionItem{
				Priority:          string(severityToPriority(a.Severity)),
				UrgencyGroup:      urgencyGroupForPriority(string(severityToPriority(a.Severity))),
				Action:            a.Title,
				Reason:            a.Message,
				RelatedEntityType: a.EntityType,
				RelatedEntityID:   textPtr(a.EntityID),
				Source:            "alert",
				LinkedAlertID:     &id,
			})
		}
	}

	actionPool := buildTopActionPool(nil, recs, alertsRows, resp.AIDiagnostics.AdEfficiency.Points, actionsFeedLimit)
	_ = actionPool
	if !metricDate.IsZero() {
		s.applyStoredDashboardNarrativeToHome(ctx, sellerID, metricDate, dashboardDTO, publishedsnapshot.Snapshot{
			MetricDate: metricDate,
		}, resp)
	}
	if aiOnly {
		// Stage 4: dashboard no longer serves active recommended_actions block.
		resp.RecommendedActions = []ActionItem{}
		return
	}
	var narrativeActionItems []ActionItem
	if resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" && len(resp.AIDiagnostics.TopActions) > 0 {
		narrativeActionItems = recommendedActionsFromTopActions(resp.AIDiagnostics.TopActions)
	}
	resp.RecommendedActions = dedupeActionFeed(
		collectActionCandidates(narrativeActionItems, aiItems, recItems, alertItems),
		actionsFeedLimit,
	)
}

func (s *Service) loadStoredDashboardNarrative(ctx context.Context, sellerID int64, metricDate time.Time) *storedDashboardNarrative {
	row, err := s.q.GetAIDashboardNarrativeBySellerAndMetricDate(ctx, dbgen.GetAIDashboardNarrativeBySellerAndMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(metricDate),
	})
	if err != nil {
		return nil
	}
	stored := &storedDashboardNarrative{
		Status:              row.Status,
		PromptVersion:       row.PromptVersion,
		ContextHash:         row.ContextHash,
		RenderedContextHash: row.RiskDriversHash,
		RiskDriversHash:     row.RiskDriversHash,
	}
	if row.AiPipelineRunID.Valid {
		stored.AiPipelineRunID = row.AiPipelineRunID.Int64
	}
	if row.DataPackHash.Valid {
		stored.DataPackHash = strings.TrimSpace(row.DataPackHash.String)
	}
	if row.GlobalContextHash.Valid {
		stored.GlobalContextHash = strings.TrimSpace(row.GlobalContextHash.String)
	}
	if row.MetricDate.Valid {
		stored.MetricDate = row.MetricDate.Time.UTC().Format("2006-01-02")
	}
	if row.ID.Valid {
		stored.ID = row.ID.String()
	}
	if len(row.ValidationErrors) > 0 {
		_ = json.Unmarshal(row.ValidationErrors, &stored.ValidationErrors)
	}
	if len(row.ValidatedPayload) > 0 {
		payload, parseErr := parseDashboardNarrativePayload(string(row.ValidatedPayload))
		if parseErr != nil {
			stored.ValidationErrors = append(stored.ValidationErrors, parseErr.Error())
			return stored
		}
		stored.Payload = payload
	}
	return stored
}

func (s *Service) loadStoredDashboardNarrativeBySnapshot(ctx context.Context, sellerID int64, snapshot publishedsnapshot.Snapshot) *storedDashboardNarrative {
	if strings.TrimSpace(snapshot.DashboardRunID) == "" {
		return s.loadStoredDashboardNarrative(ctx, sellerID, snapshot.MetricDate)
	}
	row, err := s.q.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              parseUUID(snapshot.DashboardRunID),
		SellerAccountID: sellerID,
	})
	if err != nil {
		return nil
	}
	stored := &storedDashboardNarrative{
		Status:              row.Status,
		PromptVersion:       row.PromptVersion,
		ContextHash:         row.ContextHash,
		RenderedContextHash: row.RiskDriversHash,
		RiskDriversHash:     row.RiskDriversHash,
	}
	if row.AiPipelineRunID.Valid {
		stored.AiPipelineRunID = row.AiPipelineRunID.Int64
	}
	if row.DataPackHash.Valid {
		stored.DataPackHash = strings.TrimSpace(row.DataPackHash.String)
	}
	if row.GlobalContextHash.Valid {
		stored.GlobalContextHash = strings.TrimSpace(row.GlobalContextHash.String)
	}
	if row.MetricDate.Valid {
		stored.MetricDate = row.MetricDate.Time.UTC().Format("2006-01-02")
	}
	if row.ID.Valid {
		stored.ID = row.ID.String()
	}
	if len(row.ValidationErrors) > 0 {
		_ = json.Unmarshal(row.ValidationErrors, &stored.ValidationErrors)
	}
	if len(row.ValidatedPayload) > 0 {
		payload, parseErr := parseDashboardNarrativePayload(string(row.ValidatedPayload))
		if parseErr != nil {
			stored.ValidationErrors = append(stored.ValidationErrors, parseErr.Error())
			return stored
		}
		stored.Payload = payload
	}
	return stored
}

func (s *Service) runBlock(name string, fn func()) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dashboard home block panic", "block", name, "panic", r, "duration_ms", time.Since(start).Milliseconds())
		}
	}()
	fn()
	slog.Debug("dashboard home block", "block", name, "duration_ms", time.Since(start).Milliseconds())
}

func normalizeActionKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func priorityRank(p string) int {
	switch strings.ToLower(p) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	case "watch":
		return 4
	default:
		return 5
	}
}

func urgencyGroupForPriority(p string) string {
	switch strings.ToLower(p) {
	case "critical", "high":
		return "urgent"
	case "medium":
		return "today"
	default:
		return "watch"
	}
}

func severityToPriority(sev string) string {
	switch sev {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func dateValue(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t.UTC().Truncate(24 * time.Hour), Valid: true}
}

func parseUUID(raw string) pgtype.UUID {
	var out pgtype.UUID
	if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil {
		copy(out.Bytes[:], id[:])
		out.Valid = true
	}
	return out
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func derefStr(s *string, fallback string) string {
	if s != nil && *s != "" {
		return *s
	}
	return fallback
}

func derefText(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	return &t.String
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func firstSignal(signals []string, fallback string) string {
	if len(signals) > 0 {
		return signals[0]
	}
	return fallback
}

func uniqueInt64(in []int64) []int64 {
	seen := make(map[int64]struct{})
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.0f ₽", math.Round(v))
}

func formatInt(v int64) string {
	return fmt.Sprintf("%d", v)
}
