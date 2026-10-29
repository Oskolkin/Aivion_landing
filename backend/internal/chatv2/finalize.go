package chatv2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type FinalizeFailpoint string

const (
	FailAfterAssistant   FinalizeFailpoint = "after_assistant"
	FailGrounding        FinalizeFailpoint = "grounding"
	FailExportMeta       FinalizeFailpoint = "export"
	FailExportProvenance FinalizeFailpoint = "export_provenance"
	FailIdempotency      FinalizeFailpoint = "idempotency"
	FailToolAudit        FinalizeFailpoint = "tool_audit"
	FailTurnComplete     FinalizeFailpoint = "turn_complete"
	FailSessionTouch     FinalizeFailpoint = "session_touch"
	FailCommit           FinalizeFailpoint = "commit"
)

type FinalizeSuccessInput struct {
	SellerID         int64
	SessionID        int64
	TurnID           int64
	UserMessageID    int64
	Result           TurnResult
	GroundingStatus  GroundingStatus
	ResultHash       string
	ToolCalls        []ToolCallRecord
	Grounding        []GroundingEntry
	ClientRequestID  string
	IdempotencyOwner string
	PreparedExport   *PreparedExport
	AskResult        *AskResult
	Telemetry        map[string]any
}

type FinalizeSuccessOutput struct {
	AssistantMessageID int64
	Attachment         *AttachmentDTO
}

func (s *Store) FinalizeSuccessfulTurn(ctx context.Context, in FinalizeSuccessInput) (FinalizeSuccessOutput, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.ClientRequestID != "" {
		if err := assertIdempotencyOwnerTx(ctx, tx, in.SellerID, in.ClientRequestID, in.IdempotencyOwner); err != nil {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.owner_fence: %w", err)
		}
	}

	var assistantID int64
	if err := tx.QueryRow(ctx, `INSERT INTO chat_messages (session_id, seller_account_id, role, content, message_type)
		VALUES ($1,$2,'assistant',$3,'answer') RETURNING id`, in.SessionID, in.SellerID, in.Result.Answer).Scan(&assistantID); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.assistant_insert: %w", err)
	}
	if s.Failpoint == FailAfterAssistant {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.assistant_insert: %w", ErrFinalizationFailed)
	}

	var att *AttachmentDTO
	if in.PreparedExport != nil {
		p := in.PreparedExport
		if s.Failpoint == FailExportMeta {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.export_attachment_insert: %w", ErrFinalizationFailed)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_attachments (
			id, seller_account_id, chat_session_id, chat_message_id, attachment_kind, export_type, format,
			file_name, mime_type, file_size, storage_key, status, sha256)
			VALUES ($1,$2,$3,$4,'export','factual_query','csv',$5,$6,$7,$8,'ready',$9)`,
			p.ID, in.SellerID, in.SessionID, assistantID, p.FileName, p.MimeType, p.Size, p.StorageKey, p.ContentHash); err != nil {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.export_attachment_insert: %w", err)
		}
		if s.Failpoint == FailExportProvenance {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.export_provenance_insert: %w", ErrFinalizationFailed)
		}
		from, to := p.PeriodFrom, p.PeriodTo
		js, _ := json.Marshal(p.Spec)
		atts, _ := json.Marshal(p.AttachmentIDs)
		if _, err := tx.Exec(ctx, `INSERT INTO chat_export_provenance (
			turn_id, seller_account_id, attachment_id, publication_id, factual_universe_id, factual_context_hash, query_spec_json,
			period_from, period_to, export_schema_version, artifact_hash, attachment_refs_json)
			VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8,$9,$10,$11,$12)`,
			in.TurnID, in.SellerID, p.ID, p.PubID, p.UniverseID, p.CtxHash, js, from, to, ExportSchemaV1, p.ContentHash, atts); err != nil {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.export_provenance_insert: %w", err)
		}
		dto := p.DTO
		att = &dto
	}

	if s.Failpoint == FailToolAudit {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.tool_audit_insert: %w", ErrFinalizationFailed)
	}
	for _, rec := range in.ToolCalls {
		callID := strings.TrimSpace(rec.CallID)
		if callID == "" {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.tool_audit_insert: %w", ErrInvalidToolCall)
		}
		handles, _ := json.Marshal(rec.Handles)
		if _, err := tx.Exec(ctx, `INSERT INTO chat_turn_tool_calls (
			call_id, turn_id, round_no, ordinal, tool_key, tool_version, args_hash, status, result_hash, fact_handles_json, error_code, completed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
			callID, in.TurnID, rec.Round, rec.Ordinal, rec.ToolKey, rec.Version, rec.ArgsHash, rec.Status, rec.ResultHash, handles, nullIfEmpty(rec.ErrorCode)); err != nil {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.tool_audit_insert: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM chat_turn_grounding_refs WHERE turn_id=$1`, in.TurnID); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.grounding_delete: %w", err)
	}
	if s.Failpoint == FailGrounding {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.grounding_insert: %w", ErrFinalizationFailed)
	}
	for i, e := range in.Grounding {
		if _, err := tx.Exec(ctx, `INSERT INTO chat_turn_grounding_refs (turn_id, handle, ref_type, ref_id, fingerprint, ordinal)
			VALUES ($1,$2,$3,$4,$5,$6)`, in.TurnID, e.Handle, e.Type, e.RefID, e.Fingerprint, i); err != nil {
			return FinalizeSuccessOutput{}, fmt.Errorf("finalize.grounding_insert: %w", err)
		}
	}

	if s.Failpoint == FailTurnComplete {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.turn_complete: %w", ErrFinalizationFailed)
	}
	refs, _ := json.Marshal(in.Result.ReferencedEntities)
	lim, _ := json.Marshal(in.Result.Limitations)
	if _, err := tx.Exec(ctx, `UPDATE chat_turns SET status='COMPLETED', assistant_message_id=$2, answer_mode=$3,
		grounding_status=$4, result_hash=$5, referenced_entities_json=$6, limitations_json=$7, completed_at=NOW(),
		telemetry_json=COALESCE($8::jsonb, telemetry_json)
		WHERE id=$1`, in.TurnID, assistantID, in.Result.AnswerMode, in.GroundingStatus, in.ResultHash, refs, lim, telemetryJSON(in.Telemetry)); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.turn_complete: %w", err)
	}

	if in.AskResult != nil {
		aid := assistantID
		in.AskResult.AssistantMessageID = &aid
		in.AskResult.MessageID = &aid
		if att != nil {
			in.AskResult.Attachments = []AttachmentDTO{*att}
		}
	}

	if s.Failpoint == FailIdempotency {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.idempotency_complete: %w", ErrFinalizationFailed)
	}
	if err := finishIdempotencyTx(ctx, tx, in.SellerID, in.ClientRequestID, in.IdempotencyOwner, "completed", in.AskResult, ""); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.idempotency_complete: %w", err)
	}
	if s.Failpoint == FailSessionTouch {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.session_touch: %w", ErrFinalizationFailed)
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_sessions SET updated_at=NOW(), last_message_at=NOW() WHERE id=$1`, in.SessionID); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.session_touch: %w", err)
	}
	if s.Failpoint == FailCommit {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.commit: %w", ErrFinalizationFailed)
	}
	if err := tx.Commit(ctx); err != nil {
		return FinalizeSuccessOutput{}, fmt.Errorf("finalize.commit: %w", err)
	}
	return FinalizeSuccessOutput{AssistantMessageID: assistantID, Attachment: att}, nil
}

