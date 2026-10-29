package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

var (
	ErrChatAskNotImplemented     = errors.New("chat ask flow is not implemented yet")
	ErrRepositoryRequired        = errors.New("chat repository is required")
	ErrQuestionRequired          = errors.New("chat question is required")
	ErrAIClientRequired          = errors.New("chat ai client is required")
	ErrToolExecutorRequired      = errors.New("chat tool executor is required")
	ErrToolPlanValidatorRequired = errors.New("chat tool plan validator is required")
	ErrContextAssemblerRequired  = errors.New("chat context assembler is required")
	ErrAnswerValidatorRequired   = errors.New("chat answer validator is required")
	ErrInvalidToolPlan           = errors.New("chat tool plan validation failed")
	ErrInvalidAIAnswer           = errors.New("chat answer validation failed")
	ErrAITemporarilyUnavailable  = errors.New("AI temporarily unavailable, try again later")
)

type Service struct {
	repo                   Repository
	aiClient               AIClient
	registry               *ToolRegistry
	planValidator          *ToolPlanValidator
	toolSet                ToolExecutor
	contextAssembler       *ContextAssembler
	answerValidator        *AnswerValidator
	prechecker             *Prechecker
	exporter               *chatexport.Exporter
	attachments            AttachmentRepository
	exportStorage          ExportObjectStorage
	inputStorage           InputAttachmentStorage
	inputBucket            string
	attachmentDownloadPath func(attachmentID string) string
	actionContextProvider  ActionContextProvider

	plannerModel         string
	answerModel          string
	plannerPromptVersion string
	answerPromptVersion  string
	dailyUsage           aicost.DailyUsageChecker
}

// ExportObjectStorage loads export objects for download.
type ExportObjectStorage interface {
	DownloadExport(ctx context.Context, storageKey string) ([]byte, error)
}

type ActionContextProvider interface {
	GetActionForChatContext(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionPromptContext, error)
}

func NewService(repo Repository) *Service {
	registry := NewDefaultToolRegistry()
	return &Service{
		repo:                 repo,
		registry:             registry,
		planValidator:        NewToolPlanValidator(registry),
		contextAssembler:     NewContextAssembler(),
		answerValidator:      NewAnswerValidator(),
		plannerPromptVersion: PlannerPromptVersion,
		answerPromptVersion:  AnswerPromptVersion,
	}
}

type ServiceConfig struct {
	PlannerModel         string
	AnswerModel          string
	PlannerPromptVersion string
	AnswerPromptVersion  string
}

type ServiceDeps struct {
	Repo                   Repository
	AIClient               AIClient
	ToolRegistry           *ToolRegistry
	ToolPlanValidator      *ToolPlanValidator
	ToolExecutor           ToolExecutor
	ContextAssembler       *ContextAssembler
	AnswerValidator        *AnswerValidator
	Prechecker             *Prechecker
	Exporter               *chatexport.Exporter
	Attachments            AttachmentRepository
	ExportStorage          ExportObjectStorage
	InputStorage           InputAttachmentStorage
	InputBucket            string
	AttachmentDownloadPath func(attachmentID string) string
	ActionContextProvider  ActionContextProvider
	Config                 ServiceConfig
	DailyUsage             aicost.DailyUsageChecker
}

func NewServiceWithDeps(deps ServiceDeps) (*Service, error) {
	if deps.Repo == nil {
		return nil, ErrRepositoryRequired
	}
	if deps.AIClient == nil {
		return nil, ErrAIClientRequired
	}
	if deps.ToolExecutor == nil {
		return nil, ErrToolExecutorRequired
	}
	registry := deps.ToolRegistry
	if registry == nil {
		registry = NewDefaultToolRegistry()
	}
	planValidator := deps.ToolPlanValidator
	if planValidator == nil {
		planValidator = NewToolPlanValidator(registry)
	}
	contextAssembler := deps.ContextAssembler
	if contextAssembler == nil {
		contextAssembler = NewContextAssembler()
	}
	answerValidator := deps.AnswerValidator
	if answerValidator == nil {
		answerValidator = NewAnswerValidator()
	}
	plannerPromptVersion := deps.Config.PlannerPromptVersion
	if plannerPromptVersion == "" {
		plannerPromptVersion = PlannerPromptVersion
	}
	answerPromptVersion := deps.Config.AnswerPromptVersion
	if answerPromptVersion == "" {
		answerPromptVersion = AnswerPromptVersion
	}
	prechecker := deps.Prechecker
	if prechecker == nil {
		prechecker = NewPrechecker(nil)
	}
	downloadPath := deps.AttachmentDownloadPath
	if downloadPath == nil {
		downloadPath = defaultAttachmentDownloadPath
	}
	return &Service{
		repo:                   deps.Repo,
		aiClient:               deps.AIClient,
		registry:               registry,
		planValidator:          planValidator,
		toolSet:                deps.ToolExecutor,
		contextAssembler:       contextAssembler,
		answerValidator:        answerValidator,
		prechecker:             prechecker,
		exporter:               deps.Exporter,
		attachments:            deps.Attachments,
		exportStorage:          deps.ExportStorage,
		inputStorage:           deps.InputStorage,
		inputBucket:            strings.TrimSpace(deps.InputBucket),
		attachmentDownloadPath: downloadPath,
		actionContextProvider:  deps.ActionContextProvider,
		plannerModel:           deps.Config.PlannerModel,
		answerModel:            deps.Config.AnswerModel,
		plannerPromptVersion:   plannerPromptVersion,
		answerPromptVersion:    answerPromptVersion,
		dailyUsage:             deps.DailyUsage,
	}, nil
}

