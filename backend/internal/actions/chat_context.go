package actions

import (
	"context"
	"fmt"
	"strings"
)

// ActionChatContext is a compact, seller-safe action context for chat prompts.
type ActionChatContext struct {
	ActionKey          string
	Title              string
	Type               string
	Category           string
	Priority           string
	Status             string
	MetricDate         string
	Summary            string
	Reason             string
	NextStep           string
	ExpectedEffect     string
	Evidence           []string
	EvidenceFactIDs    []string
	ObjectActions      []string
	CompletionCriteria string
	Limitations        []string
	PipelineRunID      int64
	ActionsRunID       int64
	DataPackHash       string
	GlobalContextHash  string
}

// GetActionForChatContext returns a compact action context for chat usage.
func (s *Service) GetActionForChatContext(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionChatContext, error) {
	item, err := s.GetAction(ctx, sellerAccountID, actionKey)
	if err != nil {
		return nil, err
	}
	out := BuildActionChatContext(*item)
	return &out, nil
}

func BuildActionChatContext(action ActionItem) ActionChatContext {
	out := ActionChatContext{
		ActionKey:          action.ActionKey,
		Title:              strings.TrimSpace(action.Title),
		Type:               string(action.Type),
		Category:           string(action.Category),
		Priority:           string(action.Priority),
		Status:             string(action.Status),
		MetricDate:         strings.TrimSpace(action.MetricDate),
		Summary:            strings.TrimSpace(action.Summary),
		Reason:             strings.TrimSpace(action.Reason),
		NextStep:           strings.TrimSpace(action.NextStep),
		ExpectedEffect:     strings.TrimSpace(action.ExpectedEffect),
		Limitations:        compactLimitations(action.Limitations, 4),
		Evidence:           compactEvidence(action.Evidence, 5),
		EvidenceFactIDs:    compactStrings(action.EvidenceFactIDs, 8),
		ObjectActions:      compactObjectActions(action.OperationalDetail, 6),
		CompletionCriteria: strings.TrimSpace(action.CompletionCriteria),
		ActionsRunID:       action.RunID,
	}
	if action.Source != nil {
		out.PipelineRunID = action.Source.PipelineRunID
		out.ActionsRunID = action.Source.ActionsRunID
		out.DataPackHash = action.Source.DataPackHash
		out.GlobalContextHash = action.Source.GlobalContextHash
	}
	if len(action.VisibleLimitations) > 0 {
		out.Limitations = compactLimitations(action.VisibleLimitations, 4)
	}
	return out
}

func compactEvidence(items []ActionEvidence, max int) []string {
	if max <= 0 {
		return nil
	}
	out := make([]string, 0, max)
	for _, item := range items {
		if len(out) >= max {
			break
		}
		label := strings.TrimSpace(item.Label)
		value := strings.TrimSpace(item.Value)
		unit := strings.TrimSpace(item.Unit)
		if label == "" && value == "" {
			continue
		}
		line := strings.TrimSpace(fmt.Sprintf("%s: %s", label, value))
		if label == "" {
			line = value
		}
		if unit != "" {
			line = strings.TrimSpace(line + " " + unit)
		}
		out = append(out, line)
	}
	return out
}

func compactLimitations(items []string, max int) []string {
	if max <= 0 {
		return nil
	}
	out := make([]string, 0, max)
	for _, item := range items {
		if len(out) >= max {
			break
		}
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func compactStrings(items []string, max int) []string {
	if max <= 0 {
		return nil
	}
	out := make([]string, 0, max)
	for _, item := range items {
		if len(out) >= max {
			break
		}
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func compactObjectActions(detail *OperationalDetail, max int) []string {
	if detail == nil || max <= 0 {
		return nil
	}
	out := make([]string, 0, max)
	for _, obj := range detail.ObjectActions {
		if len(out) >= max {
			break
		}
		name := strings.TrimSpace(obj.Name)
		if name == "" {
			name = strings.TrimSpace(obj.EntityID)
		}
		if name == "" {
			continue
		}
		action := strings.TrimSpace(obj.RecommendedAction)
		if action == "" {
			out = append(out, name)
			continue
		}
		out = append(out, name+": "+action)
	}
	return out
}
