package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ChatHandler struct {
	service        *chat.Service
	actionsService *actions.Service
	v2             *chatv2.Service
}

func NewChatHandler(service *chat.Service, actionsService *actions.Service) *ChatHandler {
	return &ChatHandler{service: service, actionsService: actionsService}
}

func (h *ChatHandler) WithV2(svc *chatv2.Service) *ChatHandler {
	h.v2 = svc
	return h
}

type askChatRequest struct {
	SessionID        *int64   `json:"session_id"`
	Question         string   `json:"question"`
	AttachmentIDs    []string `json:"attachment_ids,omitempty"`
	ClientRequestID  string   `json:"client_request_id,omitempty"`
	ContextBindingID *int64   `json:"context_binding_id,omitempty"`
	ActionViewID     string   `json:"action_view_id,omitempty"`
	// Legacy fields are ignored by Chat v2.
	ActionKey      *string `json:"action_key,omitempty"`
	CreationSource string  `json:"creation_source,omitempty"`
	AsOfDate       *string `json:"as_of_date,omitempty"`
}

type askChatResponse struct {
	SessionID          int64                    `json:"session_id"`
	SessionCreated     bool                     `json:"session_created"`
	TurnID             int64                    `json:"turn_id,omitempty"`
	UserMessageID      int64                    `json:"user_message_id"`
	AssistantMessageID *int64                   `json:"assistant_message_id"`
	MessageID          *int64                   `json:"message_id,omitempty"`
	Answer             string                   `json:"answer"`
	AnswerMode         string                   `json:"answer_mode,omitempty"`
	ContextMode        string                   `json:"context_mode,omitempty"`
	Publication        *chatv2.PubDTO           `json:"publication"`
	ReferencedEntities []chatv2.EntityRef       `json:"referenced_entities,omitempty"`
	GroundingRefs      []chatv2.GroundingDTO    `json:"grounding_refs,omitempty"`
	Limitations        []string                 `json:"limitations"`
	Attachments        []chatAttachmentResponse `json:"attachments,omitempty"`
	Cancelled          bool                     `json:"cancelled,omitempty"`
}

