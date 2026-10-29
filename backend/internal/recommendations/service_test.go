package recommendations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
)

type mockServiceRepo struct {
	runID       int64
	createErr   error
	completeErr error
	failErr     error
	upsertErr   error
	linkErr     error
	created     bool
	completed   bool
	failed      bool
	upserts     int
	deletes     int
	links       int
	lastRunType string
	lastRaw     json.RawMessage
	lastDiag    *CreateRunDiagnosticInput
	feedback    *RecommendationFeedback
	feedbackErr error
}

func (m *mockServiceRepo) CreateRun(ctx context.Context, input CreateRecommendationRunInput) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	m.created = true
	m.lastRunType = input.RunType
	if m.runID == 0 {
		m.runID = 11
	}
	return m.runID, nil
}
func (m *mockServiceRepo) CompleteRun(ctx context.Context, input CompleteRecommendationRunInput) error {
	if m.completeErr != nil {
		return m.completeErr
	}
	m.completed = true
	return nil
}
func (m *mockServiceRepo) FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.failed = true
	return nil
}
func (m *mockServiceRepo) UpsertRecommendation(ctx context.Context, input UpsertRecommendationInput) (int64, error) {
	if m.upsertErr != nil {
		return 0, m.upsertErr
	}
	m.upserts++
	m.lastRaw = input.RawAIResponse
	return int64(100 + m.upserts), nil
}
func (m *mockServiceRepo) DeleteRecommendationAlertLinks(ctx context.Context, sellerAccountID int64, recommendationID int64) error {
	m.deletes++
	return nil
}
func (m *mockServiceRepo) LinkRecommendationAlert(ctx context.Context, sellerAccountID int64, recommendationID int64, alertID int64) error {
	if m.linkErr != nil {
		return m.linkErr
	}
	m.links++
	return nil
}
func (m *mockServiceRepo) ListRecommendationsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Recommendation, error) {
	return nil, nil
}
func (m *mockServiceRepo) GetRecommendationByID(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return Recommendation{ID: recommendationID, Title: "rec"}, nil
}
func (m *mockServiceRepo) ListAlertsByRecommendationID(ctx context.Context, sellerAccountID int64, recommendationID int64) ([]RelatedAlert, error) {
	return []RelatedAlert{{ID: 1, AlertType: "stock_oos_risk", EvidencePayload: map[string]any{"k": "v"}}}, nil
}
func (m *mockServiceRepo) AcceptRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return Recommendation{}, nil
}
func (m *mockServiceRepo) DismissRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return Recommendation{}, nil
}
func (m *mockServiceRepo) ResolveRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return Recommendation{}, nil
}
func (m *mockServiceRepo) CountOpenRecommendations(ctx context.Context, sellerAccountID int64) (int64, error) {
	return 0, nil
}
func (m *mockServiceRepo) CountOpenRecommendationsByPriority(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return nil, nil
}
func (m *mockServiceRepo) CountOpenRecommendationsByConfidence(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	return nil, nil
}
func (m *mockServiceRepo) GetLatestRecommendationRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error) {
	return nil, nil
}
func (m *mockServiceRepo) CreateRunDiagnostic(ctx context.Context, input CreateRunDiagnosticInput) error {
	in := input
	m.lastDiag = &in
	return nil
}

func (m *mockServiceRepo) CreateFeedback(ctx context.Context, input AddRecommendationFeedbackInput) (*RecommendationFeedback, error) {
	if m.feedbackErr != nil {
		return nil, m.feedbackErr
	}
	if m.feedback != nil {
		return m.feedback, nil
	}
	return &RecommendationFeedback{
		ID:               1,
		RecommendationID: input.RecommendationID,
		SellerAccountID:  input.SellerAccountID,
		Rating:           input.Rating,
		Comment:          input.Comment,
		CreatedAt:        time.Now().UTC(),
	}, nil
}

type mockBuilder struct {
	ctx *AIRecommendationContext
	err error
}

