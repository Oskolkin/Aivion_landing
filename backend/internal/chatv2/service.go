package chatv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	Store             *Store
	Resolver          *ContextResolver
	Agent             *Agent
	Actions           ActionViews
	Downloads         func(id string) string
	Exporter          *FactualExporter
	IdempotencyLease  time.Duration
	HeartbeatInterval time.Duration
}

func (s *Service) leaseTTL() time.Duration {
	if s.IdempotencyLease > 0 {
		return s.IdempotencyLease
	}
	return 2 * time.Minute
}

func (s *Service) heartbeatEvery() time.Duration {
	if s.HeartbeatInterval > 0 {
		return s.HeartbeatInterval
	}
	return 30 * time.Second
}

func (s *Service) Ask(ctx context.Context, in AskInput) (*AskResult, error) {
	return s.ask(ctx, in, nil)
}

func (s *Service) AskStream(ctx context.Context, in AskInput, emit func(StreamEvent) error) (*AskResult, error) {
	return s.ask(ctx, in, emit)
}

func (s *Service) ask(ctx context.Context, in AskInput, emit func(StreamEvent) error) (*AskResult, error) {
	ctx, span := observability.Tracer(observability.TracerChat).Start(ctx, "chat.context.resolve")
	defer span.End()
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return nil, ErrQuestionRequired
	}
	emitStatus := func(st string) {
		if emit != nil {
			_ = emit(StreamEvent{Type: "status", Status: st})
		}
	}
	emitStatus("queued")
	reqHash := RequestHash(in.SellerAccountID, in.UserID, in.SessionID, question, in.AttachmentIDs, in.BindingID)
	owner := uuid.NewString()
	claimed := false
	if strings.TrimSpace(in.ClientRequestID) != "" {
		row, done, err := s.Store.BeginIdempotency(ctx, in.SellerAccountID, in.UserID, in.ClientRequestID, reqHash, owner, time.Now().Add(s.leaseTTL()))
		if err != nil {
			return nil, err
		}
		if done {
			if row.Status == "completed" {
				if replay, ok, rerr := s.Store.ReplayCompletedAsk(ctx, in.SellerAccountID, in.ClientRequestID); rerr == nil && ok {
					return replay, nil
				}
			}
			var existing AskResult
			if jsonUnmarshal(row.ResponsePayload, &existing) {
				return &existing, nil
			}
			if row.TurnID != nil && *row.TurnID > 0 {
				if rec, rerr := s.Store.reconstructAsk(*row.TurnID); rerr == nil {
					if row.Status == "cancelled" {
						rec.Cancelled = true
						rec.ErrorCode = "CANCELLED"
					}
					return rec, nil
				}
			}
			return nil, ErrFinalizationFailed
		}
		claimed = true
		var existingTurn int64
		if row.TurnID != nil {
			existingTurn = *row.TurnID
		}
		if row.UserMessageID != nil && *row.UserMessageID > 0 && row.SessionID != nil {
			return s.continueOwnedAsk(ctx, in, emit, emitStatus, owner, *row.SessionID, *row.UserMessageID, existingTurn, question)
		}
	}

	lost := &atomic.Bool{}
	stopHB := s.startHeartbeat(ctx, in.SellerAccountID, in.ClientRequestID, owner, lost)
	defer stopHB()

	failSetup := func(sessionID, userMsgID, turnID int64, err error) (*AskResult, error) {
		finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if isRequestCancelled(ctx) {
			res := &AskResult{SessionID: sessionID, TurnID: turnID, UserMessageID: userMsgID, Cancelled: true, ErrorCode: "CANCELLED"}
			if claimed {
				if ferr := s.Store.FinalizeCancelledTurn(finCtx, in.SellerAccountID, sessionID, turnID, userMsgID, nil, in.ClientRequestID, owner, res); ferr != nil {
					return nil, ferr
				}
			}
			return res, ErrCancelled
		}
		if claimed || turnID > 0 {
			code := setupFailureCode(ctx, err)
			res := &AskResult{SessionID: sessionID, TurnID: turnID, UserMessageID: userMsgID, ErrorCode: code}
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, nil, code, in.ClientRequestID, owner, res); ferr != nil {
				return nil, fmt.Errorf("setup failed and terminal finalization failed: %w", ferr)
			}
		}
		return nil, err
	}

	emitStatus("resolving_context")
	sessionID := int64(0)
	created := false
	if in.SessionID != nil && *in.SessionID > 0 {
		sess, err := s.Store.GetSession(ctx, in.SellerAccountID, *in.SessionID)
		if err != nil {
			return failSetup(0, 0, 0, err)
		}
		sessionID = sess.ID
	} else {
		title := clipExcerpt(question, 80)
		sess, err := s.Store.CreateSession(ctx, in.SellerAccountID, in.UserID, title)
		if err != nil {
			return failSetup(0, 0, 0, err)
		}
		sessionID = sess.ID
		created = true
	}
	userMsgID, err := s.Store.CreateMessage(ctx, sessionID, in.SellerAccountID, "user", question, "question")
	if err != nil {
		return failSetup(sessionID, 0, 0, err)
	}
	if claimed {
		if berr := s.Store.BindIdempotencyProgress(ctx, in.SellerAccountID, in.ClientRequestID, owner, sessionID, userMsgID, 0); berr != nil {
			return failSetup(sessionID, userMsgID, 0, berr)
		}
	}
	return s.runOwnedAsk(ctx, in, emit, emitStatus, owner, claimed, lost, sessionID, created, userMsgID, 0, question)
}