func defaultAttachmentDownloadPath(attachmentID string) string {
	return "/api/v1/chat/attachments/" + attachmentID + "/download"
}

// NewDownloadOnlyService wires attachment download paths without the legacy ask pipeline.
func NewDownloadOnlyService(
	atts AttachmentRepository,
	input InputAttachmentStorage,
	inputBucket string,
	export ExportObjectStorage,
) *Service {
	return &Service{
		attachments:            atts,
		inputStorage:           input,
		inputBucket:            inputBucket,
		exportStorage:          export,
		attachmentDownloadPath: defaultAttachmentDownloadPath,
	}
}

type CreateSessionInput struct {
	SellerAccountID int64
	UserID          *int64
	Title           string
	ActionKey       *string
}

type CreateMessageInput struct {
	SessionID       int64
	SellerAccountID int64
	Role            ChatMessageRole
	Content         string
	MessageType     ChatMessageType
	ClientRequestID string
}

type AddFeedbackInput struct {
	SellerAccountID int64
	SessionID       int64
	MessageID       int64
	Rating          ChatFeedbackRating
	Comment         *string
}

func (s *Service) CreateSession(ctx context.Context, input CreateSessionInput) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	input.Title = normalizeSessionTitle(input.Title)
	return s.repo.CreateSession(ctx, input)
}

func (s *Service) ListSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	return s.repo.ListSessions(ctx, sellerAccountID, limit, offset)
}

func (s *Service) GetSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	return s.repo.GetSession(ctx, sellerAccountID, sessionID)
}

func (s *Service) RenameSession(ctx context.Context, sellerAccountID, sessionID int64, title string) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	normalized, err := validateSessionTitle(title)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateSessionTitle(ctx, sellerAccountID, sessionID, normalized)
}

func (s *Service) SoftDeleteSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	return s.repo.SoftDeleteSession(ctx, sellerAccountID, sessionID)
}

func validateSessionTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", errors.New("title is required")
	}
	if len([]rune(trimmed)) > 80 {
		return "", errors.New("title must be at most 80 characters")
	}
	return trimmed, nil
}

func (s *Service) ArchiveSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	return s.repo.ArchiveSession(ctx, sellerAccountID, sessionID)
}

func (s *Service) ListAttachmentsByMessageIDs(
	ctx context.Context,
	sellerAccountID int64,
	messageIDs []int64,
) (map[int64][]ChatAnswerAttachment, error) {
	if s.attachments == nil || len(messageIDs) == 0 {
		return map[int64][]ChatAnswerAttachment{}, nil
	}
	rows, err := s.attachments.ListAttachmentsByMessageIDs(ctx, sellerAccountID, messageIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][]ChatAnswerAttachment)
	for _, row := range rows {
		dto := mapAttachmentsToDTO([]ChatAttachment{row}, s.attachmentDownloadPath)
		if len(dto) == 0 || row.MessageID == nil {
			continue
		}
		out[*row.MessageID] = append(out[*row.MessageID], dto[0])
	}
	return out, nil
}

func (s *Service) ListMessages(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatMessage, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	return s.repo.ListMessages(ctx, sellerAccountID, sessionID, limit, offset)
}

func (s *Service) AddFeedback(ctx context.Context, input AddFeedbackInput) (*ChatFeedback, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	if input.SellerAccountID <= 0 || input.SessionID <= 0 || input.MessageID <= 0 {
		return nil, errors.New("seller_account_id, session_id and message_id must be > 0")
	}
	switch input.Rating {
	case ChatFeedbackRatingPositive, ChatFeedbackRatingNegative, ChatFeedbackRatingNeutral:
	default:
		return nil, errors.New("invalid feedback rating")
	}
	msg, err := s.repo.GetMessage(ctx, input.SellerAccountID, input.MessageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("message not found")
	}
	if msg.SessionID != input.SessionID {
		return nil, errors.New("message does not belong to session")
	}
	if msg.Role != ChatMessageRoleAssistant || msg.MessageType != ChatMessageTypeAnswer {
		return nil, errors.New("feedback is allowed only for assistant answer messages")
	}
	return s.repo.CreateFeedback(ctx, input)
}

type answerGenerator func(context.Context, GenerateAnswerInput) (*GenerateAnswerOutput, error)

func (s *Service) Ask(ctx context.Context, input AskInput) (*AskResult, error) {
	return s.askWithGenerator(ctx, input, func(ctx context.Context, answerInput GenerateAnswerInput) (*GenerateAnswerOutput, error) {
		return s.aiClient.GenerateAnswer(ctx, answerInput)
	})
}

