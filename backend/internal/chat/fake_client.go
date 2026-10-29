package chat

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const fakeChatModel = "fake-chat-deterministic"

var (
	controlCodeRe = regexp.MustCompile(`(?i)\b([A-Z]{2,12}_CONTROL_\d+)\b`)
	e2eMarkerRe   = regexp.MustCompile(`\b(E2E-[A-Z0-9]+-\d+)\b`)
)

// DeterministicFakeAIClient answers without calling OpenAI.
// Used when AI_PROVIDER_MODE=fake for local/CI runtime smoke.
type DeterministicFakeAIClient struct{}

func NewDeterministicFakeAIClient() *DeterministicFakeAIClient {
	return &DeterministicFakeAIClient{}
}

func (c *DeterministicFakeAIClient) PlanTools(ctx context.Context, input PlanToolsInput) (*PlanToolsOutput, error) {
	_ = ctx
	plan := ToolPlan{
		Intent:     ChatIntentGeneralOverview,
		Confidence: 0.95,
		Language:   "ru",
		Assumptions: []string{
			"deterministic fake chat provider; OpenAI was not called",
		},
	}
	raw, _ := json.Marshal(map[string]any{
		"provider": "fake-chat-deterministic",
		"plan":     plan,
	})
	return &PlanToolsOutput{
		Plan:          plan,
		Content:       `{"intent":"general_overview","confidence":0.95,"language":"ru"}`,
		RawResponse:   raw,
		Model:         fakeChatModel,
		RequestID:     "fake-chat-plan",
		InputTokens:   32,
		OutputTokens:  16,
		TotalTokens:   48,
		FinishedAtUTC: time.Now().UTC(),
	}, nil
}

func (c *DeterministicFakeAIClient) GenerateAnswer(ctx context.Context, input GenerateAnswerInput) (*GenerateAnswerOutput, error) {
	_ = ctx
	text := buildDeterministicFakeAnswer(input)
	answer := synthesizeStreamAnswer(text, input.FactContext)
	raw, _ := json.Marshal(map[string]any{
		"provider": "fake-chat-deterministic",
		"answer":   text,
	})
	return &GenerateAnswerOutput{
		Answer:        answer,
		Content:       text,
		RawResponse:   raw,
		Model:         fakeChatModel,
		RequestID:     "fake-chat-answer",
		InputTokens:   64,
		OutputTokens:  int32(len([]rune(text)) / 4),
		TotalTokens:   64 + int32(len([]rune(text))/4),
		FinishedAtUTC: time.Now().UTC(),
	}, nil
}

func (c *DeterministicFakeAIClient) StreamAnswer(
	ctx context.Context,
	input GenerateAnswerInput,
	onDelta func(delta string) error,
) (*GenerateAnswerOutput, error) {
	out, err := c.GenerateAnswer(ctx, input)
	if err != nil {
		return nil, err
	}
	if onDelta != nil {
		runes := []rune(out.Content)
		chunk := 24
		for i := 0; i < len(runes); i += chunk {
			end := i + chunk
			if end > len(runes) {
				end = len(runes)
			}
			if err := onDelta(string(runes[i:end])); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// UploadUserFile satisfies fileUploadClient without calling OpenAI.
// Real uploads are skipped via shouldUploadAttachmentToProvider; this exists so
// attachInputFilesForAsk can type-assert the fake client.
func (c *DeterministicFakeAIClient) UploadUserFile(ctx context.Context, fileName string, data []byte, mimeType string) (string, error) {
	_ = ctx
	_ = data
	_ = mimeType
	return "fake-file-" + strings.TrimSpace(fileName), nil
}

// DeleteUserFile is a no-op companion for UploadUserFile.
func (c *DeterministicFakeAIClient) DeleteUserFile(ctx context.Context, fileID string) error {
	_ = ctx
	_ = fileID
	return nil
}

func buildDeterministicFakeAnswer(input GenerateAnswerInput) string {
	combined := strings.Join([]string{input.UserPrompt, input.SystemPrompt}, "\n")
	if input.FactContext != nil {
		for _, item := range input.FactContext.Facts.Other {
			for _, v := range item {
				if s, ok := v.(string); ok {
					combined += "\n" + s
				}
			}
		}
	}

	parts := make([]string, 0, 4)
	if codes := uniqueMatches(controlCodeRe, combined); len(codes) > 0 {
		parts = append(parts, "Контрольный код из вложения: "+strings.Join(codes, ", ")+".")
	}
	if markers := uniqueMatches(e2eMarkerRe, combined); len(markers) > 0 {
		parts = append(parts, "Маркер запроса: "+strings.Join(markers, ", ")+".")
	}
	if len(parts) == 0 {
		parts = append(parts,
			"Детерминированный тестовый ответ Aivion (fake chat provider).",
			"OpenAI не вызывался.",
		)
		if input.FactContext != nil && factContextHasFacts(input.FactContext) {
			parts = append(parts, "Ответ сформирован по доступному фактическому контексту кабинета.")
		} else {
			parts = append(parts, "Фактический контекст кабинета ограничен; ответ носит общий характер.")
		}
	} else {
		parts = append(parts, "Ответ сформирован детерминированным fake chat provider без вызова OpenAI.")
	}
	return strings.Join(parts, " ")
}

func uniqueMatches(re *regexp.Regexp, text string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, m := range re.FindAllString(text, -1) {
		key := strings.ToUpper(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
	}
	return out
}
