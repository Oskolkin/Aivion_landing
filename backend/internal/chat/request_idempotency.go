package chat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrRequestInProgress    = errors.New("chat request is already in progress")
	ErrIdempotencyKeyReused = errors.New("idempotency key was reused with a different request")
	requestIdempotencyLease = 2 * time.Minute
)

type RequestIdempotency struct {
	q       *dbgen.Queries
	ownerID string
}

func newRequestIdempotency(repo Repository) *RequestIdempotency {
	sqlcRepo, ok := repo.(*SQLCRepository)
	if !ok || sqlcRepo == nil {
		return nil
	}
	return &RequestIdempotency{q: sqlcRepo.q}
}

// RequestHash returns a stable hash for the request fields that affect the
// generated response. Attachment order is intentionally ignored.
func RequestHash(userID *int64, sellerAccountID int64, sessionID *int64, actionKey, messageText string, attachmentIDs []string, requestType string) string {
	attachments := make([]string, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if id = strings.TrimSpace(id); id != "" {
			attachments = append(attachments, id)
		}
	}
	sort.Strings(attachments)
	normalize := func(value string) string { return strings.Join(strings.Fields(strings.TrimSpace(value)), " ") }
	user, session := "", ""
	if userID != nil {
		user = strconv.FormatInt(*userID, 10)
	}
	if sessionID != nil {
		session = strconv.FormatInt(*sessionID, 10)
	}
	payload := strings.Join([]string{
		user,
		strconv.FormatInt(sellerAccountID, 10),
		session,
		normalize(actionKey),
		normalize(messageText),
		strings.Join(attachments, ","),
		normalize(requestType),
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", sum)
}

// BeginRequest reserves a client request ID. A completed request is returned to
// the caller; a live lease is rejected rather than duplicating AI work.
func (r *RequestIdempotency) BeginRequest(ctx context.Context, sellerAccountID int64, userID *int64, clientRequestID string, actionID *int64, actionKey, requestHash, requestType string) (*AskResult, error) {
	if r == nil || strings.TrimSpace(clientRequestID) == "" {
		return nil, nil
	}
	r.ownerID = uuid.NewString()
	leaseUntil := time.Now().UTC().Add(requestIdempotencyLease)
	existing, err := r.q.UpsertChatRequestIdempotency(ctx, dbgen.UpsertChatRequestIdempotencyParams{
		SellerAccountID: sellerAccountID,
		UserID:          nullableInt64(userID),
		ClientRequestID: clientRequestID,
		ActionID:        nullableInt64(actionID),
		ActionKey:       nullableText(actionKey),
		Status:          "running",
		RequestHash:     pgtype.Text{String: requestHash, Valid: true},
		RequestType:     pgtype.Text{String: requestType, Valid: true},
		LeaseUntil:      pgtype.Timestamptz{Time: leaseUntil, Valid: true},
		OwnerID:         pgtype.Text{String: r.ownerID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("reserve chat request idempotency: %w", err)
	}
	if !sameNullableInt64(existing.UserID, userID) || !existing.RequestHash.Valid || existing.RequestHash.String != requestHash {
		return nil, ErrIdempotencyKeyReused
	}
	if existing.Status == "completed" {
		var result AskResult
		if len(existing.ResponsePayload) == 0 || json.Unmarshal(existing.ResponsePayload, &result) != nil {
			return nil, fmt.Errorf("completed chat request has invalid response payload")
		}
		return &result, nil
	}
	if existing.OwnerID.Valid && existing.OwnerID.String == r.ownerID {
		return nil, nil // This invocation inserted the row.
	}
	if existing.Status == "running" && !leaseExpired(existing.LeaseUntil, time.Now().UTC()) {
		return nil, ErrRequestInProgress
	}
	_, err = r.q.ReclaimChatRequestIdempotency(ctx, dbgen.ReclaimChatRequestIdempotencyParams{
		SellerAccountID: sellerAccountID,
		ClientRequestID: clientRequestID,
		RequestHash:     pgtype.Text{String: requestHash, Valid: true},
		LeaseUntil:      pgtype.Timestamptz{Time: leaseUntil, Valid: true},
		OwnerID:         pgtype.Text{String: r.ownerID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRequestInProgress
	}
	return nil, err
}

func (r *RequestIdempotency) CompleteRequest(ctx context.Context, sellerAccountID int64, clientRequestID string, result *AskResult) error {
	if r == nil || strings.TrimSpace(clientRequestID) == "" || result == nil {
		return nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.q.UpdateChatRequestIdempotency(ctx, dbgen.UpdateChatRequestIdempotencyParams{
		SellerAccountID: sellerAccountID, ClientRequestID: clientRequestID, Status: "completed",
		SessionID:          pgtype.Int8{Int64: result.SessionID, Valid: result.SessionID > 0},
		UserMessageID:      pgtype.Int8{Int64: result.UserMessageID, Valid: result.UserMessageID > 0},
		AssistantMessageID: nullableInt64(result.AssistantMessageID),
		ResponsePayload:    payload,
		OwnerID:            pgtype.Text{String: r.ownerID, Valid: r.ownerID != ""},
	})
	return err
}

func (r *RequestIdempotency) FailRequest(ctx context.Context, sellerAccountID int64, clientRequestID string, cause error) error {
	if r == nil || strings.TrimSpace(clientRequestID) == "" {
		return nil
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	_, err := r.q.UpdateChatRequestIdempotency(ctx, dbgen.UpdateChatRequestIdempotencyParams{
		SellerAccountID: sellerAccountID, ClientRequestID: clientRequestID, Status: "failed",
		ErrorCode:    pgtype.Text{String: "ask_failed", Valid: cause != nil},
		ErrorMessage: pgtype.Text{String: message, Valid: message != ""},
		OwnerID:      pgtype.Text{String: r.ownerID, Valid: r.ownerID != ""},
	})
	return err
}

// Heartbeat extends this request's lease. It is safe to call periodically
// while a long-running AI request is in progress.
func (r *RequestIdempotency) Heartbeat(ctx context.Context, sellerAccountID int64, clientRequestID string) error {
	if r == nil || r.ownerID == "" || strings.TrimSpace(clientRequestID) == "" {
		return nil
	}
	_, err := r.q.HeartbeatChatRequestIdempotency(ctx, dbgen.HeartbeatChatRequestIdempotencyParams{
		SellerAccountID: sellerAccountID,
		ClientRequestID: clientRequestID,
		OwnerID:         pgtype.Text{String: r.ownerID, Valid: true},
		LeaseUntil:      pgtype.Timestamptz{Time: time.Now().UTC().Add(requestIdempotencyLease), Valid: true},
	})
	return err
}

func sameNullableInt64(value pgtype.Int8, expected *int64) bool {
	return (expected == nil && !value.Valid) || (expected != nil && value.Valid && value.Int64 == *expected)
}

func leaseExpired(lease pgtype.Timestamptz, now time.Time) bool {
	return !lease.Valid || !lease.Time.After(now)
}