func (s *Service) askWithGenerator(ctx context.Context, input AskInput, generate answerGenerator) (result *AskResult, err error) {
	question := strings.TrimSpace(input.Question)
	if question == "" {
		return nil, ErrQuestionRequired
	}
	if input.SellerAccountID <= 0 {
		return nil, errors.New("seller_account_id must be > 0")
	}
	if input.SessionID != nil && *input.SessionID <= 0 {
		return nil, errors.New("session_id must be > 0")
	}
	if input.UserID != nil && *input.UserID <= 0 {
		return nil, errors.New("user_id must be > 0")
	}
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	if s.aiClient == nil {
		return nil, ErrAIClientRequired
	}
	if s.planValidator == nil {
		return nil, ErrToolPlanValidatorRequired
	}
	if s.toolSet == nil {
		return nil, ErrToolExecutorRequired
	}
	if s.contextAssembler == nil {
		return nil, ErrContextAssemblerRequired
	}
	if s.answerValidator == nil {
		return nil, ErrAnswerValidatorRequired
	}
	if s.registry == nil {
		s.registry = NewDefaultToolRegistry()
	}
	clientRequestID := strings.TrimSpace(input.ClientRequestID)
	if requestIdempotency := newRequestIdempotency(s.repo); clientRequestID != "" && requestIdempotency != nil {
		requestHash := RequestHash(input.UserID, input.SellerAccountID, input.SessionID, input.ActionKey, question, input.AttachmentIDs, "ask")
		replayed, beginErr := requestIdempotency.BeginRequest(ctx, input.SellerAccountID, input.UserID, clientRequestID, nil, input.ActionKey, requestHash, "ask")
		if beginErr != nil {
			return nil, beginErr
		}
		if replayed != nil {
			return replayed, nil
		}
		defer func() {
			if err != nil {
				_ = requestIdempotency.FailRequest(ctx, input.SellerAccountID, clientRequestID, err)
			} else {
				_ = requestIdempotency.CompleteRequest(ctx, input.SellerAccountID, clientRequestID, result)
			}
		}()
	}

	session, sessionCreated, err := s.resolveSession(ctx, input, question)
	if err != nil {
		return nil, err
	}
	sessionReused := strings.TrimSpace(input.ActionKey) != "" && !sessionCreated && input.SessionID == nil

	var resumeUserMsg *ChatMessage
	if clientID := strings.TrimSpace(input.ClientRequestID); clientID != "" {
		if existing, lookupErr := s.repo.GetMessageByClientRequestID(ctx, input.SellerAccountID, clientID); lookupErr != nil {
			return nil, lookupErr
		} else if existing != nil {
			existingSession, sessErr := s.repo.GetSession(ctx, input.SellerAccountID, existing.SessionID)
			if sessErr != nil {
				return nil, sessErr
			}
			rebuilt, rebuildErr := s.rebuildAskResultFromUserMessage(ctx, input.SellerAccountID, existing, existingSession, false, sessionReused)
			if rebuildErr != nil {
				return nil, rebuildErr
			}
			// Completed prior ask — safe idempotent replay.
			if rebuilt != nil && rebuilt.AssistantMessageID != nil {
				return rebuilt, nil
			}
			// Incomplete prior attempt (user message saved, answer never persisted).
			// Resume the pipeline instead of returning an empty answer forever.
			session = existingSession
			resumeUserMsg = existing
			slog.Warn("chat ask resuming incomplete client_request_id",
				"seller_account_id", input.SellerAccountID,
				"session_id", session.ID,
				"user_message_id", existing.ID,
				"client_request_id", clientID,
			)
		}
	}

	inputAttachments, err := s.resolveInputAttachments(ctx, input.SellerAccountID, input.AttachmentIDs)
	if err != nil {
		return nil, err
	}
	if s.dailyUsage != nil && resumeUserMsg == nil {
		if limitErr := s.dailyUsage.CheckChatQuestion(ctx, input.SellerAccountID); limitErr != nil {
			if errors.Is(limitErr, aicost.ErrDailyLimitExceeded) {
				userMsg, msgErr := s.repo.CreateMessage(ctx, CreateMessageInput{
					SessionID: session.ID, SellerAccountID: input.SellerAccountID, Role: ChatMessageRoleUser,
					Content: question, MessageType: ChatMessageTypeQuestion,
				})
				if msgErr != nil {
					return nil, msgErr
				}
				trace, traceErr := s.repo.CreateTrace(ctx, CreateTraceInput{
					SessionID: session.ID, UserMessageID: &userMsg.ID, SellerAccountID: input.SellerAccountID,
					PlannerPromptVersion: safePromptVersion(s.plannerPromptVersion, PlannerPromptVersion),
					AnswerPromptVersion:  safePromptVersion(s.answerPromptVersion, AnswerPromptVersion),
					PlannerModel:         fallbackString(s.plannerModel, "unknown"),
					AnswerModel:          fallbackString(s.answerModel, "unknown"),
				})
				if traceErr != nil {
					return nil, traceErr
				}
				return s.completeControlledFailure(ctx, session, input, userMsg, trace, "daily_limit_exceeded",
					controlledMsgDailyLimit, []string{"OpenAI не вызывался: превышен дневной лимит."},
					map[string]any{"openai_called": false, "limit_error": limitErr.Error()})
			}
			return nil, limitErr
		}
	}
	var userMsg *ChatMessage
	if resumeUserMsg != nil {
		userMsg = resumeUserMsg
	} else {
		userMsg, err = s.repo.CreateMessage(ctx, CreateMessageInput{
			SessionID: session.ID, SellerAccountID: input.SellerAccountID, Role: ChatMessageRoleUser,
			Content: question, MessageType: ChatMessageTypeQuestion, ClientRequestID: strings.TrimSpace(input.ClientRequestID),
		})
		if err != nil {
			return nil, err
		}
	}
	_, _ = s.repo.TouchSession(ctx, input.SellerAccountID, session.ID)

	trace, err := s.repo.CreateTrace(ctx, CreateTraceInput{
		SessionID:            session.ID,
		UserMessageID:        &userMsg.ID,
		SellerAccountID:      input.SellerAccountID,
		PlannerPromptVersion: safePromptVersion(s.plannerPromptVersion, PlannerPromptVersion),
		AnswerPromptVersion:  safePromptVersion(s.answerPromptVersion, AnswerPromptVersion),
		PlannerModel:         fallbackString(s.plannerModel, "unknown"),
		AnswerModel:          fallbackString(s.answerModel, "unknown"),
	})
	if err != nil {
		return nil, err
	}

	inputFileIDs, attachmentWarnings, attachErr := s.attachInputFilesForAsk(ctx, input.SellerAccountID, session.ID, userMsg.ID, inputAttachments)
	attachedForCleanup := attachedItemsWithFileIDs(inputAttachments, inputFileIDs)
	if attachErr != nil {
		s.cleanupOpenAIFilesAfterAsk(ctx, input.SellerAccountID, attachedForCleanup)
		s.failTraceBestEffort(ctx, FailTraceInput{
			SellerAccountID: input.SellerAccountID,
			TraceID:         trace.ID,
			ErrorMessage:    errorString(attachErr),
		})
		return nil, attachErr
	}
	defer func() {
		if len(attachedForCleanup) > 0 {
			s.cleanupOpenAIFilesAfterAsk(ctx, input.SellerAccountID, attachedForCleanup)
		}
	}()

	precheck, err := s.prechecker.Evaluate(ctx, input.SellerAccountID, question, input.AsOfDate)
	if err != nil {
		s.failTraceBestEffort(ctx, FailTraceInput{
			SellerAccountID: input.SellerAccountID,
			TraceID:         trace.ID,
			ErrorMessage:    errorString(err),
		})
		return nil, err
	}
	if precheck.Route == PrecheckRouteExportExecute && precheck.ExportRequest != nil {
		return s.completeExportRequest(ctx, session, input, userMsg, trace, *precheck.ExportRequest)
	}
	if !precheck.OpenAICalled {
		return s.completeControlledPrecheck(ctx, session, input, userMsg, trace, precheck)
	}

	tools := s.registry.List()
	plannerSystemPrompt := BuildPlannerSystemPrompt(tools)
	actionPromptContext := s.resolveActionPromptContext(ctx, input.SellerAccountID, input.ActionKey)
	var plannerOutput *PlanToolsOutput
	var validatedPlan *ValidatedToolPlan
	if len(inputAttachments) > 0 {
		plannerOutput = &PlanToolsOutput{
			Plan: ToolPlan{
				Intent:      ChatIntentGeneralOverview,
				Confidence:  1,
				Language:    "ru",
				Assumptions: []string{"answer using attached file contents"},
			},
		}
		validatedPlan = &ValidatedToolPlan{
			Intent:      ChatIntentGeneralOverview,
			ToolCalls:   []ToolCall{},
			Assumptions: []string{"answer using attached file contents"},
			Warnings:    []string{"using attachment context only"},
		}
	} else {
		plannerUserPrompt := BuildPlannerUserPrompt(question, input.AsOfDate, actionPromptContext)
		var planErr error
		plannerOutput, planErr = s.aiClient.PlanTools(ctx, PlanToolsInput{
			SystemPrompt: plannerSystemPrompt,
			UserPrompt:   plannerUserPrompt,
		})
		if planErr != nil {
			err = wrapChatOpenAIUnavailable(planErr)
			if errors.Is(err, ErrAITemporarilyUnavailable) {
				slog.Error("chat planner unavailable", "trace_id", trace.ID, "seller_account_id", input.SellerAccountID, "err", err)
				s.failTraceBestEffort(ctx, FailTraceInput{
					SellerAccountID: input.SellerAccountID,
					TraceID:         trace.ID,
					ErrorMessage:    errorString(err),
				})
				return nil, err
			}
			return s.completeControlledFailure(ctx, session, input, userMsg, trace, "planner_failed", controlledMsgPlanner, nil, map[string]any{
				"planner_error": errorString(err),
			})
		}
		validatedPlan, err = resolveValidatedToolPlan(s.planValidator, plannerOutput.Plan, actionPromptContext)
		if err != nil {
			return s.completeControlledFailure(ctx, session, input, userMsg, trace, "invalid_tool_plan", controlledMsgInvalidPlan, nil, map[string]any{
				"validation_error": errorString(err),
				"raw_plan":         payloadMap(plannerOutput.Plan),
			})
		}
	}

	toolResults := make([]ToolResult, 0)
	if validatedPlan.Intent != ChatIntentUnsupported && len(validatedPlan.ToolCalls) > 0 {
		var toolErr error
		toolResults, toolErr = s.toolSet.ExecutePlan(ctx, input.SellerAccountID, *validatedPlan)
		if toolErr != nil && len(toolResults) == 0 {
			if actionPromptContext == nil {
				return s.completeControlledFailure(ctx, session, input, userMsg, trace, "tool_execution_failed", controlledMsgToolsFailed, nil, map[string]any{
					"tool_error": errorString(toolErr),
				})
			}
			validatedPlan.ToolCalls = []ToolCall{}
			validatedPlan.Warnings = append(validatedPlan.Warnings, "tool execution failed; using action context only")
			toolResults = []ToolResult{}
		}
	}

	factContext, err := s.contextAssembler.Assemble(AssembleContextInput{
		Question:        question,
		SellerAccountID: input.SellerAccountID,
		Plan:            *validatedPlan,
		ToolResults:     toolResults,
		AsOfDate:        input.AsOfDate,
		Language:        plannerOutput.Plan.Language,
	})
	if err != nil {
		return s.completeControlledFailure(ctx, session, input, userMsg, trace, "context_assembly_failed", controlledMsgContextFailed, nil, map[string]any{
			"assembly_error": errorString(err),
		})
	}
	factContext.Action = actionPromptContext
	injectAttachmentFacts(factContext, inputAttachments)
	for _, tr := range toolResults {
		if tr.Error != nil && strings.TrimSpace(*tr.Error) != "" {
			factContext.Limitations = append(factContext.Limitations, "Часть данных недоступна: "+tr.Name)
		}
	}
	if validatedPlan.Intent == ChatIntentUnsupported && validatedPlan.UnsupportedReason != nil {
		reason := strings.TrimSpace(*validatedPlan.UnsupportedReason)
		if reason != "" {
			factContext.Limitations = append(factContext.Limitations, reason)
		}
	}
	factContext.Limitations = append(factContext.Limitations, attachmentWarnings...)

	answerInput := GenerateAnswerInput{
		SystemPrompt: BuildAnswerSystemPrompt(),
		UserPrompt:   appendExtractedAttachmentContext(BuildAnswerUserPrompt(*factContext), inputAttachments),
		FactContext:  factContext,
		InputFileIDs: compactFileIDs(inputFileIDs),
		InputImages:  s.loadAnswerImages(ctx, inputAttachments),
	}
	answerOutput, err := generate(ctx, answerInput)
	if err != nil && len(compactFileIDs(inputFileIDs)) > 0 && attachmentsHaveExtractedText(inputAttachments) && !attachmentsRequireVision(inputAttachments) {
		// Provider may reject some binary formats; local extraction is already in the prompt.
		// Never strip vision inputs — images have only stub extraction text.
		answerInput.InputFileIDs = nil
		answerOutput, err = generate(ctx, answerInput)
	}
	if err != nil {
		err = wrapChatOpenAIUnavailable(err)
		if errors.Is(err, ErrAITemporarilyUnavailable) {
			slog.Error("chat answerer unavailable", "trace_id", trace.ID, "seller_account_id", input.SellerAccountID, "err", err)
			s.failTraceBestEffort(ctx, FailTraceInput{
				SellerAccountID:          input.SellerAccountID,
				TraceID:                  trace.ID,
				DetectedIntent:           validatedPlan.Intent,
				ToolPlanPayload:          payloadMap(plannerOutput.Plan),
				ValidatedToolPlanPayload: payloadMap(validatedPlan),
				ToolResultsPayload:       payloadMap(toolResults),
				FactContextPayload:       payloadMap(factContext),
				RawPlannerResponse:       payloadJSONMap(plannerOutput.RawResponse),
				ErrorMessage:             errorString(err),
				InputTokens:              plannerOutput.InputTokens,
				OutputTokens:             plannerOutput.OutputTokens,
			})
			return nil, err
		}
		return s.completeControlledFailure(ctx, session, input, userMsg, trace, "openai_answer_failed", controlledMsgPlanner, nil, map[string]any{
			"answer_error": errorString(err),
		})
	}
	normalizeAnswerForValidation(&answerOutput.Answer, factContext)
	validation, err := s.answerValidator.Validate(&answerOutput.Answer, factContext)
	if err != nil || !validation.IsValid {
		// One soft repair pass for common no-facts / missing supporting-fact shape issues.
		normalizeAnswerForValidation(&answerOutput.Answer, factContext)
		validation, err = s.answerValidator.Validate(&answerOutput.Answer, factContext)
	}
	if err != nil || !validation.IsValid {
		lims := validation.Errors
		if len(lims) == 0 {
			lims = validation.Warnings
		}
		return s.completeControlledFailure(ctx, session, input, userMsg, trace, "answer_validation_failed", controlledMsgAnswerInvalid, lims, map[string]any{
			"validation": payloadMap(validation),
		})
	}

	finalAnswer := answerOutput.Answer
	finalAnswer.ConfidenceLevel = validation.FinalConfidenceLevel
	assistantMsg, err := s.repo.CreateMessage(ctx, CreateMessageInput{
		SessionID: session.ID, SellerAccountID: input.SellerAccountID, Role: ChatMessageRoleAssistant,
		Content: strings.TrimSpace(finalAnswer.Answer), MessageType: ChatMessageTypeAnswer,
	})
	if err != nil {
		s.failTraceBestEffort(ctx, FailTraceInput{
			SellerAccountID:          input.SellerAccountID,
			TraceID:                  trace.ID,
			DetectedIntent:           validatedPlan.Intent,
			ToolPlanPayload:          payloadMap(plannerOutput.Plan),
			ValidatedToolPlanPayload: payloadMap(validatedPlan),
			ToolResultsPayload:       payloadMap(toolResults),
			FactContextPayload:       payloadMap(factContext),
			RawPlannerResponse:       payloadJSONMap(plannerOutput.RawResponse),
			RawAnswerResponse:        payloadJSONMap(answerOutput.RawResponse),
			AnswerValidationPayload:  payloadMap(validation),
			InputTokens:              plannerOutput.InputTokens + answerOutput.InputTokens,
			OutputTokens:             plannerOutput.OutputTokens + answerOutput.OutputTokens,
			ErrorMessage:             errorString(err),
		})
		return nil, err
	}
	_, _ = s.repo.TouchSession(ctx, input.SellerAccountID, session.ID)

	totalIn := int(plannerOutput.InputTokens + answerOutput.InputTokens)
	totalOut := int(plannerOutput.OutputTokens + answerOutput.OutputTokens)
	priceModel := strings.TrimSpace(s.answerModel)
	if priceModel == "" {
		priceModel = "unknown"
	}
	est := aicost.EstimateUSD(priceModel, totalIn, totalOut)
	estCost := est.CostUSD
	if !est.Known {
		estCost = 0
	}

	traceDiagnostics := buildChatTraceDiagnostics(input, actionPromptContext, true, false)
	_, err = s.repo.CompleteTrace(ctx, CompleteTraceInput{
		SellerAccountID:          input.SellerAccountID,
		TraceID:                  trace.ID,
		AssistantMessageID:       &assistantMsg.ID,
		DetectedIntent:           validatedPlan.Intent,
		ToolPlanPayload:          payloadMap(plannerOutput.Plan),
		ValidatedToolPlanPayload: mergeTraceDiagnostics(payloadMap(validatedPlan), traceDiagnostics),
		ToolResultsPayload:       payloadMap(toolResults),
		FactContextPayload:       payloadMap(factContext),
		RawPlannerResponse:       payloadJSONMap(plannerOutput.RawResponse),
		RawAnswerResponse:        payloadJSONMap(answerOutput.RawResponse),
		AnswerValidationPayload:  payloadMap(validation),
		InputTokens:              plannerOutput.InputTokens + answerOutput.InputTokens,
		OutputTokens:             plannerOutput.OutputTokens + answerOutput.OutputTokens,
		EstimatedCost:            estCost,
	})
	if err != nil {
		return nil, err
	}
	return &AskResult{
		SessionID:          session.ID,
		SessionCreated:     sessionCreated,
		SessionReused:      sessionReused,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: &assistantMsg.ID,
		TraceID:            trace.ID,
		Answer:             &finalAnswer,
	}, nil
}

