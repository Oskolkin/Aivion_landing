package aipipeline

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

var gcRepairSpacedThousandsPattern = regexp.MustCompile(`\d{1,3}(?:\s\d{3})+`)

// RepairGlobalSemanticContext applies conservative backend repairs before validation.
// It drops invalid references, normalizes safe defaults, and must not invent new facts.
func RepairGlobalSemanticContext(ctx *aicontext.GlobalSemanticContext, input GlobalContextGenerationInput) {
	if ctx == nil {
		return
	}
	NormalizeGlobalSemanticContext(ctx)
	if strings.TrimSpace(ctx.SchemaVersion) == "" {
		ctx.SchemaVersion = aicontext.GlobalSemanticContextSchemaVersion
	}

	allowedNumbers := collectGlobalContextAllowedNumbers(input)
	repairGlobalContextConfidence(ctx)
	repairGlobalContextCategoriesAndDropInvalid(ctx)
	repairGlobalContextEntities(ctx, input)
	repairGlobalContextFactReferences(ctx, input)
	repairGlobalContextSeverity(ctx)
	repairGlobalContextDropIncompleteItems(ctx)
	repairGlobalContextStripUnsupportedNumbers(ctx, allowedNumbers)
	repairGlobalContextPriorityOrder(ctx)

	if strings.TrimSpace(ctx.Confidence) == "" {
		ctx.Confidence = "medium"
	}
	if len(ctx.SourceFactIDs) == 0 {
		if inferred := inferGlobalContextSourceFactIDs(*ctx, input); len(inferred) > 0 {
			ctx.SourceFactIDs = inferred
		}
	}
}

// GlobalContextIsEffectivelyEmpty reports whether the context has no usable semantic content.
func GlobalContextIsEffectivelyEmpty(ctx *aicontext.GlobalSemanticContext) bool {
	if ctx == nil {
		return true
	}
	if strings.TrimSpace(ctx.OverallStory) != "" {
		return false
	}
	return len(ctx.PrimaryRisks) == 0 && len(ctx.SecondaryRisks) == 0 && len(ctx.Opportunities) == 0
}

func repairGlobalContextConfidence(ctx *aicontext.GlobalSemanticContext) {
	conf := strings.TrimSpace(strings.ToLower(ctx.Confidence))
	if conf == "" {
		return
	}
	if containsString(aicontext.AllowedConfidenceValues, conf) {
		ctx.Confidence = conf
		return
	}
	// Model sometimes returns a numeric score instead of high/medium/low.
	if _, ok := parseGCNumberToken(conf); ok {
		ctx.Confidence = "medium"
		return
	}
	ctx.Confidence = "medium"
}

func repairGlobalContextSeverity(ctx *aicontext.GlobalSemanticContext) {
	repair := func(risks []aicontext.SemanticRisk) {
		for i := range risks {
			sev := strings.TrimSpace(risks[i].Severity)
			if sev == "" {
				continue
			}
			if !containsString(aicontext.AllowedSeverityValues, sev) {
				risks[i].Severity = ""
			}
		}
	}
	repair(ctx.PrimaryRisks)
	repair(ctx.SecondaryRisks)
}

func repairGlobalContextCategoriesAndDropInvalid(ctx *aicontext.GlobalSemanticContext) {
	ctx.PrimaryRisks = filterSemanticRisksByCategory(ctx.PrimaryRisks)
	ctx.SecondaryRisks = filterSemanticRisksByCategory(ctx.SecondaryRisks)
	ctx.Opportunities = filterSemanticSignalsByCategory(ctx.Opportunities)
}

