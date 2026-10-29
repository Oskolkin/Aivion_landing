package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aijsonrepair"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// StructuredJSONClient is the minimal surface from recommendations.OpenAIClient.
type StructuredJSONClient interface {
	GenerateStructuredJSON(ctx context.Context, input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error)
}

// OpenAIActionClient adapts recommendations.OpenAIClient to ActionAIClient.
type OpenAIActionClient struct {
	client       StructuredJSONClient
	detailClient StructuredJSONClient
	model        string
}

func NewOpenAIActionClient(client StructuredJSONClient, model string) *OpenAIActionClient {
	return &OpenAIActionClient{client: client, model: model}
}

// NewOpenAIActionClientWithDetail wires separate compose and Object Detail HTTP clients.
func NewOpenAIActionClientWithDetail(composeClient, detailClient StructuredJSONClient, model string) *OpenAIActionClient {
	if detailClient == nil {
		detailClient = composeClient
	}
	return &OpenAIActionClient{client: composeClient, detailClient: detailClient, model: model}
}

func (c *OpenAIActionClient) composeProvider() StructuredJSONClient {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client
}

func (c *OpenAIActionClient) detailProvider() StructuredJSONClient {
	if c == nil {
		return nil
	}
	if c.detailClient != nil {
		return c.detailClient
	}
	return c.client
}

func (c *OpenAIActionClient) ComposeDomainActions(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
	return c.call(ctx, domainComposerSystemPrompt(input.Domain), domainComposerUserPrompt(input.Domain), compactComposerContext(input))
}

func (c *OpenAIActionClient) RepairDomainActions(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error) {
	payload := map[string]any{
		"domain_context":    compactDomainContext(input.DomainContext),
		"draft_output":      input.DraftOutput,
		"validation_issues": input.ValidationIssues,
	}
	return c.call(ctx, domainRepairSystemPrompt(), domainRepairUserPrompt(input.DomainContext.Domain), payload)
}

func (c *OpenAIActionClient) RunLegacyGlobalCheck(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
	raw, resp, err := c.callRaw(ctx, legacyGlobalCheckSystemPrompt(), legacyGlobalCheckUserPrompt(input.MaxTopActions, input.MaxActionsOutput), input)
	if err != nil {
		return nil, resp, err
	}
	var out LegacyGlobalCheckOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, resp, fmt.Errorf("parse legacy global check output: %w", err)
	}
	return &out, resp, nil
}

func (c *OpenAIActionClient) call(ctx context.Context, system, user string, contextPayload any) (*DomainComposerOutput, *AIRawResponse, error) {
	raw, resp, err := c.callRaw(ctx, system, user, contextPayload)
	if err != nil {
		return nil, resp, err
	}
	out, parseErr := parseDomainComposerOutput(raw, "")
	if parseErr != nil {
		return nil, resp, parseErr
	}
	return out, resp, nil
}

func (c *OpenAIActionClient) callRaw(ctx context.Context, system, user string, contextPayload any, maxOutputTokens ...int) (string, *AIRawResponse, error) {
	return c.callRawWith(ctx, c.composeProvider(), system, user, contextPayload, maxOutputTokens...)
}

func (c *OpenAIActionClient) callRawDetail(ctx context.Context, system, user string, contextPayload any, maxOutputTokens ...int) (string, *AIRawResponse, error) {
	return c.callRawWith(ctx, c.detailProvider(), system, user, contextPayload, maxOutputTokens...)
}