func (s *Store) FinalizeCancelledTurn(ctx context.Context, sellerID, sessionID, turnID, userMsgID int64, tools []ToolCallRecord, clientRequestID, owner string, res *AskResult) error {
	return s.finalizeTerminal(ctx, sellerID, turnID, userMsgID, tools, StatusCancelled, "CANCELLED", clientRequestID, owner, "cancelled", res)
}

func (s *Store) FinalizeFailedTurn(ctx context.Context, sellerID, turnID, userMsgID int64, tools []ToolCallRecord, code, clientRequestID, owner string, res *AskResult) error {
	return s.finalizeTerminal(ctx, sellerID, turnID, userMsgID, tools, StatusFailed, code, clientRequestID, owner, "failed", res)
}

func (s *Store) finalizeTerminal(ctx context.Context, sellerID, turnID, userMsgID int64, tools []ToolCallRecord, status TurnStatus, code, clientRequestID, owner, idemStatus string, res *AskResult) error {
	if turnID == 0 && clientRequestID == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if clientRequestID != "" && owner != "" {
		if err := assertIdempotencyOwnerTx(ctx, tx, sellerID, clientRequestID, owner); err != nil {
			return err
		}
	}
	if turnID > 0 {
		for _, rec := range tools {
			callID := strings.TrimSpace(rec.CallID)
			if callID == "" {
				callID = fmt.Sprintf("anon-%d-%d", rec.Round, rec.Ordinal)
			}
			handles, _ := json.Marshal(rec.Handles)
			if _, err := tx.Exec(ctx, `INSERT INTO chat_turn_tool_calls (
				call_id, turn_id, round_no, ordinal, tool_key, tool_version, args_hash, status, result_hash, fact_handles_json, error_code, completed_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
				callID, turnID, rec.Round, rec.Ordinal, rec.ToolKey, rec.Version, rec.ArgsHash, rec.Status, rec.ResultHash, handles, nullIfEmpty(rec.ErrorCode)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE chat_turns SET status=$2, error_code=NULLIF($3,''), completed_at=NOW() WHERE id=$1`, turnID, status, code); err != nil {
			return err
		}
	}
	if err := finishIdempotencyTx(ctx, tx, sellerID, clientRequestID, owner, idemStatus, res, code); err != nil {
		return err
	}
	_ = userMsgID
	return tx.Commit(ctx)
}

func assertIdempotencyOwnerTx(ctx context.Context, tx pgx.Tx, sellerID int64, requestID, owner string) error {
	if requestID == "" {
		return nil
	}
	var got *string
	err := tx.QueryRow(ctx, `SELECT owner_id FROM chat_request_idempotency WHERE seller_account_id=$1 AND client_request_id=$2 AND status='running'`,
		sellerID, requestID).Scan(&got)
	if err != nil {
		return ErrIdempotencyOwnershipLost
	}
	if got == nil || *got != owner {
		return ErrIdempotencyOwnershipLost
	}
	return nil
}

func finishIdempotencyTx(ctx context.Context, tx pgx.Tx, sellerID int64, clientRequestID, owner, status string, result *AskResult, errCode string) error {
	if clientRequestID == "" {
		return nil
	}
	var payload []byte
	if result != nil {
		payload, _ = json.Marshal(result)
	}
	var sid, umid, amid, tid any
	if result != nil {
		sid = nullIfZero(result.SessionID)
		umid = nullIfZero(result.UserMessageID)
		if result.AssistantMessageID != nil {
			amid = nullIfZero(*result.AssistantMessageID)
		}
		tid = nullIfZero(result.TurnID)
	}
	tag, err := tx.Exec(ctx, `UPDATE chat_request_idempotency SET status=$3, session_id=COALESCE($4, session_id),
		user_message_id=COALESCE($5, user_message_id), assistant_message_id=COALESCE($6, assistant_message_id),
		turn_id=COALESCE($7, turn_id), response_payload=COALESCE($8, response_payload), error_code=$9,
		lease_until=NULL, owner_id=NULL, updated_at=NOW()
		WHERE seller_account_id=$1 AND client_request_id=$2 AND status='running' AND owner_id=$10`,
		sellerID, clientRequestID, status, sid, umid, amid, tid, payload, nullIfEmpty(errCode), owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdempotencyOwnershipLost
	}
	return nil
}

func (s *Store) HeartbeatIdempotency(ctx context.Context, sellerID int64, requestID, owner string, nextLease time.Time) error {
	if requestID == "" {
		return nil
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chat_request_idempotency
		SET heartbeat_at=NOW(), lease_until=$4, updated_at=NOW()
		WHERE seller_account_id=$1 AND client_request_id=$2 AND owner_id=$3 AND status='running'`,
		sellerID, requestID, owner, nextLease)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdempotencyOwnershipLost
	}
	return nil
}

func (s *Store) ReplayCompletedAsk(ctx context.Context, sellerID int64, clientRequestID string) (*AskResult, bool, error) {
	row, err := s.getIdempotency(ctx, sellerID, clientRequestID)
	if err != nil {
		return nil, false, err
	}
	if row.Status != "completed" {
		return nil, false, nil
	}
	var existing AskResult
	if jsonUnmarshal(row.ResponsePayload, &existing) && existing.TurnID != 0 {
		return &existing, true, nil
	}
	if row.TurnID == nil {
		return nil, false, nil
	}
	res, err := s.reconstructAsk(*row.TurnID)
	if err != nil {
		return nil, false, err
	}
	return res, true, nil
}

func (s *Store) getIdempotency(ctx context.Context, sellerID int64, clientRequestID string) (IdempotencyRow, error) {
	var row IdempotencyRow
	err := s.pool.QueryRow(ctx, `SELECT status, COALESCE(request_hash,''), session_id, user_message_id, assistant_message_id, turn_id, response_payload, lease_until, owner_id
		FROM chat_request_idempotency WHERE seller_account_id=$1 AND client_request_id=$2`, sellerID, clientRequestID).
		Scan(&row.Status, &row.RequestHash, &row.SessionID, &row.UserMessageID, &row.AssistantMessageID, &row.TurnID, &row.ResponsePayload, &row.LeaseUntil, &row.OwnerID)
	return row, err
}

func (s *Store) reconstructAsk(turnID int64) (*AskResult, error) {
	t, err := s.GetTurn(context.Background(), turnID)
	if err != nil {
		return nil, err
	}
	var answer string
	if t.AssistantMessageID != nil {
		_ = s.pool.QueryRow(context.Background(), `SELECT content FROM chat_messages WHERE id=$1`, *t.AssistantMessageID).Scan(&answer)
	}
	res := &AskResult{
		SessionID: t.SessionID, TurnID: t.ID, UserMessageID: t.UserMessageID,
		AssistantMessageID: t.AssistantMessageID, MessageID: t.AssistantMessageID,
		Answer: answer, AnswerMode: t.AnswerMode, ContextMode: t.ContextMode, GroundingStatus: t.GroundingStatus,
	}
	return res, nil
}

func (s *Store) BindIdempotencyProgress(ctx context.Context, sellerID int64, requestID, owner string, sessionID, userMsgID, turnID int64) error {
	if requestID == "" {
		return nil
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chat_request_idempotency SET session_id=$4, user_message_id=$5, turn_id=$6, updated_at=NOW()
		WHERE seller_account_id=$1 AND client_request_id=$2 AND owner_id=$3 AND status='running'`,
		sellerID, requestID, owner, sessionID, userMsgID, nullIfZero(turnID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdempotencyOwnershipLost
	}
	return nil
}

func telemetryJSON(m map[string]any) any {
	if len(m) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func nullIfZero(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
