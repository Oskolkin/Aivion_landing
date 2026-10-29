package recommendations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type serviceContextBuilder interface {
	BuildForAccount(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (*AIRecommendationContext, error)
}

type serviceValidator interface {
	Validate(output *GenerateRecommendationsOutput, ctx *AIRecommendationContext) (*ValidationResult, error)
}

type serviceRepository interface {
	CreateRun(ctx context.Context, input CreateRecommendationRunInput) (int64, error)
	CompleteRun(ctx context.Context, input CompleteRecommendationRunInput) error
	FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) error
	UpsertRecommendation(ctx context.Context, input UpsertRecommendationInput) (int64, error)
	DeleteRecommendationAlertLinks(ctx context.Context, sellerAccountID int64, recommendationID int64) error
	LinkRecommendationAlert(ctx context.Context, sellerAccountID int64, recommendationID int64, alertID int64) error
	ListRecommendationsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Recommendation, error)
	GetRecommendationByID(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error)
	ListAlertsByRecommendationID(ctx context.Context, sellerAccountID int64, recommendationID int64) ([]RelatedAlert, error)
	AcceptRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error)
	DismissRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error)
	ResolveRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error)
	CountOpenRecommendations(ctx context.Context, sellerAccountID int64) (int64, error)
	CountOpenRecommendationsByPriority(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	CountOpenRecommendationsByConfidence(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	GetLatestRecommendationRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error)
	CreateFeedback(ctx context.Context, input AddRecommendationFeedbackInput) (*RecommendationFeedback, error)
	CreateRunDiagnostic(ctx context.Context, input CreateRunDiagnosticInput) error
}

type Service struct {
	repo           serviceRepository
	contextBuilder serviceContextBuilder
	aiClient       AIClient
	validator      serviceValidator
	cfg            ServiceConfig
	dailyUsage     aicost.DailyUsageChecker
}

type ServiceConfig struct {
	RunType       string
	Source        string
	Model         string
	PromptVersion string
	SystemPrompt  string
	UserPrompt    string
}

var (
	openAIKeyRegex       = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)
	bearerTokenJSONRegex = regexp.MustCompile(`(?i)"authorization"\s*:\s*"Bearer [^"]+"`)
	ErrNotFound          = errors.New("recommendation not found")
)

type GenerateForAccountSummary struct {
	SellerAccountID   int64     `json:"seller_account_id"`
	AsOfDate          time.Time `json:"as_of_date"`
	RunID             int64     `json:"run_id"`
	GeneratedTotal    int       `json:"generated_total"`
	ValidTotal        int       `json:"valid_total"`
	RejectedTotal     int       `json:"rejected_total"`
	UpsertedTotal     int       `json:"upserted_total"`
	LinkedAlertsTotal int       `json:"linked_alerts_total"`
	WarningsTotal     int       `json:"warnings_total"`
	InputTokens       int       `json:"input_tokens"`
	OutputTokens      int       `json:"output_tokens"`
	TotalTokens       int       `json:"total_tokens"`
}

type ListFilter struct {
	Status             *string
	RecommendationType *string
	PriorityLevel      *string
	ConfidenceLevel    *string
	Horizon            *string
	EntityType         *string
	Limit              int
	Offset             int
}

