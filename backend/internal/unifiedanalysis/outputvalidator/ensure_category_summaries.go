package outputvalidator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// EnsureCategorySummaries backfills minimal dashboard.category_summaries when
// the Pack has categories but the model left the array empty. Scores/headlines
// are conservative placeholders grounded only in Pack category ids — never
// invented SKUs or metrics.
func EnsureCategorySummaries(out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) int {
	if out == nil || idx == nil || !idx.HasCategories() {
		return 0
	}
	if out.AnalysisOutcome != contract.AnalysisOutcomeMaterialActionsFound {
		return 0
	}
	if len(out.Dashboard.CategorySummaries) > 0 {
		return 0
	}

	ids := make([]string, 0, len(idx.CategoryIds))
	for id := range idx.CategoryIds {
		if strings.TrimSpace(id) == "" {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return 0
	}

	score := int32(50)
	out.Dashboard.CategorySummaries = make([]contract.UnifiedCategorySummaryV1, 0, len(ids))
	for _, id := range ids {
		name := fmt.Sprintf("Категория %s", id)
		out.Dashboard.CategorySummaries = append(out.Dashboard.CategorySummaries, contract.UnifiedCategorySummaryV1{
			CategoryID:        id,
			CategoryName:      name,
			CategoryScore:     &score,
			ScoreMax:          100,
			Severity:          "watch",
			Headline:          fmt.Sprintf("Категория «%s»: требуется контроль ключевых метрик", name),
			KeyRisk:           "По категории зафиксированы материальные отклонения, требующие проверки в связанных действиях.",
			RecommendedAction: "Открыть связанные действия по остаткам, продажам и рекламе для SKU этой категории.",
		})
	}
	return len(out.Dashboard.CategorySummaries)
}
