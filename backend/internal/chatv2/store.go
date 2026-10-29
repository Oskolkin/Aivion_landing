package chatv2

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool      *pgxpool.Pool
	Failpoint FinalizeFailpoint
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type SessionRow struct {
	ID              int64
	SellerAccountID int64
	Title           string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastMessageAt   *time.Time
}

func (s *Store) CreateSession(ctx context.Context, sellerID int64, userID *int64, title string) (SessionRow, error) {
	var row SessionRow
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_sessions (seller_account_id, user_id, title, status)
		VALUES ($1,$2,$3,'active') RETURNING id, seller_account_id, title, status, created_at, updated_at, last_message_at`,
		sellerID, userID, title).Scan(&row.ID, &row.SellerAccountID, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.LastMessageAt)
	return row, err
}

func (s *Store) GetSession(ctx context.Context, sellerID, sessionID int64) (SessionRow, error) {
	var row SessionRow
	err := s.pool.QueryRow(ctx, `SELECT id, seller_account_id, title, status, created_at, updated_at, last_message_at
		FROM chat_sessions WHERE id=$1 AND seller_account_id=$2 AND deleted_at IS NULL`, sessionID, sellerID).
		Scan(&row.ID, &row.SellerAccountID, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.LastMessageAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrSessionNotFound
	}
	return row, err
}

func (s *Store) TouchSession(ctx context.Context, sessionID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE chat_sessions SET updated_at=NOW(), last_message_at=NOW() WHERE id=$1`, sessionID)
	return err
}

func (s *Store) CreateMessage(ctx context.Context, sessionID, sellerID int64, role, content, msgType string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_messages (session_id, seller_account_id, role, content, message_type)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, sessionID, sellerID, role, content, msgType).Scan(&id)
	return id, err
}