type Recommendation struct {
	ID                 int64
	Source             string
	RecommendationType string
	Horizon            string
	EntityType         string
	EntityID           *string
	EntitySKU          *int64
	EntityOfferID      *string
	Title              string
	WhatHappened       string
	WhyItMatters       string
	RecommendedAction  string
	ExpectedEffect     *string
	PriorityScore      float64
	PriorityLevel      string
	Urgency            string
	ConfidenceLevel    string
	Status             string
	SupportingMetrics  map[string]any
	Constraints        map[string]any
	AIModel            *string
	AIPromptVersion    *string
	RawAIResponse      map[string]any
	FirstSeenAt        time.Time
	LastSeenAt         time.Time
	AcceptedAt         *time.Time
	DismissedAt        *time.Time
	ResolvedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Summary struct {
	OpenTotal    int64
	ByPriority   []NamedCount
	ByConfidence []NamedCount
	LatestRun    *RunInfo
}

type RelatedAlert struct {
	ID              int64
	AlertType       string
	AlertGroup      string
	EntityType      string
	EntityID        *string
	EntitySKU       *int64
	EntityOfferID   *string
	Title           string
	Message         string
	Severity        string
	Urgency         string
	Status          string
	EvidencePayload map[string]any
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
}

type RecommendationDetail struct {
	Recommendation Recommendation
	RelatedAlerts  []RelatedAlert
}

type CreateRecommendationRunInput struct {
	SellerAccountID int64
	RunType         string
	AsOfDate        time.Time
	AIModel         string
	AIPromptVersion string
}

type CompleteRecommendationRunInput struct {
	RunID                         int64
	SellerAccountID               int64
	InputTokens                   int
	OutputTokens                  int
	EstimatedCost                 float64
	GeneratedRecommendationsCount int
	AcceptedRecommendationsCount  int
}

type UpsertRecommendationInput struct {
	SellerAccountID    int64
	Source             string
	RecommendationType string
	Horizon            string
	EntityType         string
	EntityID           *string
	EntitySKU          *int64
	EntityOfferID      *string
	Title              string
	WhatHappened       string
	WhyItMatters       string
	RecommendedAction  string
	ExpectedEffect     *string
	PriorityScore      float64
	PriorityLevel      string
	Urgency            string
	ConfidenceLevel    string
	SupportingMetrics  map[string]any
	Constraints        map[string]any
	AIModel            string
	AIPromptVersion    string
	RawAIResponse      json.RawMessage
	Fingerprint        string
}

func NewService(repo serviceRepository, builder serviceContextBuilder, aiClient AIClient, validator serviceValidator, cfg ServiceConfig, dailyUsage aicost.DailyUsageChecker) *Service {
	if cfg.RunType == "" {
		cfg.RunType = "manual"
	}
	if cfg.Source == "" {
		cfg.Source = "chatgpt"
	}
	if cfg.PromptVersion == "" {
		cfg.PromptVersion = "stage8.prompt.v1"
	}
	if strings.TrimSpace(cfg.SystemPrompt) == "" {
		cfg.SystemPrompt = DefaultSystemPrompt()
	}
	if strings.TrimSpace(cfg.UserPrompt) == "" {
		cfg.UserPrompt = DefaultUserPrompt()
	}
	return &Service{
		repo:           repo,
		contextBuilder: builder,
		aiClient:       aiClient,
		validator:      validator,
		cfg:            cfg,
		dailyUsage:     dailyUsage,
	}
}

func (s *Service) GenerateForAccount(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (*GenerateForAccountSummary, error) {
	return s.GenerateForAccountWithType(ctx, sellerAccountID, asOfDate, s.cfg.RunType)
}

func (s *Service) GenerateForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType string) (*GenerateForAccountSummary, error) {
	asOf := asOfDate.UTC().Truncate(24 * time.Hour)
	if strings.TrimSpace(runType) == "" {
		runType = s.cfg.RunType
	}
	if s.dailyUsage != nil {
		if err := s.dailyUsage.CheckRecommendationRun(ctx, sellerAccountID); err != nil {
			return nil, fmt.Errorf("recommendation generation blocked: %w", err)
		}
	}
	runID, err := s.repo.CreateRun(ctx, CreateRecommendationRunInput{
		SellerAccountID: sellerAccountID,
		RunType:         runType,
		AsOfDate:        asOf,
		AIModel:         s.cfg.Model,
		AIPromptVersion: s.cfg.PromptVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("create recommendation run: %w", err)
	}

	persistDiagnostic := func(
		contextPayload *AIRecommendationContext,
		aiOutput *GenerateRecommendationsOutput,
		validation *ValidationResult,
		outcome runOutcome,
		savedCount int,
		estCost float64,
	) error {
		diag := buildRunDiagnosticInput(runID, sellerAccountID, contextPayload, aiOutput, validation, outcome, savedCount, estCost)
		return persistRunDiagnostic(s.repo, ctx, diag, s.cfg.PromptVersion)
	}

	failAndWrap := func(sourceErr error, stage string, contextPayload *AIRecommendationContext, aiOutput *GenerateRecommendationsOutput, validation *ValidationResult, outcome runOutcome, savedCount int, estCost float64) error {
		outcome.FailRun = true
		if outcome.ErrorMessage == "" {
			outcome.ErrorMessage = sourceErr.Error()
		}
		if outcome.ErrorStage == "" {
			outcome.ErrorStage = stage
		}
		if diagErr := persistDiagnostic(contextPayload, aiOutput, validation, outcome, savedCount, estCost); diagErr != nil {
			return fmt.Errorf("%s: %w (diagnostics: %v)", stage, sourceErr, diagErr)
		}
		if failErr := s.repo.FailRun(ctx, sellerAccountID, runID, outcome.ErrorMessage); failErr != nil {
			return fmt.Errorf("%s: %w (also failed to mark run as failed: %v)", stage, sourceErr, failErr)
		}
		return fmt.Errorf("%s: %w", stage, sourceErr)
	}

	contextPayload, err := s.contextBuilder.BuildForAccount(ctx, sellerAccountID, asOf)
	if err != nil {
		out := runOutcome{ErrorStage: errStageContext, ErrorMessage: err.Error(), FailRun: true}
		return nil, failAndWrap(err, "build recommendation context", nil, nil, nil, out, 0, 0)
	}

	aiOutput, err := s.aiClient.GenerateRecommendations(ctx, GenerateRecommendationsInput{
		SystemPrompt: s.cfg.SystemPrompt,
		UserPrompt:   s.cfg.UserPrompt,
		Context:      contextPayload,
	})
	if err != nil {
		wrapped := err
		stage := "openai"
		switch {
		case openaix.IsTemporarilyUnavailable(err):
			wrapped = fmt.Errorf("[error_code=openai_unavailable] %w", err)
		case errors.Is(err, ErrOpenAIRequestTooLarge):
			wrapped = fmt.Errorf("[error_code=context_budget_exceeded] %w", err)
			stage = "context_budget"
		}
		return nil, failAndWrap(wrapped, "generate recommendations with ai client", contextPayload, nil, nil, runOutcome{ErrorStage: stage, ErrorMessage: wrapped.Error(), FailRun: true}, 0, 0)
	}

	validation, err := s.validator.Validate(aiOutput, contextPayload)
	if err != nil {
		return nil, failAndWrap(err, "validate ai output", contextPayload, aiOutput, nil, runOutcome{ErrorStage: errStageContextParse, ErrorMessage: err.Error(), FailRun: true}, 0, 0)
	}

	est := aicost.EstimateUSD(aiOutput.Model, aiOutput.InputTokens, aiOutput.OutputTokens)
	estCost := est.CostUSD
	if !est.Known {
		estCost = 0
	}

	openAlerts := countOpenAlertsInContext(contextPayload)
	outcome := evaluateRunOutcome(contextPayload, validation)
	if outcome.FailRun {
		return nil, failAndWrap(errors.New(outcome.ErrorMessage), "recommendation validation outcome", contextPayload, aiOutput, validation, outcome, 0, estCost)
	}

	upserted := 0
	linkedAlerts := 0
	warningsTotal := len(outcome.Warnings)
	for _, item := range validation.ValidRecommendations {
		rec := item.Recommendation
		warningsTotal += len(item.Warnings)
		fingerprint := recFingerprint(sellerAccountID, rec)
		recID, upsertErr := s.repo.UpsertRecommendation(ctx, UpsertRecommendationInput{
			SellerAccountID:    sellerAccountID,
			Source:             s.cfg.Source,
			RecommendationType: rec.RecommendationType,
			Horizon:            rec.Horizon,
			EntityType:         rec.EntityType,
			EntityID:           rec.EntityID,
			EntitySKU:          rec.EntitySKU,
			EntityOfferID:      rec.EntityOfferID,
			Title:              rec.Title,
			WhatHappened:       rec.WhatHappened,
			WhyItMatters:       rec.WhyItMatters,
			RecommendedAction:  rec.RecommendedAction,
			ExpectedEffect:     rec.ExpectedEffect,
			PriorityScore:      rec.PriorityScore,
			PriorityLevel:      rec.PriorityLevel,
			Urgency:            rec.Urgency,
			ConfidenceLevel:    item.FinalConfidenceLevel,
			SupportingMetrics:  rec.SupportingMetrics,
			Constraints:        rec.Constraints,
			AIModel:            aiOutput.Model,
			AIPromptVersion:    s.cfg.PromptVersion,
			RawAIResponse:      sanitizeRawAIResponse(aiOutput.RawResponse),
			Fingerprint:        fingerprint,
		})
		if upsertErr != nil {
			return nil, failAndWrap(upsertErr, "upsert recommendation", contextPayload, aiOutput, validation, outcome, upserted, estCost)
		}
		upserted++
		if delErr := s.repo.DeleteRecommendationAlertLinks(ctx, sellerAccountID, recID); delErr != nil {
			return nil, failAndWrap(delErr, "delete previous recommendation alert links", contextPayload, aiOutput, validation, outcome, upserted, estCost)
		}

		for _, alertID := range rec.SupportingAlertIDs {
			if linkErr := s.repo.LinkRecommendationAlert(ctx, sellerAccountID, recID, alertID); linkErr != nil {
				return nil, failAndWrap(linkErr, "link recommendation to alert", contextPayload, aiOutput, validation, outcome, upserted, estCost)
			}
			linkedAlerts++
		}
	}

	outcome = evaluateAfterSave(openAlerts, upserted, validation, outcome)
	if outcome.FailRun {
		return nil, failAndWrap(errors.New(outcome.ErrorMessage), "recommendation save outcome", contextPayload, aiOutput, validation, outcome, upserted, estCost)
	}

	if diagErr := persistDiagnostic(contextPayload, aiOutput, validation, outcome, upserted, estCost); diagErr != nil {
		return nil, fmt.Errorf("persist recommendation run diagnostic: %w", diagErr)
	}

	if err := s.repo.CompleteRun(ctx, CompleteRecommendationRunInput{
		RunID:                         runID,
		SellerAccountID:               sellerAccountID,
		InputTokens:                   aiOutput.InputTokens,
		OutputTokens:                  aiOutput.OutputTokens,
		EstimatedCost:                 estCost,
		GeneratedRecommendationsCount: upserted,
		AcceptedRecommendationsCount:  0,
	}); err != nil {
		return nil, fmt.Errorf("complete recommendation run id=%d: %w", runID, err)
	}

	return &GenerateForAccountSummary{
		SellerAccountID:   sellerAccountID,
		AsOfDate:          asOf,
		RunID:             runID,
		GeneratedTotal:    validation.TotalRecommendations,
		ValidTotal:        len(validation.ValidRecommendations),
		RejectedTotal:     len(validation.RejectedRecommendations),
		UpsertedTotal:     upserted,
		LinkedAlertsTotal: linkedAlerts,
		WarningsTotal:     warningsTotal,
		InputTokens:       aiOutput.InputTokens,
		OutputTokens:      aiOutput.OutputTokens,
		TotalTokens:       aiOutput.TotalTokens,
	}, nil
}

func (s *Service) ListRecommendations(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Recommendation, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.ListRecommendationsFiltered(ctx, sellerAccountID, filter)
}

func (s *Service) GetRecommendationByID(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return s.repo.GetRecommendationByID(ctx, sellerAccountID, recommendationID)
}

func (s *Service) GetRecommendationDetailByID(ctx context.Context, sellerAccountID int64, recommendationID int64) (RecommendationDetail, error) {
	rec, err := s.repo.GetRecommendationByID(ctx, sellerAccountID, recommendationID)
	if err != nil {
		return RecommendationDetail{}, err
	}
	related, err := s.repo.ListAlertsByRecommendationID(ctx, sellerAccountID, recommendationID)
	if err != nil {
		return RecommendationDetail{}, err
	}
	return RecommendationDetail{
		Recommendation: rec,
		RelatedAlerts:  related,
	}, nil
}

func (s *Service) AcceptRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return s.repo.AcceptRecommendation(ctx, sellerAccountID, recommendationID)
}

func (s *Service) DismissRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return s.repo.DismissRecommendation(ctx, sellerAccountID, recommendationID)
}