// AskStream shares Ask's precheck, planning, tool and persistence pipeline,
// but receives answer text directly from the provider as it is generated.
func (s *Service) AskStream(ctx context.Context, input AskInput, emit func(AskStreamEvent) error) (*AskResult, error) {
	if emit == nil {
		return nil, errors.New("chat stream emitter is required")
	}
	requestID := strings.TrimSpace(input.ClientRequestID)
	emitEvent := func(event AskStreamEvent) error {
		event.RequestID = requestID
		return emit(event)
	}
	if err := emitEvent(AskStreamEvent{Type: "status", Status: "queued"}); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = emitEvent(AskStreamEvent{Type: "status", Status: "cancelled"})
		return nil, err
	}
	if err := emitEvent(AskStreamEvent{Type: "status", Status: "running"}); err != nil {
		return nil, err
	}

	metrics := StreamMetrics{ProviderStartedAt: time.Now().UTC()}
	result, err := s.askWithGenerator(ctx, input, func(streamCtx context.Context, answerInput GenerateAnswerInput) (*GenerateAnswerOutput, error) {
		if err := emitEvent(AskStreamEvent{Type: "status", Status: "streaming"}); err != nil {
			return nil, err
		}
		output, streamErr := s.aiClient.StreamAnswer(streamCtx, answerInput, func(delta string) error {
			if strings.TrimSpace(delta) == "" {
				return nil
			}
			now := time.Now().UTC()
			if metrics.FirstDeltaAt.IsZero() {
				metrics.FirstDeltaAt = now
				metrics.TimeToFirstDeltaMS = now.Sub(metrics.ProviderStartedAt).Milliseconds()
			}
			metrics.EmittedDeltaCount++
			return emitEvent(AskStreamEvent{Type: "delta", Delta: delta})
		})
		if output != nil {
			metrics.ProviderResponseID = firstNonEmpty(output.ProviderResponseID, output.RequestID)
		}
		return output, streamErr
	})
	metrics.CompletedAt = time.Now().UTC()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			metrics.TerminationReason = "cancelled"
			s.logStreamMetrics(input, metrics)
			_ = emitEvent(AskStreamEvent{Type: "status", Status: "cancelled"})
			return nil, ctx.Err()
		}
		metrics.TerminationReason = "error"
		s.logStreamMetrics(input, metrics)
		_ = emitEvent(AskStreamEvent{Type: "error", Status: "failed", Error: errorString(err)})
		return nil, err
	}
	if result == nil || result.Answer == nil {
		err := errors.New("chat answer was not produced")
		_ = emitEvent(AskStreamEvent{Type: "error", Status: "failed", Error: err.Error()})
		return nil, err
	}
	metrics.TerminationReason = "completed"
	s.logStreamMetrics(input, metrics)
	if err := emitEvent(AskStreamEvent{Type: "done", Status: "completed", Result: result}); err != nil {
		return nil, err
	}
	return result, nil
}