func (c *OpenAIActionClient) callRawWith(ctx context.Context, client StructuredJSONClient, system, user string, contextPayload any, maxOutputTokens ...int) (string, *AIRawResponse, error) {
	start := time.Now()
	promptJSON, _ := json.Marshal(contextPayload)
	maxOut := 0
	if len(maxOutputTokens) > 0 {
		maxOut = maxOutputTokens[0]
	}
	if client == nil {
		return "", nil, fmt.Errorf("openai client is not configured")
	}
	out, err := client.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt:    system,
		UserPrompt:      user,
		Context:         contextPayload,
		MaxOutputTokens: maxOut,
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return "", &AIRawResponse{
			Model:                    c.model,
			PromptJSON:               promptJSON,
			LatencyMs:                latency,
			RequestedMaxOutputTokens: maxOut,
			ProviderError:            err.Error(),
		}, err
	}
	raw := out.Content
	repaired, _, repairErr := aijsonrepair.RepairRawJSON(raw)
	if repairErr == nil {
		raw = repaired
	}
	model := out.Model
	if model == "" {
		model = c.model
	}
	truncated := out.ResponseStatus == "incomplete" || out.IncompleteReason == "max_output_tokens"
	if maxOut > 0 && out.OutputTokens >= maxOut && out.IncompleteReason == "" {
		truncated = truncated || out.OutputTokens >= maxOut
	}
	return raw, &AIRawResponse{
		Model:                    model,
		PromptJSON:               promptJSON,
		ResponseJSON:             out.RawResponse,
		RawText:                  raw,
		RequestID:                out.RequestID,
		InputTokens:              out.InputTokens,
		OutputTokens:             out.OutputTokens,
		LatencyMs:                latency,
		ProviderStatus:           out.ResponseStatus,
		FinishReason:             out.FinishReason,
		IncompleteReason:         out.IncompleteReason,
		OutputTruncated:          truncated,
		RequestedMaxOutputTokens: maxOut,
	}, nil
}

func compactComposerContext(input DomainComposerInput) map[string]any {
	return map[string]any{
		"domain":               input.Domain,
		"domain_context":       compactDomainContext(input.DomainContext),
		"account_snapshot":     input.AccountSnapshot,
		"forbidden_claims":     input.ForbiddenClaims,
		"allowed_action_types": input.AllowedTypes,
		"prompt_version":       actionAIPromptVersion,
	}
}

func compactDomainContext(domain ActionDomainContext) map[string]any {
	candidates := make([]map[string]any, 0, len(domain.Candidates))
	for _, c := range domain.Candidates {
		candidates = append(candidates, map[string]any{
			"candidate_id":     c.CandidateID,
			"type":             c.Type,
			"category":         c.Category,
			"priority_hint":    c.PriorityHint,
			"summary":          c.Summary,
			"aggregates":       c.Aggregates,
			"facts":            c.Facts,
			"top_examples":     limitExamples(c.TopExamples, 5),
			"entity_group_ref": c.EntityGroupRef,
			"allowed_ctas":     c.AllowedCTAs,
			"limitations":      c.Limitations,
		})
	}
	return map[string]any{
		"domain":             domain.Domain,
		"status":             domain.Status,
		"candidates":         candidates,
		"data_quality_notes": domain.DataQualityNotes,
	}
}

// FakeActionAIClient is a test double for ActionAIClient.
type FakeActionAIClient struct {
	ComposeFn func(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error)
	RepairFn  func(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error)
	ReviewFn  func(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error)

	ComposeAIFirstFn        func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error)
	ComposeAIFirstComposeFn func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstComposeOutput, *AIRawResponse, error)
	ComposeObjectDetailFn   func(ctx context.Context, batchInput ObjectDetailBatchInput) (*ObjectDetailBatchOutput, *AIRawResponse, error)
	RetryAIFirstFn          func(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error)

	ComposeCalls []DomainComposerInput
	RepairCalls  []DomainRepairInput
	ReviewCalls  []LegacyGlobalCheckInput

	ComposeAIFirstCalls      []ActionsAIFirstInput
	ComposeObjectDetailCalls []ObjectDetailBatchInput
	RetryAIFirstCalls        []AIFirstRetryInput
}

func (f *FakeActionAIClient) ComposeDomainActions(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error) {
	f.ComposeCalls = append(f.ComposeCalls, input)
	if f.ComposeFn != nil {
		return f.ComposeFn(ctx, input)
	}
	return nil, nil, fmt.Errorf("fake compose not configured")
}

func (f *FakeActionAIClient) RepairDomainActions(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error) {
	f.RepairCalls = append(f.RepairCalls, input)
	if f.RepairFn != nil {
		return f.RepairFn(ctx, input)
	}
	return nil, nil, fmt.Errorf("fake repair not configured")
}