func (s *Service) continueOwnedAsk(ctx context.Context, in AskInput, emit func(StreamEvent) error, emitStatus func(string), owner string, sessionID, userMsgID, existingTurn int64, question string) (*AskResult, error) {
	lost := &atomic.Bool{}
	stopHB := s.startHeartbeat(ctx, in.SellerAccountID, in.ClientRequestID, owner, lost)
	defer stopHB()
	return s.runOwnedAsk(ctx, in, emit, emitStatus, owner, true, lost, sessionID, false, userMsgID, existingTurn, question)
}

func (s *Service) runOwnedAsk(ctx context.Context, in AskInput, emit func(StreamEvent) error, emitStatus func(string), owner string, claimed bool, lost *atomic.Bool, sessionID int64, created bool, userMsgID, existingTurn int64, question string) (*AskResult, error) {
	start := time.Now()
	failSetup := func(turnID int64, err error) (*AskResult, error) {
		finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if isRequestCancelled(ctx) {
			res := &AskResult{SessionID: sessionID, SessionCreated: created, TurnID: turnID, UserMessageID: userMsgID, Cancelled: true, ErrorCode: "CANCELLED", Answer: "", Limitations: []string{"Генерация остановлена."}}
			if turnID > 0 || claimed {
				if ferr := s.Store.FinalizeCancelledTurn(finCtx, in.SellerAccountID, sessionID, turnID, userMsgID, nil, in.ClientRequestID, owner, res); ferr != nil {
					return nil, ferr
				}
			}
			return res, ErrCancelled
		}
		if claimed || turnID > 0 {
			code := setupFailureCode(ctx, err)
			res := &AskResult{SessionID: sessionID, TurnID: turnID, UserMessageID: userMsgID, ErrorCode: code}
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, nil, code, in.ClientRequestID, owner, res); ferr != nil {
				return nil, fmt.Errorf("setup failed and terminal finalization failed: %w", ferr)
			}
		}
		return nil, err
	}

	frozen, err := s.Resolver.Resolve(ctx, in.SellerAccountID, sessionID, question, in.AttachmentIDs, in.BindingID, userMsgID)
	if err != nil {
		return failSetup(0, err)
	}
	frozen.UserID = in.UserID
	var pubID *int64
	var uni *string
	if frozen.Publication != nil {
		id := frozen.Publication.ID
		pubID = &id
		u := frozen.Publication.UniverseID
		uni = &u
	}
	turnID := existingTurn
	if turnID == 0 {
		turnID, err = s.Store.InsertTurn(ctx, TurnRow{
			SessionID: sessionID, SellerID: in.SellerAccountID, UserMessageID: userMsgID,
			AnswerMode: ModeGeneralGuidance, ContextMode: frozen.ContextMode, Status: StatusResolvingContext,
			PublicationID: pubID, UniverseID: uni, ModelPolicyVersion: ModelPolicyV1, ToolPolicyVersion: ToolPolicyV1,
			MemoryPolicyVersion: MemoryPolicyV1, PromptVersion: PromptVersionV1, ResultSchemaVersion: ResultSchemaV1,
			GroundingStatus: GroundingNone, ResultHash: frozen.FactualIdentity,
		})
		if err != nil {
			return failSetup(0, err)
		}
		if claimed {
			if berr := s.Store.BindIdempotencyProgress(ctx, in.SellerAccountID, in.ClientRequestID, owner, sessionID, userMsgID, turnID); berr != nil {
				return failSetup(turnID, berr)
			}
		}
		if err := s.Store.InsertTurnContext(ctx, turnID, frozen); err != nil {
			return failSetup(turnID, err)
		}
	} else if claimed {
		if berr := s.Store.BindIdempotencyProgress(ctx, in.SellerAccountID, in.ClientRequestID, owner, sessionID, userMsgID, turnID); berr != nil {
			return failSetup(turnID, berr)
		}
	}
	_ = s.Store.UpdateTurnStatus(ctx, turnID, StatusAnalyzing, "")

	onStatus := func(st TurnStatus) {
		_ = s.Store.UpdateTurnStatus(ctx, turnID, st, "")
		switch st {
		case StatusReadingData:
			emitStatus("reading_data")
		case StatusAnalyzing:
			emitStatus("analyzing")
		case StatusValidating:
			emitStatus("validating")
		}
	}
	out, err := s.Agent.Run(ctx, frozen, onStatus)
	if lost.Load() {
		finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res := failedResult(sessionID, created, turnID, userMsgID, frozen, "OWNERSHIP_LOST")
		if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, "OWNERSHIP_LOST", in.ClientRequestID, owner, res); ferr != nil && !errors.Is(ferr, ErrIdempotencyOwnershipLost) {
			return nil, ferr
		}
		return nil, ErrIdempotencyOwnershipLost
	}
	if err != nil {
		code := out.ErrorCode
		finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if isRequestCancelled(ctx) {
			res := &AskResult{SessionID: sessionID, SessionCreated: created, TurnID: turnID, UserMessageID: userMsgID, ContextMode: frozen.ContextMode, Cancelled: true, ErrorCode: "CANCELLED", Answer: "", Limitations: []string{"Генерация остановлена."}}
			if ferr := s.Store.FinalizeCancelledTurn(finCtx, in.SellerAccountID, sessionID, turnID, userMsgID, out.ToolCalls, in.ClientRequestID, owner, res); ferr != nil {
				return nil, ferr
			}
			if emit != nil {
				_ = emit(StreamEvent{Type: "status", Status: "cancelled"})
				_ = emit(StreamEvent{Type: "error", Status: "cancelled", Error: "cancelled"})
			}
			return res, ErrCancelled
		}
		if timeoutCode := internalAgentErrorCode(err); timeoutCode != "" {
			res := failedResult(sessionID, created, turnID, userMsgID, frozen, timeoutCode)
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, timeoutCode, in.ClientRequestID, owner, res); ferr != nil {
				return nil, fmt.Errorf("agent timeout and terminal finalization failed: %w", ferr)
			}
			if emit != nil {
				_ = emit(StreamEvent{Type: "status", Status: "failed"})
				_ = emit(StreamEvent{Type: "error", Status: "failed", Error: res.Answer})
			}
			return res, nil
		}
		if out.Failed || code == "PROVIDER_FAILED" || code == "VALIDATION_FAILED" || code == "TOOL_BUDGET" {
			if code == "" {
				code = "PROVIDER_FAILED"
			}
			res := failedResult(sessionID, created, turnID, userMsgID, frozen, code)
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, code, in.ClientRequestID, owner, res); ferr != nil {
				return nil, fmt.Errorf("agent failed and terminal finalization failed: %w", ferr)
			}
			if emit != nil {
				_ = emit(StreamEvent{Type: "status", Status: "failed"})
				_ = emit(StreamEvent{Type: "error", Status: "failed", Error: res.Answer})
			}
			return res, nil
		}
		failCode := code
		if failCode == "" {
			failCode = "PROVIDER_FAILED"
		}
		res := failedResult(sessionID, created, turnID, userMsgID, frozen, failCode)
		if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, failCode, in.ClientRequestID, owner, res); ferr != nil {
			return nil, fmt.Errorf("agent failed and terminal finalization failed: %w", ferr)
		}
		if emit != nil {
			_ = emit(StreamEvent{Type: "status", Status: "failed"})
			_ = emit(StreamEvent{Type: "error", Status: "failed", Error: res.Answer})
		}
		return res, nil
	}

	result := out.Result
	if frozen.ContextMode == ContextPrepublicationFactual && result.AnswerMode == ModePublishedExplanation {
		result.AnswerMode = ModeFactualQuery
		result.Limitations = append(result.Limitations, PrepublicationCopy())
	}
	result.ReferencedEntities = uniqueEntityRefs(append(append([]EntityRef{}, frozen.ReferencedEntities...), result.ReferencedEntities...), 32)
	gstatus := GroundingStatusFor(result, frozen.Limitations)

	var prepared *PreparedExport
	if result.AnswerMode == ModeExport {
		if s.Exporter == nil {
			res := failedResult(sessionID, created, turnID, userMsgID, frozen, "EXPORT_FAILED")
			finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, "EXPORT_FAILED", in.ClientRequestID, owner, res); ferr != nil {
				return nil, ferr
			}
			return res, nil
		}
		p, eerr := s.Exporter.Prepare(ctx, in.SellerAccountID, sessionID, frozen, result)
		if eerr != nil {
			slog.Error("chat export prepare failed", "turn_id", turnID, "err", eerr)
			res := failedResult(sessionID, created, turnID, userMsgID, frozen, "EXPORT_FAILED")
			finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if ferr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, out.ToolCalls, "EXPORT_FAILED", in.ClientRequestID, owner, res); ferr != nil {
				return nil, fmt.Errorf("export failed and terminal finalization failed: %w", ferr)
			}
			if emit != nil {
				_ = emit(StreamEvent{Type: "status", Status: "failed"})
				_ = emit(StreamEvent{Type: "error", Status: "failed", Error: res.Answer})
			}
			return res, nil
		}
		prepared = p
	}

	var grounding []GroundingEntry
	if out.Registry != nil {
		grounding = out.Registry.All()
	}
	res := toAskResult(sessionID, created, turnID, userMsgID, 0, frozen, result, gstatus, nil, out.Registry)
	fout, ferr := s.Store.FinalizeSuccessfulTurn(ctx, FinalizeSuccessInput{
		SellerID: in.SellerAccountID, SessionID: sessionID, TurnID: turnID, UserMessageID: userMsgID,
		Result: result, GroundingStatus: gstatus, ResultHash: hashJSON(result),
		ToolCalls: out.ToolCalls, Grounding: grounding,
		ClientRequestID: in.ClientRequestID, IdempotencyOwner: owner,
		PreparedExport: prepared, AskResult: res,
		Telemetry: map[string]any{
			"final_submit_call_id": out.FinalSubmitCallID,
			"provider_request_id":  out.ProviderRequestID,
			"model":                out.Model,
			"input_tokens":         out.InputTokens,
			"output_tokens":        out.OutputTokens,
			"cached_tokens":        out.CachedTokens,
			"provider_calls":       out.ProviderCalls,
		},
	})
	if ferr != nil {
		return s.recoverFailedSuccessFinalization(ctx, in, emit, owner, sessionID, created, turnID, userMsgID, frozen, prepared, out.ToolCalls, ferr)
	}
	res.AssistantMessageID = &fout.AssistantMessageID
	res.MessageID = &fout.AssistantMessageID
	if fout.Attachment != nil {
		res.Attachments = []AttachmentDTO{*fout.Attachment}
	}
	if emit != nil {
		_ = emit(StreamEvent{Type: "status", Status: "streaming"})
		for _, chunk := range chunkText(result.Answer, 24) {
			if ctx.Err() != nil {
				break
			}
			_ = emit(StreamEvent{Type: "delta", Delta: chunk})
		}
		_ = emit(StreamEvent{Type: "status", Status: "completed"})
		_ = emit(StreamEvent{Type: "done", Status: "completed", Result: res})
	}
	observability.ObserveChatTurn("COMPLETED", string(result.AnswerMode), time.Since(start).Seconds(), out.Rounds, out.InputTokens, out.OutputTokens)
	return res, nil
}