// StreamMetrics is emitted to logs to make provider latency and termination
// observable without leaking answer content.
type StreamMetrics struct {
	ProviderStartedAt  time.Time
	FirstDeltaAt       time.Time
	CompletedAt        time.Time
	TimeToFirstDeltaMS int64
	EmittedDeltaCount  int
	ProviderResponseID string
	TerminationReason  string
}

func (s *Service) logStreamMetrics(input AskInput, metrics StreamMetrics) {
	slog.Info("chat answer stream completed",
		"seller_account_id", input.SellerAccountID,
		"provider_started_at", metrics.ProviderStartedAt,
		"first_delta_at", metrics.FirstDeltaAt,
		"completed_at", metrics.CompletedAt,
		"time_to_first_delta_ms", metrics.TimeToFirstDeltaMS,
		"emitted_delta_count", metrics.EmittedDeltaCount,
		"provider_response_id", metrics.ProviderResponseID,
		"termination_reason", metrics.TerminationReason,
	)
}

func injectAttachmentFacts(ctx *FactContext, items []ChatAttachment) {
	if ctx == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		fact := map[string]any{
			"source":     "attachment",
			"id":         item.ID.String(),
			"file_name":  item.FileName,
			"mime_type":  item.MimeType,
			"format":     item.Format,
			"status":     item.Status,
			"extraction": item.ExtractionStatus,
		}
		if item.ExtractedText != nil && strings.TrimSpace(*item.ExtractedText) != "" {
			text := strings.TrimSpace(*item.ExtractedText)
			const maxChars = 12000
			if len([]rune(text)) > maxChars {
				text = string([]rune(text)[:maxChars]) + "…"
			}
			fact["extracted_text"] = text
		}
		if len(item.ExtractedMeta) > 0 {
			fact["extracted_meta"] = string(item.ExtractedMeta)
		}
		ctx.Facts.Other = append(ctx.Facts.Other, fact)
		ctx.ContextStats.TotalItemsIncluded++
	}
}

func compactFileIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}

func attachmentsHaveExtractedText(items []ChatAttachment) bool {
	for _, item := range items {
		if item.ExtractedText != nil && strings.TrimSpace(*item.ExtractedText) != "" {
			return true
		}
	}
	return false
}

func attachmentsRequireVision(items []ChatAttachment) bool {
	for _, item := range items {
		format := strings.ToLower(strings.TrimSpace(item.Format))
		if format == "" {
			format = strings.ToLower(strings.TrimPrefix(filepath.Ext(item.FileName), "."))
		}
		switch format {
		case "png", "jpg", "jpeg", "webp":
			return true
		}
		if item.ExtractedText != nil && strings.Contains(*item.ExtractedText, "vision_required=true") {
			return true
		}
	}
	return false
}

func (s *Service) loadAnswerImages(ctx context.Context, items []ChatAttachment) []AnswerImageInput {
	if s.inputStorage == nil || strings.TrimSpace(s.inputBucket) == "" || len(items) == 0 {
		return nil
	}
	out := make([]AnswerImageInput, 0)
	for _, item := range items {
		format := strings.ToLower(strings.TrimSpace(item.Format))
		if format == "" {
			format = strings.ToLower(strings.TrimPrefix(filepath.Ext(item.FileName), "."))
		}
		switch format {
		case "png", "jpg", "jpeg", "webp":
		default:
			continue
		}
		data, err := s.inputStorage.DownloadInputAttachment(ctx, s.inputBucket, item.StorageKey)
		if err != nil || len(data) == 0 {
			continue
		}
		mime := resolveImageMIMEType(format, item.MimeType, data)
		out = append(out, AnswerImageInput{MimeType: mime, Data: data})
	}
	return out
}

