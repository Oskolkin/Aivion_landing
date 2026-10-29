package aipipeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

var gcNumberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
var gcSpacedThousandsPattern = regexp.MustCompile(`\d{1,3}(?:\s\d{3})+`)

func ValidateGlobalSemanticContext(
	input GlobalContextGenerationInput,
	ctx aicontext.GlobalSemanticContext,
) GlobalContextValidationResult {
	result := GlobalContextValidationResult{Status: aicontext.ValidationStatusValid}
	add := func(severity, code, message, field string) {
		result.Issues = append(result.Issues, GlobalContextValidationIssue{
			Severity: severity, Code: code, Message: message, Field: field,
		})
	}

	if input.DataPack == nil {
		add(aicontext.IssueSeverityFatal, "missing_data_pack", "backend data pack is required", "data_pack")
		return finalizeGlobalContextValidation(result)
	}

	factAllowed := globalContextAllowedFactIDs(input)
	entityPack := *input.DataPack
	allowedNumbers := collectGlobalContextAllowedNumbers(input)

	if strings.TrimSpace(ctx.SchemaVersion) == "" {
		add(aicontext.IssueSeverityFatal, "missing_schema_version", "schema_version is required", "schema_version")
	}
	if strings.TrimSpace(ctx.OverallStory) == "" {
		add(aicontext.IssueSeverityFatal, "missing_overall_story", "overall_story is required", "overall_story")
	}
	if strings.TrimSpace(ctx.Confidence) == "" {
		add(aicontext.IssueSeverityFatal, "missing_confidence", "confidence is required", "confidence")
	} else if !containsString(aicontext.AllowedConfidenceValues, ctx.Confidence) {
		add(aicontext.IssueSeverityFatal, "invalid_confidence", fmt.Sprintf("unknown confidence %q", ctx.Confidence), "confidence")
	}
	if len(ctx.SourceFactIDs) == 0 {
		add(aicontext.IssueSeverityFatal, "missing_source_fact_ids", "source_fact_ids is required", "source_fact_ids")
	}

	for _, id := range ctx.SourceFactIDs {
		if _, ok := factAllowed[id]; !ok {
			add(aicontext.IssueSeverityFatal, "unknown_source_fact_id", fmt.Sprintf("unknown fact_id %q", id), "source_fact_ids")
		}
	}

	validateRiskList := func(risks []aicontext.SemanticRisk, prefix string) {
		for i, risk := range risks {
			field := fmt.Sprintf("%s[%d]", prefix, i)
			validateCategory(risk.Category, field, add)
			if strings.TrimSpace(risk.RiskID) == "" {
				add(aicontext.IssueSeverityRepairable, "missing_risk_id", "risk_id is required", field+".risk_id")
			}
			if strings.TrimSpace(risk.Title) == "" {
				add(aicontext.IssueSeverityFatal, "missing_risk_title", "title is required", field+".title")
			}
			if risk.Severity != "" && !containsString(aicontext.AllowedSeverityValues, risk.Severity) {
				add(aicontext.IssueSeverityRepairable, "invalid_severity", fmt.Sprintf("unknown severity %q", risk.Severity), field+".severity")
			}
			for _, id := range risk.EvidenceFactIDs {
				if _, ok := factAllowed[id]; !ok {
					add(aicontext.IssueSeverityFatal, "unknown_evidence_fact_id", fmt.Sprintf("unknown fact_id %q", id), field+".evidence_fact_ids")
				}
			}
			for _, ent := range risk.Entities {
				validateGlobalContextEntity(ent, input, entityPack, field+".entities", add)
			}
			for _, fieldText := range []struct{ n, v string }{
				{field + ".title", risk.Title},
				{field + ".rationale", risk.Rationale},
			} {
				validateNumbersInText(fieldText.v, fieldText.n, allowedNumbers, add)
			}
		}
	}

	validateSignalList := func(signals []aicontext.SemanticSignal, prefix string) {
		for i, signal := range signals {
			field := fmt.Sprintf("%s[%d]", prefix, i)
			validateCategory(signal.Category, field, add)
			if strings.TrimSpace(signal.SignalID) == "" {
				add(aicontext.IssueSeverityRepairable, "missing_signal_id", "signal_id is required", field+".signal_id")
			}
			if strings.TrimSpace(signal.Title) == "" {
				add(aicontext.IssueSeverityFatal, "missing_signal_title", "title is required", field+".title")
			}
			for _, id := range signal.EvidenceFactIDs {
				if _, ok := factAllowed[id]; !ok {
					add(aicontext.IssueSeverityFatal, "unknown_evidence_fact_id", fmt.Sprintf("unknown fact_id %q", id), field+".evidence_fact_ids")
				}
			}
			for _, ent := range signal.Entities {
				validateGlobalContextEntity(ent, input, entityPack, field+".entities", add)
			}
			for _, fieldText := range []struct{ n, v string }{
				{field + ".title", signal.Title},
				{field + ".rationale", signal.Rationale},
			} {
				validateNumbersInText(fieldText.v, fieldText.n, allowedNumbers, add)
			}
		}
	}

	validateRiskList(ctx.PrimaryRisks, "primary_risks")
	validateRiskList(ctx.SecondaryRisks, "secondary_risks")
	validateSignalList(ctx.Opportunities, "opportunities")

	for i, cat := range ctx.PriorityOrder {
		validateCategory(cat, fmt.Sprintf("priority_order[%d]", i), add)
	}

	validateNumbersInText(ctx.OverallStory, "overall_story", allowedNumbers, add)
	for _, issue := range DetectUnresolvedPlaceholdersInFields(map[string]string{
		"overall_story": ctx.OverallStory,
	}) {
		add(aicontext.IssueSeverityWarning, GlobalContextUnresolvedNumericPlaceholderCode,
			fmt.Sprintf("unresolved numeric placeholder %q", issue.MatchedFragment), issue.Field)
	}
	for i, lim := range ctx.DataLimitations {
		validateNumbersInText(lim, fmt.Sprintf("data_limitations[%d]", i), allowedNumbers, add)
	}

	validateForbiddenClaims(ctx, add)

	if input.MaxPrimaryRisks > 0 && len(ctx.PrimaryRisks) > input.MaxPrimaryRisks {
		add(aicontext.IssueSeverityWarning, "too_many_primary_risks", "primary_risks exceeds max", "primary_risks")
	}
	if input.MaxSecondaryRisks > 0 && len(ctx.SecondaryRisks) > input.MaxSecondaryRisks {
		add(aicontext.IssueSeverityWarning, "too_many_secondary_risks", "secondary_risks exceeds max", "secondary_risks")
	}
	if input.MaxOpportunities > 0 && len(ctx.Opportunities) > input.MaxOpportunities {
		add(aicontext.IssueSeverityWarning, "too_many_opportunities", "opportunities exceeds max", "opportunities")
	}

	return finalizeGlobalContextValidation(result)
}

