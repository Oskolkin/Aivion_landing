package actions

import (
	"context"
	"encoding/json"
	"fmt"
)

// AIFirstRetryInput is sent to the AI-first retry call.
type AIFirstRetryInput struct {
	Input            ActionsAIFirstInput  `json:"input"`
	DraftOutput      ActionsAIFirstOutput `json:"draft_output"`
	ValidationIssues []ValidationIssue    `json:"validation_issues"`
}

// ActionsAIFirstClient is the optional AI surface for the AI-first pipeline.
type ActionsAIFirstClient interface {
	ComposeAIFirstActions(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error)
	ComposeAIFirstCompose(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstComposeOutput, *AIRawResponse, error)
	ComposeObjectDetailBatch(ctx context.Context, batchInput ObjectDetailBatchInput) (*ObjectDetailBatchOutput, *AIRawResponse, error)
	RetryAIFirstActions(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error)
}

func (c *OpenAIActionClient) ComposeAIFirstActions(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	if input.IsV2CandidateInput() && IsTwoPhaseActionsPrompt(actionsAIFirstV2PromptVersion) {
		compose, resp, err := c.ComposeAIFirstCompose(ctx, input)
		if err != nil {
			return nil, resp, err
		}
		out := composeOutputToLegacyOutput(compose)
		out.Actions = make([]AIFirstDraftAction, 0, len(compose.Actions))
		for _, a := range compose.Actions {
			out.Actions = append(out.Actions, baseActionToDraft(AIFirstBaseAction{AIFirstComposeAction: a}))
		}
		return out, resp, nil
	}
	wireInput, err := marshalActionsAIFirstWirePayload(input)
	if err != nil {
		return nil, nil, err
	}
	systemPrompt, userPrompt, promptVersion := actionsAIFirstPromptForInput(input)
	payloadKey := "data_pack_input_json"
	if input.IsV2CandidateInput() {
		payloadKey = "action_candidates_input_json"
	}
	payload := map[string]any{
		"prompt_version": promptVersion,
		payloadKey:       wireInput,
	}
	raw, resp, err := c.callRaw(ctx, systemPrompt, userPrompt, payload, ActionsAIFirstRequestedMaxOutputTokens())
	if err != nil {
		return nil, resp, err
	}
	out, parseErr := parseActionsAIFirstOutput(raw)
	if parseErr != nil {
		return nil, resp, parseErr
	}
	return out, resp, nil
}

func (c *OpenAIActionClient) ComposeAIFirstCompose(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstComposeOutput, *AIRawResponse, error) {
	wireInput, err := marshalActionsAIFirstWirePayload(input)
	if err != nil {
		return nil, nil, err
	}
	systemPrompt, userPrompt, promptVersion := actionsAIFirstPromptForInput(input)
	payload := map[string]any{
		"prompt_version":               promptVersion,
		"action_candidates_input_json": wireInput,
	}
	maxTokens := ActionsComposeRequestedMaxOutputTokens()
	raw, resp, err := c.callRaw(ctx, systemPrompt, userPrompt, payload, maxTokens)
	if err != nil {
		return nil, resp, err
	}
	out, parseErr := parseActionsAIFirstComposeOutput(raw)
	if parseErr != nil {
		return nil, resp, parseErr
	}
	return out, resp, nil
}

func (c *OpenAIActionClient) ComposeObjectDetailBatch(ctx context.Context, batchInput ObjectDetailBatchInput) (*ObjectDetailBatchOutput, *AIRawResponse, error) {
	systemPrompt, userPrompt, promptVersion := actionsObjectDetailPrompt()
	payload := map[string]any{
		"prompt_version":           promptVersion,
		"object_detail_input_json": batchInput,
	}
	raw, resp, err := c.callRawDetail(ctx, systemPrompt, userPrompt, payload, ActionsObjectDetailRequestedMaxOutputTokens())
	if err != nil {
		return nil, resp, err
	}
	out, parseErr := parseObjectDetailBatchOutput(raw)
	if parseErr != nil {
		return nil, resp, parseErr
	}
	return out, resp, nil
}

func marshalActionsAIFirstWirePayload(input ActionsAIFirstInput) (json.RawMessage, error) {
	raw, err := MarshalActionsAIFirstInputWire(input)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func (c *OpenAIActionClient) RetryAIFirstActions(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	if input.Input.IsV2CandidateInput() {
		return nil, nil, fmt.Errorf("%s: retry is not supported for actions v2", ActionsErrorValidationFailed)
	}
	wireInput, err := marshalActionsAIFirstWirePayload(input.Input)
	if err != nil {
		return nil, nil, err
	}
	payload := map[string]any{
		"prompt_version":       actionsAIFirstPromptVersion,
		"data_pack_input_json": wireInput,
		"draft_output":         input.DraftOutput,
		"validation_issues":    input.ValidationIssues,
	}
	raw, resp, err := c.callRaw(ctx, aiFirstRetrySystemPrompt(), aiFirstRetryUserPrompt(), payload, ActionsAIFirstRequestedMaxOutputTokens())
	if err != nil {
		return nil, resp, err
	}
	out, parseErr := parseActionsAIFirstOutput(raw)
	if parseErr != nil {
		return nil, resp, parseErr
	}
	return out, resp, nil
}