func (s *Store) ListRecentMessages(ctx context.Context, sellerID, sessionID int64, limit int) ([]HistoryMessage, error) {
	if limit <= 0 {
		limit = 400
	}
	rows, err := s.pool.Query(ctx, `SELECT id, role, content, created_at FROM chat_messages
		WHERE seller_account_id=$1 AND session_id=$2 ORDER BY created_at DESC, id DESC LIMIT $3`, sellerID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryMessage
	for rows.Next() {
		var m HistoryMessage
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Created); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (s *Store) InsertTurn(ctx context.Context, t TurnRow) (int64, error) {
	var id int64
	var uni any
	if t.UniverseID != nil && strings.TrimSpace(*t.UniverseID) != "" {
		uni = *t.UniverseID
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_turns (
		session_id, seller_account_id, user_message_id, assistant_message_id, answer_mode, context_mode, status,
		publication_id, factual_universe_id, factual_context_hash, model_policy_version, tool_policy_version,
		memory_policy_version, prompt_version, result_schema_version, grounding_status, error_code)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::uuid,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		t.SessionID, t.SellerID, t.UserMessageID, t.AssistantMessageID, t.AnswerMode, t.ContextMode, t.Status,
		t.PublicationID, uni, t.ResultHash, t.ModelPolicyVersion, t.ToolPolicyVersion,
		t.MemoryPolicyVersion, t.PromptVersion, t.ResultSchemaVersion, t.GroundingStatus, t.ErrorCode).Scan(&id)
	return id, err
}

func (s *Store) UpdateTurnStatus(ctx context.Context, turnID int64, status TurnStatus, errCode string) error {
	_, err := s.pool.Exec(ctx, `UPDATE chat_turns SET status=$2, error_code=NULLIF($3,''),
		completed_at=CASE WHEN $2 IN ('COMPLETED','FAILED','CANCELLED') THEN NOW() ELSE completed_at END
		WHERE id=$1`, turnID, status, errCode)
	return err
}

func (s *Store) CompleteTurn(ctx context.Context, turnID int64, assistantID int64, result TurnResult, gstatus GroundingStatus, resultHash string) error {
	refs, _ := json.Marshal(result.ReferencedEntities)
	lim, _ := json.Marshal(result.Limitations)
	_, err := s.pool.Exec(ctx, `UPDATE chat_turns SET status='COMPLETED', assistant_message_id=$2, answer_mode=$3,
		grounding_status=$4, result_hash=$5, referenced_entities_json=$6, limitations_json=$7, completed_at=NOW()
		WHERE id=$1`, turnID, assistantID, result.AnswerMode, gstatus, resultHash, refs, lim)
	return err
}

func (s *Store) CompleteTurnWithAudit(ctx context.Context, turnID, assistantID int64, result TurnResult, gstatus GroundingStatus, resultHash string, toolCalls []ToolCallRecord, grounding []GroundingEntry) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, rec := range toolCalls {
		handles, _ := json.Marshal(rec.Handles)
		if _, err := tx.Exec(ctx, `INSERT INTO chat_turn_tool_calls (
			call_id, turn_id, round_no, ordinal, tool_key, tool_version, args_hash, status, result_hash, fact_handles_json, error_code, completed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
			rec.CallID, turnID, rec.Round, rec.Ordinal, rec.ToolKey, rec.Version, rec.ArgsHash, rec.Status, rec.ResultHash, handles, nullIfEmpty(rec.ErrorCode)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_turn_grounding_refs WHERE turn_id=$1`, turnID); err != nil {
		return err
	}
	for i, e := range grounding {
		if _, err := tx.Exec(ctx, `INSERT INTO chat_turn_grounding_refs (turn_id, handle, ref_type, ref_id, fingerprint, ordinal)
			VALUES ($1,$2,$3,$4,$5,$6)`, turnID, e.Handle, e.Type, e.RefID, e.Fingerprint, i); err != nil {
			return err
		}
	}
	refs, _ := json.Marshal(result.ReferencedEntities)
	lim, _ := json.Marshal(result.Limitations)
	if _, err := tx.Exec(ctx, `UPDATE chat_turns SET status='COMPLETED', assistant_message_id=$2, answer_mode=$3,
		grounding_status=$4, result_hash=$5, referenced_entities_json=$6, limitations_json=$7, completed_at=NOW()
		WHERE id=$1`, turnID, assistantID, result.AnswerMode, gstatus, resultHash, refs, lim); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) PersistToolCalls(ctx context.Context, turnID int64, toolCalls []ToolCallRecord) error {
	for _, rec := range toolCalls {
		if err := s.InsertToolCall(ctx, turnID, rec); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteExportAttachment(ctx context.Context, sellerID int64, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_attachments WHERE id=$1 AND seller_account_id=$2`, id, sellerID)
	return err
}

func (s *Store) GetMessageRole(ctx context.Context, messageID int64) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `SELECT role FROM chat_messages WHERE id=$1`, messageID).Scan(&role)
	return role, err
}

func (s *Store) InsertTurnContext(ctx context.Context, turnID int64, frozen FrozenTurnContext) error {
	var date any
	var cutoff any
	if frozen.Publication != nil {
		date = frozen.Publication.AnalysisDate
		cutoff = frozen.Publication.CutoffAt
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_turn_contexts (
		turn_id, analysis_date, analysis_cutoff_at, publication_changed_from_id,
		base_context_hash, conversation_context_hash, attachment_context_hash, action_context_hash,
		context_truncated, truncation_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		turnID, date, cutoff, frozen.PreviousPublicationID,
		frozen.Hashes.Base, frozen.Hashes.Conversation, frozen.Hashes.Attachment, frozen.Hashes.Action,
		frozen.Truncated, nullIfEmpty(frozen.TruncationReason))
	return err
}

func (s *Store) InsertToolCall(ctx context.Context, turnID int64, rec ToolCallRecord) error {
	handles, _ := json.Marshal(rec.Handles)
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_turn_tool_calls (
		call_id, turn_id, round_no, ordinal, tool_key, tool_version, args_hash, status, result_hash, fact_handles_json, error_code, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
		rec.CallID, turnID, rec.Round, rec.Ordinal, rec.ToolKey, rec.Version, rec.ArgsHash, rec.Status, rec.ResultHash, handles, nullIfEmpty(rec.ErrorCode))
	return err
}

func (s *Store) ReplaceGrounding(ctx context.Context, turnID int64, entries []GroundingEntry) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM chat_turn_grounding_refs WHERE turn_id=$1`, turnID); err != nil {
		return err
	}
	for i, e := range entries {
		if _, err := s.pool.Exec(ctx, `INSERT INTO chat_turn_grounding_refs (turn_id, handle, ref_type, ref_id, fingerprint, ordinal)
			VALUES ($1,$2,$3,$4,$5,$6)`, turnID, e.Handle, e.Type, e.RefID, e.Fingerprint, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetTurn(ctx context.Context, turnID int64) (TurnRow, error) {
	var t TurnRow
	err := s.pool.QueryRow(ctx, `SELECT id, session_id, seller_account_id, user_message_id, assistant_message_id,
		answer_mode, context_mode, status, publication_id, factual_universe_id::text, model_policy_version,
		tool_policy_version, memory_policy_version, prompt_version, result_schema_version, grounding_status,
		COALESCE(result_hash,''), COALESCE(error_code,'')
		FROM chat_turns WHERE id=$1`, turnID).Scan(
		&t.ID, &t.SessionID, &t.SellerID, &t.UserMessageID, &t.AssistantMessageID,
		&t.AnswerMode, &t.ContextMode, &t.Status, &t.PublicationID, &t.UniverseID, &t.ModelPolicyVersion,
		&t.ToolPolicyVersion, &t.MemoryPolicyVersion, &t.PromptVersion, &t.ResultSchemaVersion, &t.GroundingStatus,
		&t.ResultHash, &t.ErrorCode)
	return t, err
}

type IdempotencyRow struct {
	Status             string
	RequestHash        string
	SessionID          *int64
	UserMessageID      *int64
	AssistantMessageID *int64
	TurnID             *int64
	ResponsePayload    []byte
	LeaseUntil         *time.Time
	OwnerID            *string
}

func (s *Store) BeginIdempotency(ctx context.Context, sellerID int64, userID *int64, clientRequestID, reqHash, owner string, lease time.Time) (IdempotencyRow, bool, error) {
	var row IdempotencyRow
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_request_idempotency (
		seller_account_id, user_id, client_request_id, status, request_hash, request_type, started_at, heartbeat_at, lease_until, owner_id)
		VALUES ($1,$2,$3,'running',$4,'ask',NOW(),NOW(),$5,$6)
		ON CONFLICT (seller_account_id, client_request_id) DO NOTHING
		RETURNING status, COALESCE(request_hash,''), session_id, user_message_id, assistant_message_id, turn_id, response_payload, lease_until, owner_id`,
		sellerID, userID, clientRequestID, reqHash, lease, owner).Scan(
		&row.Status, &row.RequestHash, &row.SessionID, &row.UserMessageID, &row.AssistantMessageID, &row.TurnID, &row.ResponsePayload, &row.LeaseUntil, &row.OwnerID)
	if err == nil {
		return row, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return row, false, err
	}
	row, err = s.getIdempotency(ctx, sellerID, clientRequestID)
	if err != nil {
		return row, false, err
	}
	if row.RequestHash != "" && row.RequestHash != reqHash {
		return row, false, ErrIdempotencyConflict
	}
	if row.Status == "completed" || row.Status == "failed" || row.Status == "cancelled" {
		return row, true, nil
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chat_request_idempotency
		SET status='running', request_hash=$3, owner_id=$4, lease_until=$5, heartbeat_at=NOW(), updated_at=NOW(), error_code=NULL
		WHERE seller_account_id=$1 AND client_request_id=$2 AND request_hash=$3
		  AND status = 'running'
		  AND (owner_id IS NULL OR owner_id=$4 OR lease_until IS NULL OR lease_until < NOW())`,
		sellerID, clientRequestID, reqHash, owner, lease)
	if err != nil {
		return row, false, err
	}
	if tag.RowsAffected() != 1 {
		if row.Status == "running" {
			return row, false, ErrRequestInProgress
		}
		return row, false, ErrIdempotencyOwnershipLost
	}
	row.Status = "running"
	row.OwnerID = &owner
	row.LeaseUntil = &lease
	return row, false, nil
}

func (s *Store) FinishIdempotency(ctx context.Context, sellerID int64, clientRequestID, owner, status string, result *AskResult, errCode string) error {
	var payload []byte
	if result != nil {
		payload, _ = json.Marshal(result)
	}
	var sid, umid, amid, tid any
	if result != nil {
		sid = result.SessionID
		umid = result.UserMessageID
		amid = result.AssistantMessageID
		tid = result.TurnID
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chat_request_idempotency SET status=$3, session_id=COALESCE($4, session_id),
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

func (s *Store) LoadAttachments(ctx context.Context, sellerID int64, ids []string) ([]AttachmentContext, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	uuids := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(strings.TrimSpace(id))
		if err != nil {
			return nil, ErrAttachmentNotFound
		}
		uuids = append(uuids, u)
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, file_name, mime_type, COALESCE(extraction_status,''), COALESCE(extracted_text,''),
		COALESCE(completeness_status,''), COALESCE(warning_code,''), COALESCE(extraction_error,''), COALESCE(character_count,0)
		FROM chat_attachments WHERE seller_account_id=$1 AND id = ANY($2)`, sellerID, uuids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]AttachmentContext{}
	for rows.Next() {
		var a AttachmentContext
		var extraction, completeness, warn, extrErr string
		var chars int32
		if err := rows.Scan(&a.ID, &a.FileName, &a.MimeType, &extraction, &a.Excerpt, &completeness, &warn, &extrErr, &chars); err != nil {
			return nil, err
		}
		a.Completeness = deriveCompleteness(extraction, completeness, a.MimeType, a.Excerpt, extrErr)
		a.Warning = attachmentWarning(a.Completeness, warn, extrErr)
		found[a.ID] = a
	}
	out := make([]AttachmentContext, 0, len(ids))
	for _, id := range ids {
		a, ok := found[strings.TrimSpace(id)]
		if !ok {
			return nil, ErrAttachmentNotFound
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetBindingBySession(ctx context.Context, sessionID int64) (*ActionTurnContext, error) {
	var a ActionTurnContext
	err := s.pool.QueryRow(ctx, `SELECT id, title, origin_kind, origin_publication_id, COALESCE(origin_group_id::text,''), COALESCE(origin_case_id::text,''), origin_member_count
		FROM chat_action_bindings WHERE session_id=$1`, sessionID).
		Scan(&a.BindingID, &a.Title, &a.OriginKind, &a.OriginPublicationID, &a.OriginGroupID, &a.OriginCaseID, &a.OriginMemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT case_id::text, member_order, target_kind, target_id, action_type FROM chat_action_binding_members WHERE binding_id=$1 ORDER BY member_order`, a.BindingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m ActionMemberSnapshot
		var order int
		if err := rows.Scan(&m.CaseID, &order, &m.TargetKind, &m.TargetID, &m.ActionType); err != nil {
			return nil, err
		}
		a.Members = append(a.Members, m)
	}
	return &a, rows.Err()
}

func (s *Store) FindBindingByFingerprint(ctx context.Context, sellerID int64, fingerprint string) (*HandoffResult, error) {
	var r HandoffResult
	err := s.pool.QueryRow(ctx, `SELECT id, session_id, title FROM chat_action_bindings WHERE seller_account_id=$1 AND fingerprint=$2`, sellerID, fingerprint).
		Scan(&r.BindingID, &r.SessionID, &r.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

func (s *Store) InsertBinding(ctx context.Context, sellerID, sessionID, originPub int64, kind, groupID, caseID, fingerprint, title string, originCount int, members []ActionMemberSnapshot) (int64, error) {
	var gid, cid any
	if groupID != "" {
		gid = groupID
	}
	if caseID != "" {
		cid = caseID
	}
	var id int64
	refs := make([]string, 0)
	seen := map[string]struct{}{}
	for _, m := range members {
		r := strings.TrimSpace(m.RecommendationRef)
		if r == "" {
			r = strings.TrimSpace(m.OriginRecommendationRef)
		}
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		refs = append(refs, r)
	}
	refJSON, _ := json.Marshal(refs)
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_action_bindings (
		seller_account_id, session_id, origin_publication_id, origin_kind, origin_group_id, origin_case_id, origin_member_count, fingerprint, title, origin_recommendation_refs)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		sellerID, sessionID, originPub, kind, gid, cid, originCount, fingerprint, title, refJSON).Scan(&id)
	if err != nil {
		return 0, err
	}
	for i, m := range members {
		if _, err := s.pool.Exec(ctx, `INSERT INTO chat_action_binding_members (binding_id, case_id, member_order, target_kind, target_id, action_type)
			VALUES ($1,$2,$3,$4,$5,$6)`, id, m.CaseID, i, m.TargetKind, m.TargetID, m.ActionType); err != nil {
			return 0, err
		}
	}
	return id, nil
}

type ExportAttachmentRow struct {
	ID          uuid.UUID
	SellerID    int64
	SessionID   int64
	FileName    string
	MimeType    string
	Size        int64
	StorageKey  string
	ContentHash string
}

func (s *Store) InsertExportAttachment(ctx context.Context, id uuid.UUID, sellerID, sessionID, messageID int64, name, mime string, size int64, key, sha string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_attachments (
		id, seller_account_id, chat_session_id, chat_message_id, attachment_kind, export_type, format,
		file_name, mime_type, file_size, storage_key, status, sha256)
		VALUES ($1,$2,$3,$4,'export','factual_query','csv',$5,$6,$7,$8,'ready',$9)`,
		id, sellerID, sessionID, messageID, name, mime, size, key, sha)
	return err
}

func (s *Store) GetExportAttachment(ctx context.Context, sellerID int64, id uuid.UUID) (ExportAttachmentRow, error) {
	var row ExportAttachmentRow
	err := s.pool.QueryRow(ctx, `SELECT id, seller_account_id, COALESCE(chat_session_id,0), file_name, mime_type, file_size, storage_key, COALESCE(sha256,'')
		FROM chat_attachments WHERE id=$1 AND seller_account_id=$2 AND COALESCE(attachment_kind,'export')='export'`,
		id, sellerID).Scan(&row.ID, &row.SellerID, &row.SessionID, &row.FileName, &row.MimeType, &row.Size, &row.StorageKey, &row.ContentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrExportForbidden
	}
	return row, err
}

func (s *Store) InsertExportProvenance(ctx context.Context, sellerID int64, turnID int64, attachmentID *uuid.UUID, pubID *int64, universeID, ctxHash string, spec any, from, to *time.Time, schema, artifactHash string, attIDs []string) error {
	js, _ := json.Marshal(spec)
	atts, _ := json.Marshal(attIDs)
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_export_provenance (
		turn_id, seller_account_id, attachment_id, publication_id, factual_universe_id, factual_context_hash, query_spec_json,
		period_from, period_to, export_schema_version, artifact_hash, attachment_refs_json)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8,$9,$10,$11,$12)`,
		turnID, sellerID, attachmentID, pubID, universeID, ctxHash, js, from, to, schema, artifactHash, atts)
	return err
}

func (s *Store) LoadLastReferencedEntities(ctx context.Context, sessionID int64) ([]EntityRef, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT referenced_entities_json FROM chat_turns
		WHERE session_id=$1 AND status='COMPLETED' ORDER BY id DESC LIMIT 1`, sessionID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var refs []EntityRef
	_ = json.Unmarshal(raw, &refs)
	return refs, nil
}

func (s *Store) DeleteSession(ctx context.Context, sellerID, sessionID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_sessions WHERE id=$1 AND seller_account_id=$2`, sessionID, sellerID)
	return err
}

func (s *Store) CountMessages(ctx context.Context, sessionID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE session_id=$1`, sessionID).Scan(&n)
	return n, err
}

func (s *Store) CountAssistantMessages(ctx context.Context, sessionID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE session_id=$1 AND role='assistant'`, sessionID).Scan(&n)
	return n, err
}

func (s *Store) CountToolCalls(ctx context.Context, turnID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM chat_turn_tool_calls WHERE turn_id=$1`, turnID).Scan(&n)
	return n, err
}

type SessionBindingMeta struct {
	SessionID    int64
	BindingID    int64
	ActionViewID string
}

func (s *Store) ListSessionBindings(ctx context.Context, sellerID int64) (map[int64]SessionBindingMeta, error) {
	rows, err := s.pool.Query(ctx, `SELECT session_id, id, origin_kind, COALESCE(origin_group_id::text,''), COALESCE(origin_case_id::text,'')
		FROM chat_action_bindings WHERE seller_account_id=$1`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]SessionBindingMeta{}
	for rows.Next() {
		var m SessionBindingMeta
		var kind, gid, cid string
		if err := rows.Scan(&m.SessionID, &m.BindingID, &kind, &gid, &cid); err != nil {
			return nil, err
		}
		if kind == "GROUP" {
			m.ActionViewID = actionview.EncodeGroup(gid)
		} else {
			m.ActionViewID = actionview.EncodeCase(cid)
		}
		out[m.SessionID] = m
	}
	return out, rows.Err()
}

func (s *Store) LastTurnPublication(ctx context.Context, sessionID int64) (*int64, error) {
	var id *int64
	err := s.pool.QueryRow(ctx, `SELECT publication_id FROM chat_turns WHERE session_id=$1 AND publication_id IS NOT NULL ORDER BY id DESC LIMIT 1`, sessionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return id, err
}

func (s *Store) CountTurns(ctx context.Context, sessionID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM chat_turns WHERE session_id=$1`, sessionID).Scan(&n)
	return n, err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