func (s *Service) ResolveRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	return s.repo.ResolveRecommendation(ctx, sellerAccountID, recommendationID)
}

func (s *Service) GetSummary(ctx context.Context, sellerAccountID int64) (Summary, error) {
	openTotal, err := s.repo.CountOpenRecommendations(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	byPriority, err := s.repo.CountOpenRecommendationsByPriority(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	byConfidence, err := s.repo.CountOpenRecommendationsByConfidence(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	latestRun, err := s.repo.GetLatestRecommendationRun(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	return Summary{
		OpenTotal:    openTotal,
		ByPriority:   byPriority,
		ByConfidence: byConfidence,
		LatestRun:    latestRun,
	}, nil
}

func (s *Service) AddFeedback(ctx context.Context, input AddRecommendationFeedbackInput) (*RecommendationFeedback, error) {
	if input.SellerAccountID <= 0 || input.RecommendationID <= 0 {
		return nil, ErrNotFound
	}
	rating := RecommendationFeedbackRating(strings.TrimSpace(string(input.Rating)))
	if rating != RecommendationFeedbackPositive && rating != RecommendationFeedbackNegative && rating != RecommendationFeedbackNeutral {
		return nil, errors.New("invalid feedback rating")
	}
	if input.Comment != nil {
		comment := strings.TrimSpace(*input.Comment)
		if comment == "" {
			input.Comment = nil
		} else {
			input.Comment = &comment
		}
	}
	input.Rating = rating
	if _, err := s.repo.GetRecommendationByID(ctx, input.SellerAccountID, input.RecommendationID); err != nil {
		return nil, err
	}
	return s.repo.CreateFeedback(ctx, input)
}

func recFingerprint(sellerAccountID int64, rec AIRecommendationCandidate) string {
	alertIDs := append([]int64(nil), rec.SupportingAlertIDs...)
	sort.Slice(alertIDs, func(i, j int) bool { return alertIDs[i] < alertIDs[j] })
	payload := struct {
		SellerAccountID    int64   `json:"seller_account_id"`
		RecommendationType string  `json:"recommendation_type"`
		Horizon            string  `json:"horizon"`
		EntityType         string  `json:"entity_type"`
		EntityID           *string `json:"entity_id,omitempty"`
		EntitySKU          *int64  `json:"entity_sku,omitempty"`
		EntityOfferID      *string `json:"entity_offer_id,omitempty"`
		Title              string  `json:"title"`
		AlertIDs           []int64 `json:"alert_ids,omitempty"`
	}{
		SellerAccountID:    sellerAccountID,
		RecommendationType: rec.RecommendationType,
		Horizon:            rec.Horizon,
		EntityType:         rec.EntityType,
		EntityID:           rec.EntityID,
		EntitySKU:          rec.EntitySKU,
		EntityOfferID:      rec.EntityOfferID,
		Title:              strings.ToLower(strings.TrimSpace(rec.Title)),
		AlertIDs:           alertIDs,
	}
	b, _ := json.Marshal(payload)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func sanitizeRawAIResponse(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	s := string(raw)
	s = openAIKeyRegex.ReplaceAllString(s, "[redacted-openai-key]")
	s = bearerTokenJSONRegex.ReplaceAllString(s, `"authorization":"Bearer [redacted]"`)
	return json.RawMessage(s)
}