func validateCategory(category, field string, add func(severity, code, message, field string)) {
	cat := strings.TrimSpace(category)
	if cat == "" {
		add(aicontext.IssueSeverityFatal, "missing_category", "category is required", field)
		return
	}
	if aicontext.IsForbiddenCategory(cat) {
		add(aicontext.IssueSeverityFatal, "forbidden_category", fmt.Sprintf("forbidden category %q", cat), field)
		return
	}
	if !aicontext.IsAllowedCategory(cat) {
		add(aicontext.IssueSeverityFatal, "invalid_category", fmt.Sprintf("unknown category %q", cat), field)
	}
}

func globalContextAllowedFactIDs(input GlobalContextGenerationInput) map[string]struct{} {
	out := make(map[string]struct{})
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		for id := range input.FactCatalog {
			out[id] = struct{}{}
		}
		return out
	}
	if input.DataPack != nil {
		for id := range input.DataPack.FactRegistry {
			out[id] = struct{}{}
		}
	}
	return out
}

func validateGlobalContextEntity(
	ent aicontext.SemanticEntityRef,
	input GlobalContextGenerationInput,
	pack aidatapack.SellerAIDataPack,
	field string,
	add func(severity, code, message, field string),
) {
	entityType := strings.TrimSpace(ent.EntityType)
	entityID := strings.TrimSpace(ent.EntityID)
	if entityType == "" || entityID == "" {
		add(aicontext.IssueSeverityFatal, "missing_entity_ref", "entity_type and entity_id are required", field)
		return
	}
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		if _, ok := input.EntityCatalog[entityID]; !ok {
			add(aicontext.IssueSeverityFatal, "unknown_entity", fmt.Sprintf("unknown entity %s:%s", entityType, entityID), field)
			return
		}
	} else if !entityExists(pack, entityType, entityID) {
		add(aicontext.IssueSeverityFatal, "unknown_entity", fmt.Sprintf("unknown entity %s:%s", entityType, entityID), field)
	}
	factAllowed := globalContextAllowedFactIDs(input)
	for _, id := range ent.FactIDs {
		if _, ok := factAllowed[id]; !ok {
			add(aicontext.IssueSeverityFatal, "unknown_entity_fact_id", fmt.Sprintf("unknown fact_id %q", id), field+".fact_ids")
		}
	}
}