func artifactStore(s *Service) ArtifactStore {
	if s == nil || s.Exporter == nil {
		return nil
	}
	return s.Exporter.Artifacts
}

func (s *Service) recoverFailedSuccessFinalization(
	ctx context.Context,
	in AskInput,
	emit func(StreamEvent) error,
	owner string,
	sessionID int64,
	created bool,
	turnID, userMsgID int64,
	frozen FrozenTurnContext,
	prepared *PreparedExport,
	tools []ToolCallRecord,
	ferr error,
) (*AskResult, error) {
	_ = ctx
	slog.Error("chat successful turn finalization failed", "turn_id", turnID, "err", ferr)
	cleanupErr := cleanupPreparedExport(prepared, artifactStore(s))
	if cleanupErr != nil {
		slog.Error("chat export cleanup failed", "turn_id", turnID, "storage_key", preparedStorageKey(prepared), "err", cleanupErr)
	}
	if errors.Is(ferr, ErrIdempotencyOwnershipLost) {
		return nil, ferr
	}
	code := "FINALIZATION_FAILED"
	if prepared != nil {
		code = "EXPORT_FINALIZATION_FAILED"
	}
	failed := failedResult(sessionID, created, turnID, userMsgID, frozen, code)
	finCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	terminalErr := s.Store.FinalizeFailedTurn(finCtx, in.SellerAccountID, turnID, userMsgID, tools, code, in.ClientRequestID, owner, failed)
	if terminalErr != nil {
		return nil, errors.Join(
			fmt.Errorf("successful turn finalization: %w", ferr),
			fmt.Errorf("failed turn finalization: %w", terminalErr),
			cleanupErr,
		)
	}
	if emit != nil {
		_ = emit(StreamEvent{Type: "status", Status: "failed"})
		_ = emit(StreamEvent{Type: "error", Status: "failed", Error: failed.Answer})
	}
	return failed, nil
}

