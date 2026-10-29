package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rawOpenAIResponse struct {
	ID                string          `json:"id"`
	Status            string          `json:"status"`
	Model             string          `json:"model"`
	CreatedAt         json.RawMessage `json:"created_at"`
	CompletedAt       json.RawMessage `json:"completed_at"`
	Output            json.RawMessage `json:"output"`
	OutputText        string          `json:"output_text"`
	Usage             json.RawMessage `json:"usage"`
	Error             json.RawMessage `json:"error"`
	IncompleteDetails json.RawMessage `json:"incomplete_details"`
	Refusal           string          `json:"refusal"`
}

type rawOutputItem struct {
	Type                string          `json:"type"`
	ID                  string          `json:"id"`
	Status              string          `json:"status"`
	ContainerID         string          `json:"container_id"`
	Code                string          `json:"code"`
	Content             json.RawMessage `json:"content"`
	CodeInterpreterCall json.RawMessage `json:"code_interpreter_call"`
	Refusal             string          `json:"refusal"`
}

type rawMessageContent struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}

type rawUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

type rawResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rawIncompleteDetails struct {
	Reason string `json:"reason"`
}

// ParseResponseJSON extracts normalized fields from an OpenAI Response JSON payload.
func ParseResponseJSON(raw []byte) (*ProviderResponse, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty response json")
	}
	var decoded rawOpenAIResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal response json: %w", err)
	}

	resp := &ProviderResponse{
		ID:        strings.TrimSpace(decoded.ID),
		Status:    ProviderResponseStatus(strings.TrimSpace(decoded.Status)),
		Model:     strings.TrimSpace(decoded.Model),
		CreatedAt: firstTime(decoded.CreatedAt),
		RawJSON:   append([]byte(nil), raw...),
		Refusal:   strings.TrimSpace(decoded.Refusal),
	}
	if t := optionalUnixTime(decoded.CompletedAt); t != nil {
		resp.CompletedAt = t
	}

	if len(decoded.Usage) > 0 {
		var usage rawUsage
		if err := json.Unmarshal(decoded.Usage, &usage); err == nil {
			resp.Usage = ProviderUsage{
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				TotalTokens:  usage.TotalTokens,
			}
		}
	}

	if len(decoded.Error) > 0 && string(decoded.Error) != "null" {
		var errPayload rawResponseError
		if err := json.Unmarshal(decoded.Error, &errPayload); err == nil {
			resp.Error = &ProviderResponseError{
				Code:    strings.TrimSpace(errPayload.Code),
				Message: strings.TrimSpace(errPayload.Message),
			}
		}
	}

	if len(decoded.IncompleteDetails) > 0 && string(decoded.IncompleteDetails) != "null" {
		var details rawIncompleteDetails
		if err := json.Unmarshal(decoded.IncompleteDetails, &details); err == nil {
			resp.IncompleteDetails = &IncompleteDetails{
				Reason: strings.TrimSpace(details.Reason),
			}
		}
	}

	outputText, refusal, calls, messageCount, itemTypes := extractOutput(decoded.Output)
	if strings.TrimSpace(decoded.OutputText) != "" {
		outputText = strings.TrimSpace(decoded.OutputText)
	}
	if refusal != "" && resp.Refusal == "" {
		resp.Refusal = refusal
	}
	resp.OutputText = outputText
	resp.CodeInterpreterCalls = calls
	resp.AssistantMessageCount = messageCount
	resp.OutputItemTypes = itemTypes
	return resp, nil
}

func extractOutput(raw json.RawMessage) (text string, refusal string, calls []CodeInterpreterCall, messageCount int, itemTypes []string) {
	if len(raw) == 0 {
		return "", "", nil, 0, nil
	}
	var items []rawOutputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", "", nil, 0, nil
	}

	var textParts []string
	for _, item := range items {
		itemType := strings.TrimSpace(item.Type)
		if itemType != "" {
			itemTypes = append(itemTypes, itemType)
		}
		switch itemType {
		case "message":
			messageCount++
			if strings.TrimSpace(item.Refusal) != "" && refusal == "" {
				refusal = strings.TrimSpace(item.Refusal)
			}
			if len(item.Content) > 0 {
				var content []rawMessageContent
				if err := json.Unmarshal(item.Content, &content); err == nil {
					for _, c := range content {
						if strings.TrimSpace(c.Refusal) != "" && refusal == "" {
							refusal = strings.TrimSpace(c.Refusal)
						}
						if c.Type == "output_text" || c.Type == "text" {
							if t := strings.TrimSpace(c.Text); t != "" {
								textParts = append(textParts, t)
							}
						}
						if c.Type == "refusal" {
							r := strings.TrimSpace(c.Refusal)
							if r == "" {
								r = strings.TrimSpace(c.Text)
							}
							if r != "" && refusal == "" {
								refusal = r
							}
						}
					}
				}
			}
		case "code_interpreter_call":
			calls = append(calls, parseCodeInterpreterCall(item))
		}
	}
	return strings.Join(textParts, "\n"), refusal, calls, messageCount, itemTypes
}

func parseCodeInterpreterCall(item rawOutputItem) CodeInterpreterCall {
	call := CodeInterpreterCall{
		ID:          strings.TrimSpace(item.ID),
		ContainerID: strings.TrimSpace(item.ContainerID),
		Status:      strings.TrimSpace(item.Status),
		Code:        strings.TrimSpace(item.Code),
	}
	if call.ID == "" && len(item.CodeInterpreterCall) > 0 {
		var nested rawOutputItem
		if err := json.Unmarshal(item.CodeInterpreterCall, &nested); err == nil {
			if call.ID == "" {
				call.ID = strings.TrimSpace(nested.ID)
			}
			if call.ContainerID == "" {
				call.ContainerID = strings.TrimSpace(nested.ContainerID)
			}
			if call.Status == "" {
				call.Status = strings.TrimSpace(nested.Status)
			}
			if call.Code == "" {
				call.Code = strings.TrimSpace(nested.Code)
			}
		}
	}
	return call
}

// HasCodeInterpreterCall reports whether at least one Code Interpreter call exists.
func HasCodeInterpreterCall(resp *ProviderResponse) bool {
	return resp != nil && len(resp.CodeInterpreterCalls) > 0
}

// CallsMatchContainer reports whether any call references the expected container ID.
func CallsMatchContainer(resp *ProviderResponse, containerID string) bool {
	if resp == nil {
		return false
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return false
	}
	for _, call := range resp.CodeInterpreterCalls {
		if strings.TrimSpace(call.ContainerID) == containerID {
			return true
		}
	}
	return false
}