func validateEntity(ent aicontext.SemanticEntityRef, pack aidatapack.SellerAIDataPack, field string, add func(severity, code, message, field string)) {
	entityType := strings.TrimSpace(ent.EntityType)
	entityID := strings.TrimSpace(ent.EntityID)
	if entityType == "" || entityID == "" {
		add(aicontext.IssueSeverityFatal, "missing_entity_ref", "entity_type and entity_id are required", field)
		return
	}
	if !entityExists(pack, entityType, entityID) {
		add(aicontext.IssueSeverityFatal, "unknown_entity", fmt.Sprintf("unknown entity %s:%s", entityType, entityID), field)
	}
	for _, id := range ent.FactIDs {
		if _, ok := pack.FactRegistry[id]; !ok {
			add(aicontext.IssueSeverityFatal, "unknown_entity_fact_id", fmt.Sprintf("unknown fact_id %q", id), field+".fact_ids")
		}
	}
}

func entityExists(pack aidatapack.SellerAIDataPack, entityType, entityID string) bool {
	switch entityType {
	case "sku":
		_, ok := pack.EntityRegistry.SKUs[entityID]
		return ok
	case "sku_group":
		_, ok := pack.EntityRegistry.SKUGroups[entityID]
		return ok
	case "campaign":
		_, ok := pack.EntityRegistry.Campaigns[entityID]
		return ok
	case "category":
		_, ok := pack.EntityRegistry.Categories[entityID]
		return ok
	case "warehouse":
		_, ok := pack.EntityRegistry.Warehouses[entityID]
		return ok
	case "account":
		return entityID == "account:seller"
	case "funnel":
		return entityID == "funnel:account"
	default:
		return false
	}
}

func validateForbiddenClaims(ctx aicontext.GlobalSemanticContext, add func(severity, code, message, field string)) {
	checks := []struct {
		code    string
		markers []string
	}{
		{"forbidden_margin_claim", []string{"маржинальност", "маржа ", "margin"}},
		{"forbidden_profit_claim", []string{"прибыл", "profit"}},
		{"forbidden_repricing_claim", []string{"repricing", "репрайс", "переоцен"}},
		{"forbidden_sku_ad_claim", []string{"sku-level реклам", "sku level реклам", "рекламная эффективность sku"}},
		{"forbidden_autopause_claim", []string{"автопауз", "автоматически останов", "автоматически изменить ставк"}},
	}

	scan := func(text, field string) {
		lower := strings.ToLower(text)
		for _, check := range checks {
			for _, marker := range check.markers {
				if strings.Contains(lower, marker) {
					add(aicontext.IssueSeverityFatal, check.code, "forbidden claim detected", field)
					return
				}
			}
		}
	}

	scan(ctx.OverallStory, "overall_story")
	for i, r := range ctx.PrimaryRisks {
		scan(r.Title, fmt.Sprintf("primary_risks[%d].title", i))
		scan(r.Rationale, fmt.Sprintf("primary_risks[%d].rationale", i))
	}
	for i, r := range ctx.SecondaryRisks {
		scan(r.Title, fmt.Sprintf("secondary_risks[%d].title", i))
		scan(r.Rationale, fmt.Sprintf("secondary_risks[%d].rationale", i))
	}
	for i, s := range ctx.Opportunities {
		scan(s.Title, fmt.Sprintf("opportunities[%d].title", i))
		scan(s.Rationale, fmt.Sprintf("opportunities[%d].rationale", i))
	}
	// data_limitations intentionally describe unavailable finance/margin data — not seller-facing claims.
}

func collectGlobalContextAllowedNumbers(input GlobalContextGenerationInput) map[string]bool {
	out := make(map[string]bool)
	add := func(v string) {
		for _, n := range extractGCNumbers(v) {
			out[n] = true
		}
	}
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		for _, fact := range input.FactCatalog {
			add(fact.FormattedValue)
			add(fact.FactID)
		}
	} else if input.DataPack != nil {
		for _, fact := range input.DataPack.FactRegistry {
			add(fact.FormattedValue)
			add(fact.FactID)
		}
	}
	if input.DataPack != nil {
		if input.DataPack.DataWindows.Last7d.From != "" {
			add("7")
			add("14")
		}
		for _, id := range input.DataPack.EntityRegistry.SKUs {
			add(id.SKU)
			add(id.OfferID)
		}
		for _, id := range input.DataPack.EntityRegistry.Campaigns {
			add(id.CampaignExternalID)
		}
	}
	add(input.MetricDate)
	return out
}