func (m mockBuilder) BuildForAccount(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (*AIRecommendationContext, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ctx, nil
}

type mockAIClient struct {
	out *GenerateRecommendationsOutput
	err error
}

func (m mockAIClient) GenerateRecommendations(ctx context.Context, input GenerateRecommendationsInput) (*GenerateRecommendationsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.out, nil
}

type mockValidator struct {
	res *ValidationResult
	err error
}

func (m mockValidator) Validate(output *GenerateRecommendationsOutput, ctx *AIRecommendationContext) (*ValidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

func TestServiceGenerateForAccount_StockReplenishmentAliasSavedAsCanonical(t *testing.T) {
	repo := &mockServiceRepo{}
	ctx := sampleContext()
	ctx.Alerts.OpenTotal = int64(len(ctx.Alerts.TopOpen))
	builder := mockBuilder{ctx: ctx}
	content := `{"recommendations":[{"recommendation_type":"stock_replenishment","horizon":"short_term","entity_type":"sku","entity_sku":1001,"title":"Пополнить","what_happened":"w","why_it_matters":"y","recommended_action":"a","priority_score":80,"priority_level":"high","urgency":"high","confidence_level":"high","supporting_metrics":{"stock_available":0},"constraints_checked":{"stock_checked":true},"supporting_alert_ids":[101],"related_alert_types":["stock_oos_risk"]}]}`
	client := mockAIClient{out: &GenerateRecommendationsOutput{
		Model: "gpt-test", Content: content, RawResponse: json.RawMessage(`{}`), InputTokens: 1, OutputTokens: 1,
	}}
	svc := NewService(repo, builder, client, NewOutputValidator(), ServiceConfig{Model: "gpt-test", PromptVersion: "v1"}, nil)
	sum, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateForAccount: %v", err)
	}
	if sum.UpsertedTotal != 1 {
		t.Fatalf("expected 1 upsert, got %d (valid=%d rejected=%d)", sum.UpsertedTotal, sum.ValidTotal, sum.RejectedTotal)
	}
	if repo.upserts != 1 {
		t.Fatalf("expected 1 repo upsert")
	}
}

func TestServiceGenerateForAccount_Success(t *testing.T) {
	repo := &mockServiceRepo{}
	sku := int64(1001)
	builder := mockBuilder{ctx: &AIRecommendationContext{
		Alerts: AlertsContext{
			OpenTotal: 1,
			TopOpen:   []AlertSignal{{ID: 10, Severity: "high", AlertType: "stock_oos_risk", EntitySKU: &sku}},
		},
	}}
	raw := json.RawMessage(`{"id":"resp"}`)
	client := mockAIClient{out: &GenerateRecommendationsOutput{
		Model: "gpt-test", Content: `{"recommendations":[]}`, RawResponse: raw, InputTokens: 12, OutputTokens: 8, TotalTokens: 20,
	}}
	validator := mockValidator{res: &ValidationResult{
		TotalRecommendations: 2,
		ValidRecommendations: []ValidatedRecommendation{
			{
				Recommendation: AIRecommendationCandidate{
					RecommendationType: "replenish_sku",
					Horizon:            "short_term",
					EntityType:         "sku",
					Title:              "restock",
					WhatHappened:       "w",
					WhyItMatters:       "y",
					RecommendedAction:  "a",
					PriorityScore:      50,
					PriorityLevel:      "high",
					Urgency:            "high",
					ConfidenceLevel:    "high",
					SupportingMetrics:  map[string]any{"x": 1},
					Constraints:        map[string]any{"stock_checked": true},
					SupportingAlertIDs: []int64{10},
				},
				Warnings:             []string{"warn"},
				FinalConfidenceLevel: "medium",
			},
		},
		RejectedRecommendations: []RejectedRecommendation{{Index: 1, Reason: "bad"}},
	}}
	svc := NewService(repo, builder, client, validator, ServiceConfig{
		Model: "gpt-5.4", PromptVersion: "v1",
	}, nil)
	sum, err := svc.GenerateForAccount(context.Background(), 77, time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GenerateForAccount returned error: %v", err)
	}
	if !repo.created || !repo.completed || repo.failed {
		t.Fatalf("unexpected run lifecycle: %+v", repo)
	}
	if repo.upserts != 1 || repo.links != 1 {
		t.Fatalf("expected 1 upsert and 1 link, got upserts=%d links=%d", repo.upserts, repo.links)
	}
	if repo.deletes != 1 {
		t.Fatalf("expected 1 delete old links call, got %d", repo.deletes)
	}
	if sum.ValidTotal != 1 || sum.RejectedTotal != 1 || sum.WarningsTotal != 2 || sum.TotalTokens != 20 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}

func TestServiceGenerateForAccount_FailsRunOnBuildError(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(
		repo,
		mockBuilder{err: errors.New("ctx failed")},
		mockAIClient{out: &GenerateRecommendationsOutput{}},
		mockValidator{res: &ValidationResult{}},
		ServiceConfig{},
		nil,
	)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !repo.failed {
		t.Fatalf("expected run to be marked failed")
	}
}

type stubDailyUsage struct {
	recErr error
}

func (s stubDailyUsage) CheckChatQuestion(context.Context, int64) error      { return nil }
func (s stubDailyUsage) CheckRecommendationRun(context.Context, int64) error { return s.recErr }
func (s stubDailyUsage) CheckSummaryRun(context.Context, int64) error        { return nil }

func TestServiceGenerateForAccount_BlockedByDailyLimit(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(
		repo,
		mockBuilder{err: errors.New("context must not build")},
		mockAIClient{err: errors.New("openai must not run")},
		mockValidator{res: &ValidationResult{}},
		ServiceConfig{},
		stubDailyUsage{recErr: aicost.ErrDailyLimitExceeded},
	)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, aicost.ErrDailyLimitExceeded) && !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.created {
		t.Fatal("run must not be created when daily limit exceeded")
	}
}