func filterSemanticRisksByCategory(risks []aicontext.SemanticRisk) []aicontext.SemanticRisk {
	if len(risks) == 0 {
		return nil
	}
	out := make([]aicontext.SemanticRisk, 0, len(risks))
	for _, risk := range risks {
		risk.Category = normalizeGlobalContextCategory(risk.Category)
		if !globalContextCategoryAllowed(risk.Category) {
			continue
		}
		out = append(out, risk)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterSemanticSignalsByCategory(signals []aicontext.SemanticSignal) []aicontext.SemanticSignal {
	if len(signals) == 0 {
		return nil
	}
	out := make([]aicontext.SemanticSignal, 0, len(signals))
	for _, signal := range signals {
		signal.Category = normalizeGlobalContextCategory(signal.Category)
		if !globalContextCategoryAllowed(signal.Category) {
			continue
		}
		out = append(out, signal)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func globalContextCategoryAllowed(category string) bool {
	cat := strings.TrimSpace(category)
	if cat == "" {
		return false
	}
	if aicontext.IsForbiddenCategory(cat) {
		return false
	}
	return aicontext.IsAllowedCategory(cat)
}

func repairGlobalContextPriorityOrder(ctx *aicontext.GlobalSemanticContext) {
	if len(ctx.PriorityOrder) == 0 {
		return
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(ctx.PriorityOrder))
	for _, cat := range ctx.PriorityOrder {
		cat = normalizeGlobalContextCategory(cat)
		if !globalContextCategoryAllowed(cat) {
			continue
		}
		if _, ok := seen[cat]; ok {
			continue
		}
		seen[cat] = struct{}{}
		out = append(out, cat)
	}
	if len(out) == 0 {
		ctx.PriorityOrder = nil
		return
	}
	ctx.PriorityOrder = out
}

func repairGlobalContextEntities(ctx *aicontext.GlobalSemanticContext, input GlobalContextGenerationInput) {
	repairRiskEntities := func(risks []aicontext.SemanticRisk) {
		for i := range risks {
			risks[i].Entities = filterGlobalContextEntities(risks[i].Entities, input)
		}
	}
	repairSignalEntities := func(signals []aicontext.SemanticSignal) {
		for i := range signals {
			signals[i].Entities = filterGlobalContextEntities(signals[i].Entities, input)
		}
	}
	repairRiskEntities(ctx.PrimaryRisks)
	repairRiskEntities(ctx.SecondaryRisks)
	repairSignalEntities(ctx.Opportunities)
}

func filterGlobalContextEntities(entities []aicontext.SemanticEntityRef, input GlobalContextGenerationInput) []aicontext.SemanticEntityRef {
	if len(entities) == 0 {
		return nil
	}
	out := make([]aicontext.SemanticEntityRef, 0, len(entities))
	for _, ent := range entities {
		if !globalContextEntityAllowed(ent, input) {
			continue
		}
		out = append(out, ent)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func globalContextEntityAllowed(ent aicontext.SemanticEntityRef, input GlobalContextGenerationInput) bool {
	entityType := strings.TrimSpace(ent.EntityType)
	entityID := strings.TrimSpace(ent.EntityID)
	if entityType == "" || entityID == "" {
		return false
	}
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		_, ok := input.EntityCatalog[entityID]
		return ok
	}
	if input.DataPack == nil {
		return false
	}
	return entityExists(*input.DataPack, entityType, entityID)
}

func repairGlobalContextDropIncompleteItems(ctx *aicontext.GlobalSemanticContext) {
	ctx.PrimaryRisks = filterSemanticRisksWithTitle(ctx.PrimaryRisks)
	ctx.SecondaryRisks = filterSemanticRisksWithTitle(ctx.SecondaryRisks)
	ctx.Opportunities = filterSemanticSignalsWithTitle(ctx.Opportunities)
}

func filterSemanticRisksWithTitle(risks []aicontext.SemanticRisk) []aicontext.SemanticRisk {
	if len(risks) == 0 {
		return nil
	}
	out := make([]aicontext.SemanticRisk, 0, len(risks))
	for _, risk := range risks {
		if strings.TrimSpace(risk.Title) == "" {
			continue
		}
		out = append(out, risk)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterSemanticSignalsWithTitle(signals []aicontext.SemanticSignal) []aicontext.SemanticSignal {
	if len(signals) == 0 {
		return nil
	}
	out := make([]aicontext.SemanticSignal, 0, len(signals))
	for _, signal := range signals {
		if strings.TrimSpace(signal.Title) == "" {
			continue
		}
		out = append(out, signal)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func repairGlobalContextStripUnsupportedNumbers(ctx *aicontext.GlobalSemanticContext, allowed map[string]bool) {
	ctx.OverallStory = stripUnsupportedGCNumbers(ctx.OverallStory, allowed)
	for i := range ctx.PrimaryRisks {
		ctx.PrimaryRisks[i].Title = stripUnsupportedGCNumbers(ctx.PrimaryRisks[i].Title, allowed)
		ctx.PrimaryRisks[i].Rationale = stripUnsupportedGCNumbers(ctx.PrimaryRisks[i].Rationale, allowed)
	}
	for i := range ctx.SecondaryRisks {
		ctx.SecondaryRisks[i].Title = stripUnsupportedGCNumbers(ctx.SecondaryRisks[i].Title, allowed)
		ctx.SecondaryRisks[i].Rationale = stripUnsupportedGCNumbers(ctx.SecondaryRisks[i].Rationale, allowed)
	}
	for i := range ctx.Opportunities {
		ctx.Opportunities[i].Title = stripUnsupportedGCNumbers(ctx.Opportunities[i].Title, allowed)
		ctx.Opportunities[i].Rationale = stripUnsupportedGCNumbers(ctx.Opportunities[i].Rationale, allowed)
	}
	if ctx.DataLimitations != nil {
		lims := make([]string, 0, len(ctx.DataLimitations))
		for _, lim := range ctx.DataLimitations.Strings() {
			lim = strings.TrimSpace(stripUnsupportedGCNumbers(lim, allowed))
			if lim != "" {
				lims = append(lims, lim)
			}
		}
		ctx.DataLimitations = aicontext.FlexibleStringSlice(compactStrings(lims))
	}
	ctx.OverallStory = strings.TrimSpace(ctx.OverallStory)
}

func stripUnsupportedGCNumbers(text string, allowed map[string]bool) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	text = gcRepairSpacedThousandsPattern.ReplaceAllStringFunc(text, func(s string) string {
		return strings.ReplaceAll(s, " ", "")
	})
	return gcNumberPattern.ReplaceAllStringFunc(text, func(token string) string {
		normalized := strings.ReplaceAll(strings.TrimSpace(token), ",", ".")
		if gcNumberAllowed(normalized, allowed) {
			return token
		}
		return ""
	})
}

func parseGCNumberToken(value string) (float64, bool) {
	n := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func repairGlobalContextFactReferences(ctx *aicontext.GlobalSemanticContext, input GlobalContextGenerationInput) {
	allowed := globalContextAllowedFactIDs(input)
	filter := func(ids []string) []string {
		if len(ids) == 0 {
			return nil
		}
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := allowed[id]; !ok {
				continue
			}
			out = append(out, id)
		}
		return compactStrings(out)
	}
	ctx.SourceFactIDs = filter(ctx.SourceFactIDs)
	repairRiskFacts := func(risks []aicontext.SemanticRisk) {
		for i := range risks {
			risks[i].EvidenceFactIDs = filter(risks[i].EvidenceFactIDs)
			for j := range risks[i].Entities {
				risks[i].Entities[j].FactIDs = filter(risks[i].Entities[j].FactIDs)
			}
		}
	}
	repairSignalFacts := func(signals []aicontext.SemanticSignal) {
		for i := range signals {
			signals[i].EvidenceFactIDs = filter(signals[i].EvidenceFactIDs)
			for j := range signals[i].Entities {
				signals[i].Entities[j].FactIDs = filter(signals[i].Entities[j].FactIDs)
			}
		}
	}
	repairRiskFacts(ctx.PrimaryRisks)
	repairRiskFacts(ctx.SecondaryRisks)
	repairSignalFacts(ctx.Opportunities)
}

func normalizeGlobalContextCategory(category string) string {
	cat := strings.TrimSpace(strings.ToLower(category))
	switch cat {
	case "product_data", "products", "product", "enrichment", "product_enrichment":
		return "sku"
	case "cancellations", "surplus", "returns":
		return "sales"
	case "campaign", "campaigns", "ads":
		return "advertising"
	case "inventory":
		return "stock"
	default:
		return strings.TrimSpace(category)
	}
}

func inferGlobalContextSourceFactIDs(ctx aicontext.GlobalSemanticContext, input GlobalContextGenerationInput) []string {
	allowed := globalContextAllowedFactIDs(input)
	seen := make(map[string]struct{})
	add := func(ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := allowed[id]; !ok {
				continue
			}
			seen[id] = struct{}{}
		}
	}
	add(ctx.SourceFactIDs)
	for _, risk := range ctx.PrimaryRisks {
		add(risk.EvidenceFactIDs)
		for _, ent := range risk.Entities {
			add(ent.FactIDs)
		}
	}
	for _, risk := range ctx.SecondaryRisks {
		add(risk.EvidenceFactIDs)
		for _, ent := range risk.Entities {
			add(ent.FactIDs)
		}
	}
	for _, signal := range ctx.Opportunities {
		add(signal.EvidenceFactIDs)
		for _, ent := range signal.Entities {
			add(ent.FactIDs)
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