type chatAttachmentResponse struct {
	ID           string `json:"id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	ExportType   string `json:"export_type"`
	Format       string `json:"format"`
	DownloadURL  string `json:"download_url"`
	CreatedAt    string `json:"created_at"`
	Completeness string `json:"completeness,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

type supportingFactResponse struct {
	Source string `json:"source"`
	ID     *int64 `json:"id,omitempty"`
	Fact   string `json:"fact"`
}

type chatSessionResponse struct {
	ID            int64                  `json:"id"`
	Title         string                 `json:"title"`
	ActionKey     *string                `json:"action_key,omitempty"`
	ActionViewID  string                 `json:"action_view_id,omitempty"`
	BindingID     *int64                 `json:"binding_id,omitempty"`
	ActionID      *int64                 `json:"action_id,omitempty"`
	Status        chat.ChatSessionStatus `json:"status"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	LastMessageAt *string                `json:"last_message_at"`
}

type chatMessageResponse struct {
	ID          int64                    `json:"id"`
	SessionID   int64                    `json:"session_id"`
	Role        chat.ChatMessageRole     `json:"role"`
	MessageType chat.ChatMessageType     `json:"message_type"`
	Content     string                   `json:"content"`
	CreatedAt   string                   `json:"created_at"`
	Attachments []chatAttachmentResponse `json:"attachments,omitempty"`
}

type chatFeedbackRequest struct {
	SessionID int64   `json:"session_id"`
	Rating    string  `json:"rating"`
	Comment   *string `json:"comment,omitempty"`
}

type chatFeedbackResponse struct {
	ID        int64                   `json:"id"`
	SessionID int64                   `json:"session_id"`
	MessageID int64                   `json:"message_id"`
	Rating    chat.ChatFeedbackRating `json:"rating"`
	Comment   *string                 `json:"comment"`
	CreatedAt string                  `json:"created_at"`
}

func (h *ChatHandler) Ask(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, userOk := auth.UserFromContext(r.Context())
	var userID *int64
	if userOk {
		userID = &user.ID
	}

	var req askChatRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSONError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.SessionID != nil && *req.SessionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "session_id must be > 0")
		return
	}
	if h.v2 != nil {
		h.askV2(w, r, sellerAccount.ID, userID, req, question)
		return
	}
	writeJSONError(w, http.StatusInternalServerError, "chat v2 is not configured")
}

func (h *ChatHandler) AskStream(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	user, userOK := auth.UserFromContext(r.Context())
	var userID *int64
	if userOK {
		userID = &user.ID
	}
	var req askChatRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSONError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.SessionID != nil && *req.SessionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "session_id must be > 0")
		return
	}
	if h.v2 == nil {
		writeJSONError(w, http.StatusInternalServerError, "chat v2 is not configured")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	terminalSent := false
	emit := func(event chatv2.StreamEvent) error {
		if event.Type == "done" || event.Type == "error" {
			terminalSent = true
		}
		payload, err := json.Marshal(map[string]any{
			"type": event.Type, "status": event.Status, "delta": event.Delta, "error": event.Error, "result": mapV2Ask(event.Result),
		})
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	_, err := h.v2.AskStream(r.Context(), v2AskInput(sellerAccount.ID, userID, req, question), emit)
	if err == nil {
		return
	}
	if r.Context().Err() != nil {
		return
	}
	if terminalSent {
		return
	}
	slog.Error("chat v2 stream failed without terminal event", "err", err)
	_ = emit(chatv2.StreamEvent{
		Type:   "error",
		Status: "failed",
		Error:  "Не удалось завершить обработку вопроса.",
	})
}

func (h *ChatHandler) ActionHandoff(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.v2 == nil {
		writeJSONError(w, http.StatusInternalServerError, "chat v2 is not configured")
		return
	}
	user, userOK := auth.UserFromContext(r.Context())
	var userID *int64
	if userOK {
		userID = &user.ID
	}
	var req struct {
		ActionViewID    string `json:"action_view_id"`
		ClientRequestID string `json:"client_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	res, err := h.v2.ActionHandoff(r.Context(), chatv2.HandoffInput{
		SellerAccountID: sellerAccount.ID, UserID: userID, ActionViewID: strings.TrimSpace(req.ActionViewID), ClientRequestID: strings.TrimSpace(req.ClientRequestID),
	})
	if err != nil {
		if errors.Is(err, chatv2.ErrActionViewForbidden) || errors.Is(err, chatv2.ErrActionViewNotFound) {
			writeJSONError(w, http.StatusNotFound, "action not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to create action chat")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *ChatHandler) askV2(w http.ResponseWriter, r *http.Request, sellerID int64, userID *int64, req askChatRequest, question string) {
	res, err := h.v2.Ask(r.Context(), v2AskInput(sellerID, userID, req, question))
	if err != nil {
		if errors.Is(err, chatv2.ErrQuestionRequired) {
			writeJSONError(w, http.StatusBadRequest, "question is required")
			return
		}
		if errors.Is(err, chatv2.ErrIdempotencyConflict) {
			writeJSONError(w, http.StatusConflict, "idempotency key was reused with a different request")
			return
		}
		if errors.Is(err, chatv2.ErrRequestInProgress) {
			writeJSONError(w, http.StatusConflict, "chat request is already in progress")
			return
		}
		if errors.Is(err, chatv2.ErrAttachmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "attachment not found")
			return
		}
		if errors.Is(err, chatv2.ErrCancelled) {
			writeJSON(w, http.StatusOK, mapV2Ask(res))
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to process chat ask")
		return
	}
	writeJSON(w, http.StatusOK, mapV2Ask(res))
}

func v2AskInput(sellerID int64, userID *int64, req askChatRequest, question string) chatv2.AskInput {
	return chatv2.AskInput{
		SellerAccountID: sellerID, UserID: userID, SessionID: req.SessionID, Question: question,
		AttachmentIDs: req.AttachmentIDs, ClientRequestID: strings.TrimSpace(req.ClientRequestID), BindingID: req.ContextBindingID,
	}
}

func mapV2Ask(res *chatv2.AskResult) *askChatResponse {
	if res == nil {
		return nil
	}
	atts := make([]chatAttachmentResponse, 0, len(res.Attachments))
	for _, a := range res.Attachments {
		atts = append(atts, chatAttachmentResponse{
			ID: a.ID, FileName: a.FileName, MimeType: a.MimeType, FileSize: a.FileSize, ExportType: a.ExportType,
			Format: a.Format, DownloadURL: a.DownloadURL, CreatedAt: a.CreatedAt, Completeness: a.Completeness, Warning: a.Warning,
		})
	}
	return &askChatResponse{
		SessionID: res.SessionID, SessionCreated: res.SessionCreated, TurnID: res.TurnID, UserMessageID: res.UserMessageID,
		AssistantMessageID: res.AssistantMessageID, MessageID: res.MessageID, Answer: res.Answer, AnswerMode: string(res.AnswerMode),
		ContextMode: string(res.ContextMode), Publication: res.Publication, ReferencedEntities: res.ReferencedEntities,
		GroundingRefs: res.GroundingRefs, Limitations: res.Limitations, Attachments: atts, Cancelled: res.Cancelled,
	}
}

func (h *ChatHandler) GetSessionByAction(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actionKey := strings.TrimSpace(r.URL.Query().Get("action_key"))
	if actionKey == "" {
		writeJSONError(w, http.StatusBadRequest, "action_key is required")
		return
	}
	if err := actions.ValidateActionKey(actionKey); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.actionsService != nil {
		if _, err := h.actionsService.GetAction(r.Context(), sellerAccount.ID, actionKey); err != nil {
			if errors.Is(err, actions.ErrActionNotFound) {
				writeJSONError(w, http.StatusNotFound, "action not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to load action")
			return
		}
	}
	h.respondSessionByActionKey(w, r.Context(), sellerAccount.ID, actionKey, nil)
}

func (h *ChatHandler) GetSessionByActionID(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actionID, err := parseChatIDParam(r, "action_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid action id")
		return
	}
	if h.actionsService == nil {
		writeJSONError(w, http.StatusInternalServerError, "actions service unavailable")
		return
	}
	action, err := h.actionsService.GetActionByID(r.Context(), sellerAccount.ID, actionID)
	if err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSONError(w, http.StatusNotFound, "action not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load action")
		return
	}
	h.respondSessionByActionKey(w, r.Context(), sellerAccount.ID, action.ActionKey, &actionID)
}