func attachedItemsWithFileIDs(items []ChatAttachment, fileIDs []string) []ChatAttachment {
	if len(items) == 0 {
		return nil
	}
	out := make([]ChatAttachment, len(items))
	for i, item := range items {
		out[i] = item
		if i < len(fileIDs) && strings.TrimSpace(fileIDs[i]) != "" {
			id := fileIDs[i]
			out[i].OpenAIFileID = &id
		}
	}
	return out
}

func normalizeSessionTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "Новый чат"
	}
	return trimmed
}

func (s *Service) completeControlledPrecheck(
	ctx context.Context,
	session *ChatSession,
	input AskInput,
	userMsg *ChatMessage,
	trace *ChatTrace,
	precheck PrecheckDecision,
) (*AskResult, error) {
	slog.Warn("chat precheck rejected or controlled",
		"trace_id", trace.ID,
		"seller_account_id", input.SellerAccountID,
		"session_id", session.ID,
		"precheck_result", precheck.Route,
		"unsupported_reason", precheck.UnsupportedReason,
		"openai_called", false,
		"question_len", len(strings.TrimSpace(input.Question)),
	)

	answerText := FormatControlledAnswer(precheck.Message, precheck.SuggestedRephrases)
	intent := ChatIntentUnknown
	if precheck.UnsupportedReason == PrecheckReasonUnsupportedTopic {
		intent = ChatIntentUnsupported
	}

	finalAnswer := ChatAnswer{
		Answer:          answerText,
		Summary:         strings.TrimSpace(precheck.Message),
		Intent:          intent,
		ConfidenceLevel: ConfidenceLevelHigh,
		Limitations:     []string{"Ответ сформирован без вызова OpenAI (precheck)."},
	}

	assistantMsg, err := s.repo.CreateMessage(ctx, CreateMessageInput{
		SessionID:       session.ID,
		SellerAccountID: input.SellerAccountID,
		Role:            ChatMessageRoleAssistant,
		Content:         answerText,
		MessageType:     ChatMessageTypeAnswer,
	})
	if err != nil {
		s.failTraceBestEffort(ctx, FailTraceInput{
			SellerAccountID:          input.SellerAccountID,
			TraceID:                  trace.ID,
			DetectedIntent:           intent,
			ValidatedToolPlanPayload: map[string]any{"precheck": precheck.TracePayload(input.Question)},
			ErrorMessage:             errorString(err),
		})
		return nil, err
	}
	_, _ = s.repo.TouchSession(ctx, input.SellerAccountID, session.ID)

	precheckPayload := precheck.TracePayload(input.Question)
	_, err = s.repo.CompleteTrace(ctx, CompleteTraceInput{
		SellerAccountID:          input.SellerAccountID,
		TraceID:                  trace.ID,
		AssistantMessageID:       &assistantMsg.ID,
		DetectedIntent:           intent,
		ToolPlanPayload:          map[string]any{"precheck": precheckPayload},
		ValidatedToolPlanPayload: map[string]any{"precheck": precheckPayload},
		AnswerValidationPayload:  map[string]any{"precheck_controlled": true},
	})
	if err != nil {
		return nil, err
	}
	return &AskResult{
		SessionID:          session.ID,
		SessionCreated:     false,
		SessionReused:      false,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: &assistantMsg.ID,
		TraceID:            trace.ID,
		Answer:             &finalAnswer,
	}, nil
}