func TestServiceGenerateForAccountWithType_UsesRunType(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{}}, mockAIClient{out: &GenerateRecommendationsOutput{}}, mockValidator{res: &ValidationResult{}}, ServiceConfig{RunType: "scheduled"}, nil)
	_, _ = svc.GenerateForAccountWithType(context.Background(), 1, time.Now().UTC(), "post_alerts")
	if repo.lastRunType != "post_alerts" {
		t.Fatalf("expected run type post_alerts, got %s", repo.lastRunType)
	}
}

func TestServiceGenerateForAccount_FailRunOnOpenAIError(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{}}, mockAIClient{err: errors.New("openai down")}, mockValidator{res: &ValidationResult{}}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil || !repo.failed {
		t.Fatalf("expected fail run on openai error")
	}
}

func TestServiceGenerateForAccount_FailRunOnValidatorError(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{}}, mockAIClient{out: &GenerateRecommendationsOutput{}}, mockValidator{err: errors.New("bad json")}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil || !repo.failed {
		t.Fatalf("expected fail run on validator error")
	}
}

func TestServiceGenerateForAccount_AllRejected(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 3}}}, mockAIClient{out: &GenerateRecommendationsOutput{Content: `{"recommendations":[{}]}`}}, mockValidator{res: &ValidationResult{
		TotalRecommendations:    2,
		RejectedRecommendations: []RejectedRecommendation{{Index: 0, Reason: "x"}, {Index: 1, Reason: "y"}},
	}}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error when all candidates rejected")
	}
	if !repo.failed || repo.completed {
		t.Fatalf("expected failed run, got failed=%v completed=%v", repo.failed, repo.completed)
	}
	if !strings.Contains(err.Error(), errMsgAllRejectedByValidator) {
		t.Fatalf("expected validator rejection message, got: %v", err)
	}
}

func TestServiceGenerateForAccount_SavesDiagnosticsOnFailure(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 2}}}, mockAIClient{out: &GenerateRecommendationsOutput{
		Content:     `{"recommendations":[]}`,
		RawResponse: json.RawMessage(`{"parsed":{"recommendations":[]}}`),
	}}, mockValidator{res: &ValidationResult{TotalRecommendations: 0}}, ServiceConfig{}, nil)
	_, _ = svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if repo.lastDiag == nil || len(repo.lastDiag.RawOpenAIResponse) == 0 {
		t.Fatalf("expected diagnostic with raw_openai_response, got %+v", repo.lastDiag)
	}
}

func TestServiceGenerateForAccount_EmptyAIWithOpenAlerts(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{Alerts: AlertsContext{OpenTotal: 35}}}, mockAIClient{out: &GenerateRecommendationsOutput{
		Content:     `{"recommendations":[]}`,
		RawResponse: json.RawMessage(`{"recommendations":[]}`),
	}}, mockValidator{res: &ValidationResult{TotalRecommendations: 0}}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error for empty AI output with open alerts")
	}
	if !repo.failed || strings.Contains(err.Error(), errMsgAIEmptyWithAlerts) == false {
		t.Fatalf("expected failed run with empty-ai message, err=%v failed=%v", err, repo.failed)
	}
}

func TestServiceGenerateForAccount_PartialValidationCompletesWithWarning(t *testing.T) {
	repo := &mockServiceRepo{}
	sku := int64(1001)
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{
		Alerts: AlertsContext{
			OpenTotal: 2,
			TopOpen:   []AlertSignal{{ID: 10, Severity: "high", AlertType: "stock_oos_risk", EntitySKU: &sku}},
		},
	}}, mockAIClient{out: &GenerateRecommendationsOutput{Content: `{"recommendations":[{},{}]}`}}, mockValidator{res: &ValidationResult{
		TotalRecommendations: 2,
		ValidRecommendations: []ValidatedRecommendation{{
			Recommendation: AIRecommendationCandidate{
				RecommendationType: "replenish_sku",
				Horizon:            "short_term",
				EntityType:         "sku",
				EntitySKU:          &sku,
				Title:              "restock",
				WhatHappened:       "w",
				WhyItMatters:       "y",
				RecommendedAction:  "a",
				PriorityScore:      50,
				PriorityLevel:      "high",
				Urgency:            "high",
				ConfidenceLevel:    "high",
				SupportingMetrics:  map[string]any{"stock_available": 0},
				Constraints:        map[string]any{"stock_checked": true},
				SupportingAlertIDs: []int64{10},
			},
			FinalConfidenceLevel: "high",
		}},
		RejectedRecommendations: []RejectedRecommendation{{Index: 1, Reason: "bad"}},
	}}, ServiceConfig{}, nil)
	sum, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.completed || repo.upserts != 1 || sum.WarningsTotal < 1 {
		t.Fatalf("expected completed run with warning, repo=%+v sum=%+v", repo, sum)
	}
}