type lookupSessionByActionRequest struct {
	ActionKey string `json:"action_key"`
}

func (h *ChatHandler) LookupSessionByAction(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req lookupSessionByActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	actionKey := strings.TrimSpace(req.ActionKey)
	if actionKey == "" {
		writeJSONError(w, http.StatusBadRequest, "action_key is required")
		return
	}
	if err := actions.ValidateActionKey(actionKey); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.actionsService != nil {
		if _, err := h.actionsService.GetAction(r.Context(), sellerAccount.ID, actionKey); err != nil {
			if errors.Is(err, actions.ErrActionNotFound) {
				writeJSONError(w, http.StatusNotFound, "action not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to load action")
			return
		}
	}
	h.respondSessionByActionKey(w, r.Context(), sellerAccount.ID, actionKey, nil)
}

func (h *ChatHandler) respondSessionByActionKey(
	w http.ResponseWriter,
	ctx context.Context,
	sellerAccountID int64,
	actionKey string,
	actionID *int64,
) {
	session, err := h.service.GetSessionByActionKey(ctx, sellerAccountID, actionKey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to lookup chat session by action")
		return
	}
	if session == nil {
		writeJSON(w, http.StatusOK, map[string]any{"found": false})
		return
	}
	resp := mapChatSession(*session)
	if actionID != nil {
		resp.ActionID = actionID
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"found":   true,
		"session": resp,
	})
}

func (h *ChatHandler) RenameSession(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	session, err := h.service.RenameSession(r.Context(), sellerAccount.ID, sessionID, req.Title)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "title") {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to rename chat session")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "chat session not found")
		return
	}
	writeJSON(w, http.StatusOK, mapChatSessionBound(*session, h.bindingFor(r.Context(), sellerAccount.ID, session.ID)))
}

