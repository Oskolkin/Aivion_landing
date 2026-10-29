package openaix

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseResponsesJSON(raw []byte) (ResponsesResult, error) {
	var body providerErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return ResponsesResult{}, fmt.Errorf("openaix: malformed provider JSON: %w", err)
	}
	out := ResponsesResult{ID: body.ID, Status: body.Status}
	if body.Error != nil && strings.TrimSpace(body.Error.Message) != "" && strings.EqualFold(body.Status, "failed") {
		out.Status = body.Status
	}
	if body.IncompleteDetails != nil {
		out.IncompleteReason = body.IncompleteDetails.Reason
	}
	if body.Usage != nil {
		out.Usage.InputTokens = body.Usage.InputTokens
		out.Usage.OutputTokens = body.Usage.OutputTokens
		out.Usage.TotalTokens = body.Usage.TotalTokens
		if body.Usage.InputTokensDetails != nil {
			out.Usage.CachedInputTokens = body.Usage.InputTokensDetails.CachedTokens
		}
		if body.Usage.OutputTokensDetails != nil {
			out.Usage.ReasoningTokens = body.Usage.OutputTokensDetails.ReasoningTokens
		}
	}
	out.OutputText, out.Refusal = extractOutputText(body.Output)
	if strings.EqualFold(out.Status, "incomplete") {
		return out, nil
	}
	if out.Refusal != "" {
		if out.Status == "" {
			out.Status = "refused"
		}
		return out, nil
	}
	return out, nil
}

func extractOutputText(raw json.RawMessage) (text, refusal string) {
	if len(raw) == 0 {
		return "", ""
	}
	var items []outputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		var wrapped struct {
			Output []outputItem `json:"output"`
		}
		if json.Unmarshal(raw, &wrapped) == nil {
			items = wrapped.Output
		}
	}
	var b strings.Builder
	for _, it := range items {
		for _, c := range it.Content {
			if c.Refusal != "" {
				refusal = c.Refusal
			}
			switch {
			case c.Text != "":
				b.WriteString(c.Text)
			case c.OutputText != "":
				b.WriteString(c.OutputText)
			}
			if c.Type == "refusal" && c.Refusal == "" && c.Text != "" {
				refusal = c.Text
			}
		}
	}
	return b.String(), refusal
}