func (s *Service) resolveActionPromptContext(ctx context.Context, sellerAccountID int64, actionKey string) *ActionPromptContext {
	key := strings.TrimSpace(actionKey)
	if key == "" || s.actionContextProvider == nil || sellerAccountID <= 0 {
		return nil
	}
	context, err := s.actionContextProvider.GetActionForChatContext(ctx, sellerAccountID, key)
	if err != nil {
		slog.Warn("chat action context unavailable", "seller_account_id", sellerAccountID, "action_key", key, "err", err)
		return nil
	}
	if context == nil {
		return nil
	}
	return context
}

func buildChatTraceDiagnostics(input AskInput, actionContext *ActionPromptContext, openAICalled bool, controlledFallback bool) map[string]any {
	source := strings.TrimSpace(input.ChatCreationSource)
	if source == "" && strings.TrimSpace(input.ActionKey) != "" {
		source = "action_ask_button"
	}
	if source == "" {
		source = "user_compose"
	}
	out := map[string]any{
		"chat_creation_source":  source,
		"action_key":            strings.TrimSpace(input.ActionKey),
		"action_context_loaded": actionContext != nil,
		"openai_called":         openAICalled,
		"controlled_fallback":   controlledFallback,
	}
	if actionContext != nil {
		out["context_source"] = "action_context"
	}
	return out
}

func mergeTraceDiagnostics(base map[string]any, diagnostics map[string]any) map[string]any {
	if len(diagnostics) == 0 {
		return base
	}
	out := make(map[string]any, len(base)+len(diagnostics))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range diagnostics {
		out[k] = v
	}
	return out
}

func wrapChatOpenAIUnavailable(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrOpenAIRequestTooLarge) {
		return err
	}
	if openaix.IsTemporarilyUnavailable(err) {
		return fmt.Errorf("%w: %w", ErrAITemporarilyUnavailable, err)
	}
	return err
}

func (s *Service) failTraceBestEffort(ctx context.Context, input FailTraceInput) {
	if s.repo == nil || input.TraceID <= 0 || input.SellerAccountID <= 0 {
		return
	}
	_, _ = s.repo.FailTrace(ctx, input)
}

func payloadMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"serialization_error": err.Error()}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err == nil {
		return sanitizeTracePayload(out)
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return sanitizeTracePayload(map[string]any{"items": arr})
	}
	return sanitizeTracePayload(map[string]any{"raw": string(raw)})
}

func payloadJSONMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err == nil {
		return sanitizeTracePayload(out)
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return sanitizeTracePayload(map[string]any{"items": arr})
	}
	return sanitizeTracePayload(map[string]any{"raw": string(raw)})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func safePromptVersion(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func fallbackString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
