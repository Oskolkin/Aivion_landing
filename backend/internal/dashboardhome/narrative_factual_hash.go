package dashboardhome

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const factualHashFloatPrecision = 4

// factualNarrativeContextInput captures only stable semantic facts for narrative freshness.
// Technical row IDs (alerts, recommendations, candidate ids) are excluded — only business facts matter.
type factualNarrativeContextInput struct {
	MetricDate           string                      `json:"metric_date"`
	HealthScore          int32                       `json:"health_score"`
	Severity             string                      `json:"severity"`
	SalesSummary         map[string]any              `json:"sales_summary"`
	CancellationsSummary map[string]any              `json:"cancellations_summary"`
	SupportingMetrics    map[string]any              `json:"supporting_metrics"`
	RiskDrivers          []map[string]any            `json:"risk_drivers"`
	CandidateSignatures  []factualCandidateSignature `json:"candidate_signatures"`
	StockSegmentCounts   []factualStockSegment       `json:"stock_segment_counts"`
	FunnelStages         []factualFunnelStage        `json:"funnel_stages"`
	AdCampaigns          []factualAdCampaign         `json:"ad_campaigns"`
}

type factualCandidateSignature struct {
	Category            string   `json:"category"`
	Source              string   `json:"source"`
	Priority            string   `json:"priority"`
	CompatibleDriverIDs []string `json:"compatible_driver_ids,omitempty"`
}

type factualStockSegment struct {
	Key      string `json:"key"`
	SKUCount int    `json:"sku_count"`
}

type factualFunnelStage struct {
	Key                    string   `json:"key"`
	Value                  *int64   `json:"value,omitempty"`
	ChangeVsPreviousDayPct *float64 `json:"change_vs_previous_day_pct,omitempty"`
}

type factualAdCampaign struct {
	CampaignExternalID int64   `json:"campaign_external_id"`
	Spend              float64 `json:"spend"`
	Revenue            float64 `json:"revenue"`
	Orders             int64   `json:"orders"`
	EfficiencySignal   string  `json:"efficiency_signal"`
}

func buildFactualContextHashInput(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	drivers []RiskDriver,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
	recs []dbgen.Recommendation,
) factualNarrativeContextInput {
	candidateSignatures := buildFactualCandidateSignatures(drivers, actionPool)

	stockSegments := make([]factualStockSegment, 0, len(resp.AIDiagnostics.StockCoverage.Segments))
	segmentKeys := make([]string, 0, len(resp.AIDiagnostics.StockCoverage.Segments))
	for _, segment := range resp.AIDiagnostics.StockCoverage.Segments {
		segmentKeys = append(segmentKeys, segment.Key)
	}
	sort.Strings(segmentKeys)
	segmentByKey := map[string]StockCoverageSegment{}
	for _, segment := range resp.AIDiagnostics.StockCoverage.Segments {
		segmentByKey[segment.Key] = segment
	}
	for _, key := range segmentKeys {
		segment := segmentByKey[key]
		stockSegments = append(stockSegments, factualStockSegment{
			Key:      segment.Key,
			SKUCount: segment.SKUCount,
		})
	}

	funnelStages := make([]factualFunnelStage, 0, len(resp.AIDiagnostics.SalesFunnel.Stages))
	stageKeys := make([]string, 0, len(resp.AIDiagnostics.SalesFunnel.Stages))
	for _, stage := range resp.AIDiagnostics.SalesFunnel.Stages {
		stageKeys = append(stageKeys, stage.Key)
	}
	sort.Strings(stageKeys)
	stageByKey := map[string]FunnelStage{}
	for _, stage := range resp.AIDiagnostics.SalesFunnel.Stages {
		stageByKey[stage.Key] = stage
	}
	for _, key := range stageKeys {
		stage := stageByKey[key]
		var changePct *float64
		if stage.ChangeVsPreviousDayPct != nil {
			rounded := roundFactualFloat(*stage.ChangeVsPreviousDayPct)
			changePct = &rounded
		}
		funnelStages = append(funnelStages, factualFunnelStage{
			Key:                    stage.Key,
			Value:                  stage.Value,
			ChangeVsPreviousDayPct: changePct,
		})
	}

	adCampaigns := make([]factualAdCampaign, 0, len(resp.AIDiagnostics.AdEfficiency.Points))
	for _, point := range resp.AIDiagnostics.AdEfficiency.Points {
		adCampaigns = append(adCampaigns, factualAdCampaign{
			CampaignExternalID: point.CampaignExternalID,
			Spend:              roundFactualFloat(point.Spend),
			Revenue:            roundFactualFloat(point.Revenue),
			Orders:             point.Orders,
			EfficiencySignal:   point.EfficiencySignal,
		})
	}
	sort.SliceStable(adCampaigns, func(i, j int) bool {
		return adCampaigns[i].CampaignExternalID < adCampaigns[j].CampaignExternalID
	})

	return factualNarrativeContextInput{
		MetricDate:           metricDate.UTC().Format("2006-01-02"),
		HealthScore:          0, // filled by caller if needed
		Severity:             "",
		SalesSummary:         buildDashboardNarrativeSalesSummary(metricDate, dto),
		CancellationsSummary: buildDashboardNarrativeCancellationsSummary(metricDate, dto),
		SupportingMetrics:    buildFactualSupportingMetrics(dto, resp),
		RiskDrivers:          buildSemanticRiskDriverHashInput(drivers),
		CandidateSignatures:  candidateSignatures,
		StockSegmentCounts:   stockSegments,
		FunnelStages:         funnelStages,
		AdCampaigns:          adCampaigns,
	}
}

