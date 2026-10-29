package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	funnelNoComparisonDataPattern = regexp.MustCompile(`(?i)данных о крупнейшем ухудшении|нет данных.*ухудшен|сравнени[ея].*недоступн`)
	funnelHasDeltaPattern         = regexp.MustCompile(`(?i)[-+−]?\d+(?:[.,]\d+)?\s*п\.?\s*п\.?`)
)

func reconcileFunnelAnalysisConsistency(analysis *FunnelAnalysisWidget) {
	if analysis == nil {
		return
	}
	dropMsg := strings.TrimSpace(analysis.PrimaryDropMessage)
	ai := &analysis.AIAnalysis
	if dropMsg == "" || ai.Status != WidgetAvailable {
		return
	}
	hasFactualDrop := funnelHasDeltaPattern.MatchString(dropMsg) || strings.Contains(strings.ToLower(dropMsg), "ухудш")
	aiSaysNoData := funnelNoComparisonDataPattern.MatchString(ai.ShortSummary) ||
		funnelNoComparisonDataPattern.MatchString(ai.Summary) ||
		funnelNoComparisonDataPattern.MatchString(ai.MainConclusion)

	if hasFactualDrop && aiSaysNoData {
		commentary := buildFunnelComparisonCommentaryFromPrimaryDrop(dropMsg, analysis.PrimaryDropStage)
		if strings.TrimSpace(commentary) != "" {
			if strings.TrimSpace(ai.ComparisonsCommentary.PreviousPeriod) == "" {
				ai.ComparisonsCommentary.PreviousPeriod = commentary
			}
			ai.ShortSummary = buildFunnelReconciledShortSummary(*analysis, commentary)
		}
	}
	if stage := strings.TrimSpace(analysis.PrimaryDropStage); stage != "" {
		if strings.TrimSpace(ai.WeakestStage.Key) == "" {
			ai.WeakestStage = weakestStageFromFunnelWidget(*analysis)
		}
	}
}

func buildFunnelComparisonCommentaryFromPrimaryDrop(dropMsg, stage string) string {
	stage = strings.TrimSpace(stage)
	if stage != "" {
		return "Самое сильное ухудшение к вчера — на этапе «" + stage + "»: " + strings.TrimSpace(dropMsg)
	}
	return "Самое сильное ухудшение к вчера: " + strings.TrimSpace(dropMsg)
}

func buildFunnelReconciledShortSummary(analysis FunnelAnalysisWidget, deterioration string) string {
	parts := make([]string, 0, 2)
	if ws := weakestStageFromFunnelWidget(analysis); strings.TrimSpace(ws.Label) != "" && strings.TrimSpace(ws.Value) != "" {
		parts = append(parts, "Самая низкая абсолютная конверсия — "+ws.Label+": "+ws.Value+".")
	}
	if deterioration != "" {
		parts = append(parts, deterioration)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func weakestStageFromFunnelWidget(analysis FunnelAnalysisWidget) FunnelWeakestStage {
	if ws := analysis.AIAnalysis.WeakestStage; strings.TrimSpace(ws.Label) != "" && strings.TrimSpace(ws.Value) != "" {
		return ws
	}
	return FunnelWeakestStage{}
}

func formatFunnelPercentValue(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}