func TestServiceGenerateForAccount_FailRunOnDBSaveError(t *testing.T) {
	repo := &mockServiceRepo{upsertErr: errors.New("db down")}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{}}, mockAIClient{out: &GenerateRecommendationsOutput{}}, mockValidator{res: &ValidationResult{
		TotalRecommendations: 1,
		ValidRecommendations: []ValidatedRecommendation{{Recommendation: AIRecommendationCandidate{
			RecommendationType: "replenish_sku",
			Horizon:            "short_term",
			EntityType:         "sku",
			Title:              "t",
			WhatHappened:       "w",
			WhyItMatters:       "y",
			RecommendedAction:  "a",
			PriorityScore:      10,
			PriorityLevel:      "low",
			Urgency:            "low",
			ConfidenceLevel:    "low",
			SupportingMetrics:  map[string]any{"x": 1},
			Constraints:        map[string]any{"stock_checked": true},
		}}},
	}}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err == nil || !repo.failed {
		t.Fatalf("expected fail run on db save error")
	}
}

func TestServiceGenerateForAccount_SanitizesRawAIResponse(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{ctx: &AIRecommendationContext{}}, mockAIClient{out: &GenerateRecommendationsOutput{
		RawResponse: json.RawMessage(`{"authorization":"Bearer sk-secret-123","x":"sk-abc"}`),
	}}, mockValidator{res: &ValidationResult{
		ValidRecommendations: []ValidatedRecommendation{{Recommendation: AIRecommendationCandidate{
			RecommendationType: "replenish_sku",
			Horizon:            "short_term",
			EntityType:         "sku",
			Title:              "t",
			WhatHappened:       "w",
			WhyItMatters:       "y",
			RecommendedAction:  "a",
			PriorityScore:      10,
			PriorityLevel:      "low",
			Urgency:            "low",
			ConfidenceLevel:    "low",
			SupportingMetrics:  map[string]any{"x": 1},
			Constraints:        map[string]any{"stock_checked": true},
		}}},
	}}, ServiceConfig{}, nil)
	_, err := svc.GenerateForAccount(context.Background(), 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw := string(repo.lastRaw)
	if strings.Contains(raw, "sk-secret-123") || strings.Contains(raw, "sk-abc") {
		t.Fatalf("raw response contains sensitive key material: %s", raw)
	}
}

func TestServiceGetRecommendationDetailByID_IncludesRelatedAlerts(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{}, mockAIClient{}, mockValidator{}, ServiceConfig{}, nil)
	detail, err := svc.GetRecommendationDetailByID(context.Background(), 77, 55)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Recommendation.ID != 55 {
		t.Fatalf("unexpected recommendation id: %d", detail.Recommendation.ID)
	}
	if len(detail.RelatedAlerts) != 1 || detail.RelatedAlerts[0].ID != 1 {
		t.Fatalf("expected related alerts in detail response")
	}
}

func TestServiceAddFeedback(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{}, mockAIClient{}, mockValidator{}, ServiceConfig{}, nil)
	comment := " useful "
	item, err := svc.AddFeedback(context.Background(), AddRecommendationFeedbackInput{
		SellerAccountID:  1,
		RecommendationID: 2,
		Rating:           RecommendationFeedbackPositive,
		Comment:          &comment,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Rating != RecommendationFeedbackPositive {
		t.Fatalf("unexpected rating: %s", item.Rating)
	}
	if item.Comment == nil || *item.Comment != "useful" {
		t.Fatalf("unexpected comment: %+v", item.Comment)
	}
}

func TestServiceAddFeedbackValidation(t *testing.T) {
	repo := &mockServiceRepo{}
	svc := NewService(repo, mockBuilder{}, mockAIClient{}, mockValidator{}, ServiceConfig{}, nil)
	if _, err := svc.AddFeedback(context.Background(), AddRecommendationFeedbackInput{
		SellerAccountID:  1,
		RecommendationID: 2,
		Rating:           "bad",
	}); err == nil {
		t.Fatalf("expected validation error")
	}
	repo.feedbackErr = errors.New("x")
	if _, err := svc.AddFeedback(context.Background(), AddRecommendationFeedbackInput{
		SellerAccountID:  1,
		RecommendationID: 2,
		Rating:           RecommendationFeedbackPositive,
	}); err == nil {
		t.Fatalf("expected repo error")
	}
}