func buildFactualSupportingMetrics(dto analytics.DashboardMetricsDTO, resp *Response) map[string]any {
	criticalStock, shortageStock := stockCoverageCounts(resp.AIDiagnostics.StockCoverage.Segments)
	critRiskCount := int64(0)
	for _, group := range resp.AIDiagnostics.RiskDistribution.Groups {
		if strings.EqualFold(group.Severity, "critical") || strings.EqualFold(group.Severity, "high") {
			critRiskCount += group.Count
		}
	}
	return map[string]any{
		"account_ad_spend_to_revenue": roundFactualFloat(valueOrZero(dto.Account.AccountAdSpendToRevenue)),
		"critical_stock_skus":         criticalStock,
		"shortage_stock_skus":         shortageStock,
		"open_high_or_critical_risks": critRiskCount,
	}
}

func buildSemanticRiskDriverHashInput(drivers []RiskDriver) []map[string]any {
	sorted := append([]RiskDriver(nil), drivers...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	out := make([]map[string]any, 0, len(sorted))
	for _, driver := range sorted {
		out = append(out, map[string]any{
			"id":           driver.ID,
			"category":     driver.Category,
			"severity":     driver.Severity,
			"impact_score": driver.ImpactScore,
			"title":        strings.TrimSpace(driver.Title),
			"reason":       strings.TrimSpace(driver.Reason),
			"evidence":     normalizeFactualEvidence(driver.Evidence),
		})
	}
	return out
}

func normalizeFactualEvidence(evidence map[string]any) map[string]any {
	if len(evidence) == 0 {
		return map[string]any{}
	}
	keys := make([]string, 0, len(evidence))
	for key := range evidence {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = normalizeFactualValue(evidence[key])
	}
	return out
}

func normalizeFactualValue(value any) any {
	switch typed := value.(type) {
	case float64:
		return roundFactualFloat(typed)
	case float32:
		return roundFactualFloat(float64(typed))
	case map[string]any:
		return normalizeFactualEvidence(typed)
	case []any:
		normalized := make([]any, len(typed))
		for i, item := range typed {
			normalized[i] = normalizeFactualValue(item)
		}
		return normalized
	default:
		return value
	}
}

func roundFactualFloat(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	pow := math.Pow10(factualHashFloatPrecision)
	return math.Round(value*pow) / pow
}

func buildFactualCandidateSignatures(drivers []RiskDriver, actionPool []TopActionItem) []factualCandidateSignature {
	candidates := buildDashboardNarrativeCandidates(drivers, actionPool)
	out := make([]factualCandidateSignature, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		driverIDs := append([]string(nil), candidate.CompatibleDriverIDs...)
		sort.Strings(driverIDs)
		sig := factualCandidateSignature{
			Category:            candidate.Category,
			Source:              candidate.Source,
			Priority:            candidate.Priority,
			CompatibleDriverIDs: driverIDs,
		}
		key := hashNarrativeValue(sig)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, sig)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func buildFactualContextHash(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	drivers []RiskDriver,
	healthScore int32,
	severity string,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
	recs []dbgen.Recommendation,
) string {
	input := buildFactualContextHashInput(metricDate, dto, resp, drivers, alertRows, actionPool, recs)
	input.HealthScore = healthScore
	input.Severity = severity
	return hashNarrativeValue(input)
}