func preparedStorageKey(prepared *PreparedExport) string {
	if prepared == nil {
		return ""
	}
	return prepared.StorageKey
}

func (s *Service) startHeartbeat(ctx context.Context, sellerID int64, requestID, owner string, lost *atomic.Bool) func() {
	if requestID == "" {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		beat := func() {
			hbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := s.Store.HeartbeatIdempotency(hbCtx, sellerID, requestID, owner, time.Now().Add(s.leaseTTL()))
			cancel()
			if err != nil {
				lost.Store(true)
			}
		}
		beat()
		t := time.NewTicker(s.heartbeatEvery())
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if lost.Load() {
					return
				}
				beat()
				if lost.Load() {
					return
				}
			}
		}
	}()
	return func() { close(stop) }
}

func (s *Service) DownloadExport(ctx context.Context, sellerID int64, attachmentID uuid.UUID) ([]byte, ExportAttachmentRow, error) {
	if s == nil || s.Exporter == nil {
		return nil, ExportAttachmentRow{}, ErrExportFailed
	}
	return s.Exporter.Download(ctx, sellerID, attachmentID)
}

func (s *Service) ActionHandoff(ctx context.Context, in HandoffInput) (*HandoffResult, error) {
	viewID := strings.TrimSpace(in.ActionViewID)
	if viewID == "" || s.Actions == nil {
		return nil, ErrActionViewNotFound
	}
	detail, err := s.Actions.Get(ctx, in.SellerAccountID, viewID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionViewForbidden
		}
		return nil, ErrActionViewForbidden
	}
	originPub, err := s.Actions.OriginPublication(ctx, in.SellerAccountID, viewID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionViewForbidden
		}
		return nil, ErrActionViewForbidden
	}
	kind := "CASE"
	groupID, caseID, title := "", "", "Чат по действию"
	var members []ActionMemberSnapshot
	if detail.Group != nil {
		title = detail.Group.DisplayTitle
		ref, _ := actionview.Decode(detail.Group.ActionViewID)
		if ref.Kind == "g" {
			kind = "GROUP"
			groupID = ref.ID
		} else {
			caseID = ref.ID
		}
		for _, m := range detail.Group.Members {
			members = append(members, actionMemberFromView(m))
			if caseID == "" && kind == "CASE" {
				caseID = m.CaseID
			}
		}
	} else if detail.History != nil {
		title = detail.History.DisplayTitle
		caseID = detail.History.CaseID
		members = []ActionMemberSnapshot{actionMemberFromHistory(*detail.History)}
	} else {
		return nil, ErrActionViewNotFound
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.CaseID)
	}
	fp := BindingFingerprint(originPub, kind, groupID, caseID, ids)
	if existing, err := s.Store.FindBindingByFingerprint(ctx, in.SellerAccountID, fp); err == nil && existing != nil {
		return existing, nil
	}
	sess, err := s.Store.CreateSession(ctx, in.SellerAccountID, in.UserID, title)
	if err != nil {
		return nil, err
	}
	bid, err := s.Store.InsertBinding(ctx, in.SellerAccountID, sess.ID, originPub, kind, groupID, caseID, fp, title, len(members), members)
	if err != nil {
		_ = s.Store.DeleteSession(ctx, in.SellerAccountID, sess.ID)
		return nil, err
	}
	return &HandoffResult{SessionID: sess.ID, BindingID: bid, Title: title, Created: true}, nil
}

