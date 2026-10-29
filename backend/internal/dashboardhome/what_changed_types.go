package dashboardhome

import "strings"

const (
	whatChangedKeyWorsened       = "worsened"
	whatChangedKeyNewRisks       = "new_risks"
	whatChangedKeyUnchangedRisks = "unchanged_risks"
	whatChangedKeyImproved       = "improved"
)

var whatChangedRequiredGroups = []struct {
	Key   string
	Label string
	Tone  string
}{
	{Key: whatChangedKeyWorsened, Label: "Ухудшилось", Tone: "negative"},
	{Key: whatChangedKeyNewRisks, Label: "Новые риски", Tone: "warning"},
	{Key: whatChangedKeyUnchangedRisks, Label: "Активные проблемы", Tone: "neutral"},
	{Key: whatChangedKeyImproved, Label: "Улучшилось", Tone: "positive"},
}

var whatChangedActionVerbPatterns = []string{
	"проверьте",
	"остановите",
	"сделайте",
	"пополните",
	"сократите",
	"запланируйте",
}

type dashboardNarrativeWhatChanged struct {
	Summary        string                               `json:"summary"`
	MainConclusion string                               `json:"main_conclusion,omitempty"`
	Groups         []dashboardNarrativeWhatChangedGroup `json:"groups"`
}

type dashboardNarrativeWhatChangedGroup struct {
	Key       string                              `json:"key"`
	Label     string                              `json:"label"`
	Tone      string                              `json:"tone"`
	Summary   string                              `json:"summary"`
	Items     []dashboardNarrativeWhatChangedItem `json:"items"`
	EmptyText string                              `json:"empty_text"`
}

type dashboardNarrativeWhatChangedItem struct {
	Label             string   `json:"label"`
	Text              string   `json:"text"`
	Highlights        []string `json:"highlights"`
	Domain            string   `json:"domain,omitempty"`
	Severity          string   `json:"severity,omitempty"`
	CurrentValueText  string   `json:"current_value_text,omitempty"`
	PreviousValueText string   `json:"previous_value_text,omitempty"`
	DeltaText         string   `json:"delta_text,omitempty"`
	ImpactText        string   `json:"impact_text,omitempty"`
	EntityCount       int      `json:"entity_count,omitempty"`
	EntityNames       []string `json:"entity_names,omitempty"`
	LinkedActionID    string   `json:"linked_action_id,omitempty"`
	ActionLinkLabel   string   `json:"action_link_label,omitempty"`
	DeepLink          string   `json:"deep_link,omitempty"`
	ChangeID          string   `json:"change_id,omitempty"`
}

type whatChangedFact struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	Label         string `json:"label"`
	Direction     string `json:"direction"`
	CurrentValue  string `json:"current_value"`
	PreviousValue string `json:"previous_value,omitempty"`
	DeltaAbs      string `json:"delta_abs,omitempty"`
	DeltaPct      string `json:"delta_pct,omitempty"`
	EntityName    string `json:"entity_name,omitempty"`
	Note          string `json:"note,omitempty"`
}

type dashboardNarrativeWhatChangedContext struct {
	CurrentMetricDate  string                             `json:"current_metric_date"`
	PreviousMetricDate string                             `json:"previous_metric_date,omitempty"`
	ImprovedFacts      []whatChangedFact                  `json:"improved_facts"`
	WorsenedFacts      []whatChangedFact                  `json:"worsened_facts"`
	NewRiskFacts       []whatChangedFact                  `json:"new_risk_facts"`
	UnchangedRiskFacts []whatChangedFact                  `json:"unchanged_risk_facts"`
	CampaignNames      []string                           `json:"campaign_names,omitempty"`
	ProductNames       []string                           `json:"product_names,omitempty"`
	AllowedNumbers     []string                           `json:"allowed_numbers"`
	PeriodComparison   dashboardNarrativePeriodComparison `json:"period_comparison,omitempty"`
}

type whatChangedValidationBundle struct {
	Context        dashboardNarrativeWhatChangedContext
	AllowedNumbers map[string]struct{}
	AllowedNames   map[string]struct{}
}

func toneForWhatChangedKey(key string) string {
	for _, group := range whatChangedRequiredGroups {
		if group.Key == key {
			return group.Tone
		}
	}
	return ""
}

func labelForWhatChangedKey(key string) string {
	for _, group := range whatChangedRequiredGroups {
		if group.Key == key {
			return group.Label
		}
	}
	return ""
}

func isAllowedWhatChangedKey(key string) bool {
	switch key {
	case whatChangedKeyWorsened, whatChangedKeyNewRisks, whatChangedKeyUnchangedRisks, whatChangedKeyImproved:
		return true
	default:
		return false
	}
}

func labelContainsCount(label string) bool {
	label = strings.ToLower(strings.TrimSpace(label))
	return strings.Contains(label, "·") || strings.Contains(label, "•")
}