func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := h.service.SoftDeleteSession(r.Context(), sellerAccount.ID, sessionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete chat session")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "chat session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ChatHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, userOk := auth.UserFromContext(r.Context())
	var userID *int64
	if userOk {
		userID = &user.ID
	}
	if err := r.ParseMultipartForm(chatAttachmentMaxFileBytes() + 1024*1024); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, chatAttachmentMaxFileBytes()+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	attachment, err := h.service.UploadInputAttachment(r.Context(), chat.UploadInputAttachmentInput{
		SellerAccountID: sellerAccount.ID,
		UserID:          userID,
		FileName:        header.Filename,
		MimeType:        mimeType,
		Data:            data,
	})
	if err != nil {
		if mapChatAttachmentError(w, err) {
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to upload attachment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attachment": attachment})
}

func (h *ChatHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	attachmentID, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attachment id")
		return
	}
	if err := h.service.DeleteInputAttachment(r.Context(), sellerAccount.ID, attachmentID); err != nil {
		if mapChatAttachmentError(w, err) {
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func chatAttachmentMaxFileBytes() int64 {
	return int64(getEnvIntHandler("CHAT_ATTACHMENT_MAX_FILE_BYTES", 20*1024*1024))
}

func getEnvIntHandler(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func mapChatAttachmentError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, chat.ErrAttachmentUnsupportedFormat):
		writeJSONError(w, http.StatusBadRequest, "Формат файла не поддерживается.")
		return true
	case errors.Is(err, chat.ErrAttachmentTooLarge):
		writeJSONError(w, http.StatusBadRequest, "Файл превышает допустимый размер 20 МБ.")
		return true
	case errors.Is(err, chat.ErrAttachmentTooMany):
		writeJSONError(w, http.StatusBadRequest, "Можно прикрепить не более 5 файлов.")
		return true
	case errors.Is(err, chat.ErrAttachmentTotalTooLarge):
		writeJSONError(w, http.StatusBadRequest, "Суммарный размер файлов превышает 30 МБ.")
		return true
	case errors.Is(err, chat.ErrAttachmentNotFound):
		writeJSONError(w, http.StatusNotFound, "attachment not found")
		return true
	case errors.Is(err, chat.ErrAttachmentInvalidState):
		writeJSONError(w, http.StatusBadRequest, "attachment is not available")
		return true
	default:
		return false
	}
}

func (h *ChatHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	attachmentID, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attachment id")
		return
	}
	if h.v2 != nil {
		v2data, row, v2err := h.v2.DownloadExport(r.Context(), sellerAccount.ID, attachmentID)
		switch {
		case v2err == nil:
			if len(v2data) == 0 {
				writeJSONError(w, http.StatusInternalServerError, "failed to download export")
				return
			}
			w.Header().Set("Content-Type", row.MimeType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", contentDispositionAttachment(row.FileName))
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(v2data)), 10))
			_, _ = w.Write(v2data)
			return
		case errors.Is(v2err, chatv2.ErrExportForbidden):
			// not a v2 export for this seller — legacy/input fallback below
		default:
			slog.Error("chat v2 export download failed", "attachment_id", attachmentID.String(), "err", v2err)
			writeJSONError(w, http.StatusInternalServerError, "failed to download export")
			return
		}
	}
	if h.service == nil {
		writeJSONError(w, http.StatusNotFound, "attachment not found")
		return
	}
	att, data, err := h.service.DownloadAttachment(r.Context(), sellerAccount.ID, attachmentID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to download attachment")
		return
	}
	if att == nil {
		writeJSONError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if len(data) == 0 {
		writeJSONError(w, http.StatusNotFound, "attachment file not found")
		return
	}
	w.Header().Set("Content-Type", att.MimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", contentDispositionAttachment(att.FileName))
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	_, _ = w.Write(data)
}

func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit, offset := parseChatPagination(r, 20, 100)
	items, err := h.service.ListSessions(r.Context(), sellerAccount.ID, int32(limit), int32(offset))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat sessions")
		return
	}
	resp := make([]chatSessionResponse, 0, len(items))
	idx := h.sessionBindingIndex(r.Context(), sellerAccount.ID)
	for _, item := range items {
		resp = append(resp, mapChatSessionBound(item, idx[item.ID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  resp,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *ChatHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := h.service.GetSession(r.Context(), sellerAccount.ID, sessionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get chat session")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "chat session not found")
		return
	}
	writeJSON(w, http.StatusOK, mapChatSessionBound(*session, h.bindingFor(r.Context(), sellerAccount.ID, session.ID)))
}

func (h *ChatHandler) ListSessionMessages(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := h.service.GetSession(r.Context(), sellerAccount.ID, sessionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get chat session")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "chat session not found")
		return
	}
	limit, offset := parseChatPagination(r, 50, 200)
	items, err := h.service.ListMessages(r.Context(), sellerAccount.ID, sessionID, int32(limit), int32(offset))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list chat messages")
		return
	}
	messageIDs := make([]int64, 0, len(items))
	for _, item := range items {
		messageIDs = append(messageIDs, item.ID)
	}
	attachmentsByMessage, _ := h.service.ListAttachmentsByMessageIDs(r.Context(), sellerAccount.ID, messageIDs)

	resp := make([]chatMessageResponse, 0, len(items))
	for _, item := range items {
		m := mapChatMessage(item)
		if atts, ok := attachmentsByMessage[item.ID]; ok {
			m.Attachments = mapChatAttachments(atts)
		}
		resp = append(resp, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  resp,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *ChatHandler) ArchiveSession(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	session, err := h.service.ArchiveSession(r.Context(), sellerAccount.ID, sessionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to archive chat session")
		return
	}
	if session == nil {
		writeJSONError(w, http.StatusNotFound, "chat session not found")
		return
	}
	writeJSON(w, http.StatusOK, mapChatSessionBound(*session, h.bindingFor(r.Context(), sellerAccount.ID, session.ID)))
}

func (h *ChatHandler) AddMessageFeedback(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	messageID, err := parseChatIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid message id")
		return
	}
	var req chatFeedbackRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	if req.SessionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "session_id must be > 0")
		return
	}
	rating := chat.ChatFeedbackRating(strings.TrimSpace(req.Rating))
	switch rating {
	case chat.ChatFeedbackRatingPositive, chat.ChatFeedbackRatingNegative, chat.ChatFeedbackRatingNeutral:
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid rating")
		return
	}

	item, err := h.service.AddFeedback(r.Context(), chat.AddFeedbackInput{
		SellerAccountID: sellerAccount.ID,
		SessionID:       req.SessionID,
		MessageID:       messageID,
		Rating:          rating,
		Comment:         req.Comment,
	})
	if err != nil {
		errLower := strings.ToLower(err.Error())
		switch {
		case strings.Contains(errLower, "message not found"):
			writeJSONError(w, http.StatusNotFound, "chat message not found")
		case strings.Contains(errLower, "feedback is allowed only for assistant answer messages"),
			strings.Contains(errLower, "message does not belong to session"),
			strings.Contains(errLower, "invalid feedback rating"):
			writeJSONError(w, http.StatusBadRequest, err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to add chat feedback")
		}
		return
	}
	writeJSON(w, http.StatusOK, chatFeedbackResponse{
		ID:        item.ID,
		SessionID: item.SessionID,
		MessageID: item.MessageID,
		Rating:    item.Rating,
		Comment:   item.Comment,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func parseChatIDParam(r *http.Request, name string) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func parseChatPagination(r *http.Request, defaultLimit int, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > maxLimit {
				parsed = maxLimit
			}
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func mapChatAttachments(items []chat.ChatAnswerAttachment) []chatAttachmentResponse {
	out := make([]chatAttachmentResponse, 0, len(items))
	for _, item := range items {
		out = append(out, chatAttachmentResponse{
			ID:          item.ID,
			FileName:    item.FileName,
			MimeType:    item.MimeType,
			FileSize:    item.FileSize,
			ExportType:  item.ExportType,
			Format:      item.Format,
			DownloadURL: item.DownloadURL,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func mapSupportingFacts(items []chat.SupportingFact) []supportingFactResponse {
	out := make([]supportingFactResponse, 0, len(items))
	for _, item := range items {
		out = append(out, supportingFactResponse{
			Source: item.Source,
			ID:     item.ID,
			Fact:   item.Fact,
		})
	}
	return out
}

func mapChatSession(item chat.ChatSession) chatSessionResponse {
	return mapChatSessionBound(item, chatv2.SessionBindingMeta{})
}

func mapChatSessionBound(item chat.ChatSession, meta chatv2.SessionBindingMeta) chatSessionResponse {
	resp := chatSessionResponse{
		ID:            item.ID,
		Title:         item.Title,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     item.UpdatedAt.UTC().Format(time.RFC3339),
		LastMessageAt: timePtrRFC3339(item.LastMessageAt),
	}
	if meta.BindingID > 0 {
		id := meta.BindingID
		resp.BindingID = &id
		resp.ActionViewID = meta.ActionViewID
	}
	return resp
}

func (h *ChatHandler) sessionBindingIndex(ctx context.Context, sellerID int64) map[int64]chatv2.SessionBindingMeta {
	if h.v2 == nil || h.v2.Store == nil {
		return map[int64]chatv2.SessionBindingMeta{}
	}
	idx, err := h.v2.Store.ListSessionBindings(ctx, sellerID)
	if err != nil || idx == nil {
		return map[int64]chatv2.SessionBindingMeta{}
	}
	return idx
}

func (h *ChatHandler) bindingFor(ctx context.Context, sellerID, sessionID int64) chatv2.SessionBindingMeta {
	return h.sessionBindingIndex(ctx, sellerID)[sessionID]
}

func contentDispositionAttachment(fileName string) string {
	safe := strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, filepath.Base(strings.TrimSpace(fileName)))
	if safe == "" || safe == "." {
		safe = "attachment"
	}
	ascii := safe
	if ascii != fileName {
		return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, url.PathEscape(safe))
	}
	return mime.FormatMediaType("attachment", map[string]string{"filename": safe})
}

func mapChatMessage(item chat.ChatMessage) chatMessageResponse {
	return chatMessageResponse{
		ID:          item.ID,
		SessionID:   item.SessionID,
		Role:        item.Role,
		MessageType: item.MessageType,
		Content:     item.Content,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
	}
}