func toAskResult(sessionID int64, created bool, turnID, userMsg, assistant int64, frozen FrozenTurnContext, result TurnResult, g GroundingStatus, atts []AttachmentDTO, reg *GroundingRegistry) *AskResult {
	var aid *int64
	if assistant > 0 {
		v := assistant
		aid = &v
	}
	var pub *PubDTO
	if frozen.Publication != nil {
		pub = &PubDTO{
			ID:               fmt.Sprintf("%d", frozen.Publication.ID),
			AnalysisDate:     frozen.Publication.AnalysisDate.Format("2006-01-02"),
			AnalysisCutoffAt: frozen.Publication.CutoffAt.UTC().Format(time.RFC3339),
		}
	}
	var refs []GroundingDTO
	if reg != nil {
		for _, e := range reg.All() {
			for _, h := range result.GroundingHandles {
				if h == e.Handle {
					refs = append(refs, GroundingDTO{Handle: e.Handle, Type: string(e.Type), ID: e.RefID})
				}
			}
		}
	}
	lim := append([]string{}, frozen.Limitations...)
	lim = append(lim, result.Limitations...)
	return &AskResult{
		SessionID: sessionID, SessionCreated: created, TurnID: turnID, UserMessageID: userMsg,
		AssistantMessageID: aid, MessageID: aid, Answer: result.Answer, AnswerMode: result.AnswerMode,
		ContextMode: frozen.ContextMode, GroundingStatus: g, Publication: pub,
		ReferencedEntities: result.ReferencedEntities, GroundingRefs: refs, Limitations: uniqueStrings(lim), Attachments: atts,
	}
}

