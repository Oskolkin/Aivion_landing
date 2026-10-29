package aidatapack

import (
	"strings"
)

const (
	FinalMetricSemanticMismatchCode       = "final_metric_semantic_mismatch"
	FinalUnsupportedMetricClaimCode       = "final_unsupported_metric_claim"
	FinalUnresolvedNumericPlaceholderCode = "final_unresolved_numeric_placeholder"
)

// FinalSemanticIssue records a blocking final-output semantic validation finding.
type FinalSemanticIssue struct {
	Code            string   `json:"issue_code"`
	Severity        string   `json:"severity"`
	FieldPath       string   `json:"field_path,omitempty"`
	MetricCode      string   `json:"metric_code,omitempty"`
	ForbiddenPhrase string   `json:"forbidden_phrase,omitempty"`
	ContextSnippet  string   `json:"context_snippet,omitempty"`
	ActionKey       string   `json:"action_key,omitempty"`
	EntityID        string   `json:"entity_id,omitempty"`
	SourceFactIDs   []string `json:"source_fact_ids,omitempty"`
}

var shelfLifeClaimTerms = []string{
	"срок хранения",
	"срок годности",
	"дни хранения",
	"количество дней хранения",
	"товар хранится",
}

// DataPackHasShelfLifeMetric reports whether the pack contains a canonical shelf-life metric.
func DataPackHasShelfLifeMetric(pack *SellerAIDataPack) bool {
	if pack == nil {
		return false
	}
	for _, fact := range pack.FactRegistry {
		metric := strings.ToLower(strings.TrimSpace(fact.Metric))
		if metric == "" {
			continue
		}
		if strings.Contains(metric, "shelf_life") || strings.Contains(metric, "shelf-life") || metric == "shelf_life_days" {
			return true
		}
	}
	return false
}

// MetricCodesFromFactIDs resolves metric codes for canonical fact IDs.
func MetricCodesFromFactIDs(pack *SellerAIDataPack, factIDs []string) []string {
	if pack == nil || len(factIDs) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, id := range factIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		fact, ok := pack.FactRegistry[id]
		if !ok {
			continue
		}
		metric := strings.TrimSpace(fact.Metric)
		if metric == "" {
			continue
		}
		if _, exists := seen[metric]; exists {
			continue
		}
		seen[metric] = struct{}{}
		out = append(out, metric)
	}
	return out
}

// DetectUnsupportedShelfLifeClaim flags shelf-life wording without backing metric in pack.
func DetectUnsupportedShelfLifeClaim(text string, pack *SellerAIDataPack, fieldPath string) []FinalSemanticIssue {
	if DataPackHasShelfLifeMetric(pack) {
		return nil
	}
	textLower := strings.ToLower(strings.TrimSpace(text))
	if textLower == "" {
		return nil
	}
	var issues []FinalSemanticIssue
	for _, term := range shelfLifeClaimTerms {
		if strings.Contains(textLower, term) {
			issues = append(issues, FinalSemanticIssue{
				Code:            FinalUnsupportedMetricClaimCode,
				Severity:        "blocking",
				FieldPath:       fieldPath,
				ForbiddenPhrase: term,
				ContextSnippet:  snippetAround(text, term, 60),
			})
			break
		}
	}
	return issues
}

// FinalMetricSemanticIssues converts metric semantic mismatches to blocking final issues.
func FinalMetricSemanticIssues(text string, metricCodes []string, fieldPath string, sourceFactIDs []string) []FinalSemanticIssue {
	issues := DetectMetricSemanticMismatches(text, metricCodes, fieldPath)
	if len(issues) == 0 {
		return nil
	}
	out := make([]FinalSemanticIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, FinalSemanticIssue{
			Code:            FinalMetricSemanticMismatchCode,
			Severity:        "blocking",
			FieldPath:       fieldPath,
			MetricCode:      issue.MetricCode,
			ForbiddenPhrase: issue.ForbiddenPhrase,
			ContextSnippet:  issue.ContextSnippet,
			SourceFactIDs:   append([]string(nil), sourceFactIDs...),
		})
	}
	return out
}