func (f *FakeActionAIClient) RunLegacyGlobalCheck(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error) {
	f.ReviewCalls = append(f.ReviewCalls, input)
	if f.ReviewFn != nil {
		return f.ReviewFn(ctx, input)
	}
	return nil, nil, fmt.Errorf("fake review not configured")
}

func (f *FakeActionAIClient) ComposeAIFirstActions(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	f.ComposeAIFirstCalls = append(f.ComposeAIFirstCalls, input)
	if f.ComposeAIFirstFn != nil {
		return f.ComposeAIFirstFn(ctx, input)
	}
	if input.IsV2CandidateInput() && IsTwoPhaseActionsPrompt(actionsAIFirstV2PromptVersion) {
		compose, resp, err := f.ComposeAIFirstCompose(ctx, input)
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
	return nil, nil, fmt.Errorf("fake ai-first compose not configured")
}

func (f *FakeActionAIClient) ComposeAIFirstCompose(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstComposeOutput, *AIRawResponse, error) {
	f.ComposeAIFirstCalls = append(f.ComposeAIFirstCalls, input)
	if f.ComposeAIFirstComposeFn != nil {
		return f.ComposeAIFirstComposeFn(ctx, input)
	}
	if f.ComposeAIFirstFn != nil {
		out, resp, err := f.ComposeAIFirstFn(ctx, input)
		if err != nil {
			return nil, resp, err
		}
		compose := &ActionsAIFirstComposeOutput{
			SchemaVersion: out.SchemaVersion,
			Focus:         out.Focus,
			Notes:         out.Notes,
		}
		for _, a := range out.Actions {
			compose.Actions = append(compose.Actions, composeActionFromDraft(a))
		}
		return compose, resp, nil
	}
	return nil, nil, fmt.Errorf("fake ai-first compose not configured")
}

func (f *FakeActionAIClient) ComposeObjectDetailBatch(ctx context.Context, batchInput ObjectDetailBatchInput) (*ObjectDetailBatchOutput, *AIRawResponse, error) {
	f.ComposeObjectDetailCalls = append(f.ComposeObjectDetailCalls, batchInput)
	if f.ComposeObjectDetailFn != nil {
		return f.ComposeObjectDetailFn(ctx, batchInput)
	}
	return buildDeterministicObjectDetailOutput(batchInput), &AIRawResponse{
		Model: "fake-object-detail", InputTokens: 64, OutputTokens: 32,
	}, nil
}

func (f *FakeActionAIClient) RetryAIFirstActions(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
	f.RetryAIFirstCalls = append(f.RetryAIFirstCalls, input)
	if f.RetryAIFirstFn != nil {
		return f.RetryAIFirstFn(ctx, input)
	}
	return nil, nil, fmt.Errorf("fake ai-first retry not configured")
}

func parseDomainComposerOutput(raw string, expectedDomain string) (*DomainComposerOutput, error) {
	repaired, _, err := aijsonrepair.RepairRawJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	var out DomainComposerOutput
	if err := json.Unmarshal([]byte(repaired), &out); err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	if expectedDomain != "" && out.Domain != "" && out.Domain != expectedDomain {
		return &out, fmt.Errorf("domain mismatch")
	}
	return &out, nil
}

func compactFullCandidateContext(ctx *ActionCandidateContext) map[string]any {
	domains := make([]map[string]any, 0, len(ctx.Domains))
	for _, d := range ctx.Domains {
		domains = append(domains, compactDomainContext(d))
	}
	return map[string]any{
		"seller_account_id":    ctx.SellerAccountID,
		"metric_date":          ctx.MetricDate,
		"account_snapshot":     ctx.AccountSnapshot,
		"domains":              domains,
		"allowed_action_types": ctx.AllowedActionTypes,
		"forbidden_claims":     ctx.ForbiddenClaims,
		"data_quality_notes":   ctx.DataQualityNotes,
		"coverage_report":      ctx.CoverageReport,
	}
}

func hashCandidateContext(ctx *ActionCandidateContext) string {
	compact := compactFullCandidateContext(ctx)
	data, _ := json.Marshal(compact)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16])
}