func isRequestCancelled(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}

func internalAgentErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "PROVIDER_TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		return "AGENT_TIMEOUT"
	}
	return ""
}

func setupFailureCode(ctx context.Context, err error) string {
	if code := internalAgentErrorCode(err); code != "" {
		return code
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "PROVIDER_TIMEOUT"
	}
	return "SETUP_FAILED"
}

func failedResult(sessionID int64, created bool, turnID, userMsg int64, frozen FrozenTurnContext, code string) *AskResult {
	msg := "Не удалось подготовить подтверждённый ответ. Попробуйте уточнить вопрос."
	if code == "PROVIDER_FAILED" {
		msg = "ИИ временно недоступен. Попробуйте позже."
	}
	if code == "PROVIDER_TIMEOUT" || code == "AGENT_TIMEOUT" {
		msg = "ИИ не успел подготовить ответ. Попробуйте позже."
	}
	if code == "FACTUAL_UNAVAILABLE" {
		msg = "Не могу достоверно ответить: данные для этого запроса сейчас недоступны."
	}
	if code == "EXPORT_FAILED" || code == "EXPORT_FINALIZATION_FAILED" {
		msg = "Не удалось подготовить файл выгрузки."
	}
	if code == "FINALIZATION_FAILED" {
		msg = "Не удалось завершить обработку вопроса."
	}
	if code == "INVALID_TOOL_CALL" || code == "INVALID_ARGS" || code == "FORBIDDEN_ARG" {
		msg = "Не удалось корректно обработать запрос. Попробуйте ещё раз."
	}
	res := &AskResult{SessionID: sessionID, SessionCreated: created, TurnID: turnID, UserMessageID: userMsg, Answer: msg, AnswerMode: ModeGeneralGuidance, ContextMode: frozen.ContextMode, Limitations: []string{msg}, ErrorCode: code}
	if frozen.Publication != nil {
		res.Publication = &PubDTO{
			ID:               fmt.Sprintf("%d", frozen.Publication.ID),
			AnalysisDate:     frozen.Publication.AnalysisDate.Format("2006-01-02"),
			AnalysisCutoffAt: frozen.Publication.CutoffAt.UTC().Format(time.RFC3339),
		}
	}
	return res
}

func chunkText(s string, n int) []string {
	r := []rune(s)
	if n <= 0 {
		n = 24
	}
	var out []string
	for i := 0; i < len(r); i += n {
		end := i + n
		if end > len(r) {
			end = len(r)
		}
		out = append(out, string(r[i:end]))
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func jsonUnmarshal(b []byte, v any) bool {
	if len(b) == 0 {
		return false
	}
	return json.Unmarshal(b, v) == nil
}