func validateNumbersInText(text, field string, allowed map[string]bool, add func(severity, code, message, field string)) {
	for _, num := range extractGCNumbers(text) {
		if gcNumberAllowed(num, allowed) {
			continue
		}
		add(aicontext.IssueSeverityRepairable, "unsupported_number", fmt.Sprintf("unsupported number %q in %s", num, field), field)
		break
	}
}

func extractGCNumbers(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	text = gcSpacedThousandsPattern.ReplaceAllStringFunc(text, func(s string) string {
		return strings.ReplaceAll(s, " ", "")
	})
	matches := gcNumberPattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.ReplaceAll(strings.TrimSpace(m), ",", "."))
	}
	return out
}

func gcNumberAllowed(num string, allowed map[string]bool) bool {
	n := strings.ReplaceAll(strings.TrimSpace(num), ",", ".")
	if allowed[n] {
		return true
	}
	if f, err := strconv.ParseFloat(n, 64); err == nil {
		if f == float64(int64(f)) && f >= 0 && f <= 14 {
			return true // analysis windows 7/14 days
		}
	}
	return false
}

func finalizeGlobalContextValidation(result GlobalContextValidationResult) GlobalContextValidationResult {
	hasFatal := false
	hasRepairable := false
	hasWarning := false
	for _, issue := range result.Issues {
		switch issue.Severity {
		case aicontext.IssueSeverityFatal:
			hasFatal = true
		case aicontext.IssueSeverityRepairable:
			hasRepairable = true
		case aicontext.IssueSeverityWarning:
			hasWarning = true
		}
	}
	switch {
	case hasFatal:
		result.Status = aicontext.ValidationStatusInvalid
	case hasRepairable:
		result.Status = aicontext.ValidationStatusRepairable
	case hasWarning:
		result.Status = aicontext.ValidationStatusValidWithWarnings
	default:
		result.Status = aicontext.ValidationStatusValid
	}
	return result
}

// NormalizeGlobalContextConfidenceAfterValidation downgrades confidence when placeholders or warnings exist.
func NormalizeGlobalContextConfidenceAfterValidation(ctx *aicontext.GlobalSemanticContext, result GlobalContextValidationResult) {
	if ctx == nil {
		return
	}
	hasPlaceholder := false
	hasWarning := false
	for _, issue := range result.Issues {
		if issue.Code == GlobalContextUnresolvedNumericPlaceholderCode {
			hasPlaceholder = true
		}
		if issue.Severity == aicontext.IssueSeverityWarning {
			hasWarning = true
		}
	}
	if !hasPlaceholder && !hasWarning {
		return
	}
	if ctx.Confidence == "high" {
		ctx.Confidence = "medium"
	}
	if hasPlaceholder && ctx.Confidence == "medium" {
		ctx.Confidence = "low"
	}
}

func globalContextValidationAccepted(status string) bool {
	switch status {
	case aicontext.ValidationStatusValid, aicontext.ValidationStatusValidWithWarnings, aicontext.ValidationStatusRepairable:
		return true
	default:
		return false
	}
}

func countFatalGlobalContextIssues(issues []GlobalContextValidationIssue) []GlobalContextValidationIssue {
	if len(issues) == 0 {
		return nil
	}
	out := make([]GlobalContextValidationIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.Severity == aicontext.IssueSeverityFatal {
			out = append(out, issue)
		}
	}
	return out
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// NormalizeGlobalSemanticContext trims whitespace and drops empty optional slices.
// Backend may normalize nil/empty but must not rewrite semantic meaning.
func NormalizeGlobalSemanticContext(ctx *aicontext.GlobalSemanticContext) {
	if ctx == nil {
		return
	}
	ctx.OverallStory = strings.TrimSpace(ctx.OverallStory)
	ctx.Confidence = strings.TrimSpace(ctx.Confidence)
	ctx.DataLimitations = compactStrings(ctx.DataLimitations)
	ctx.SourceFactIDs = compactStrings(ctx.SourceFactIDs)
	ctx.PriorityOrder = compactStrings(ctx.PriorityOrder)
}

func compactStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
