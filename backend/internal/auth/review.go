package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrReviewInviteInvalid = errors.New("review invite invalid")
	ErrReviewInviteExpired = errors.New("review invite expired")
	ErrReviewInviteRevoked = errors.New("review invite revoked")
)

const (
	SessionKindUser   = "user"
	SessionKindReview = "review"
	MaxReviewTTL      = 24 * time.Hour
)

type CreateReviewInviteInput struct {
	SellerAccountID int64
	UserID          int64
	TTL             time.Duration
	Label           string
}

type CreateReviewInviteResult struct {
	InviteID  int64
	RawToken  string
	ExpiresAt time.Time
}

func GenerateReviewInviteToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, HashSessionToken(raw), nil
}

func NormalizeReviewToken(raw string) (string, error) {
	token := strings.TrimSpace(strings.ToLower(raw))
	if len(token) != 64 {
		return "", ErrReviewInviteInvalid
	}
	if _, err := hex.DecodeString(token); err != nil {
		return "", ErrReviewInviteInvalid
	}
	return token, nil
}

func (s *Service) CreateReviewInvite(ctx context.Context, input CreateReviewInviteInput) (*CreateReviewInviteResult, error) {
	if input.SellerAccountID <= 0 || input.UserID <= 0 {
		return nil, errors.New("seller_account_id and user_id are required")
	}
	ttl := input.TTL
	if ttl <= 0 || ttl > MaxReviewTTL {
		ttl = MaxReviewTTL
	}
	raw, hash, err := GenerateReviewInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate review token: %w", err)
	}
	expires := time.Now().UTC().Add(ttl)
	var label pgtype.Text
	if strings.TrimSpace(input.Label) != "" {
		label = pgtype.Text{String: strings.TrimSpace(input.Label), Valid: true}
	}
	invite, err := s.queries.CreateReviewInvite(ctx, dbgen.CreateReviewInviteParams{
		TokenHash:       hash,
		SellerAccountID: input.SellerAccountID,
		UserID:          input.UserID,
		ExpiresAt:       pgtype.Timestamptz{Time: expires, Valid: true},
		Label:           label,
	})
	if err != nil {
		return nil, fmt.Errorf("create review invite: %w", err)
	}
	return &CreateReviewInviteResult{
		InviteID:  invite.ID,
		RawToken:  raw,
		ExpiresAt: expires,
	}, nil
}

// ResolveReviewInvite validates an invite token and returns a read-only auth result
// without creating a DB session or session cookie token.
func (s *Service) ResolveReviewInvite(ctx context.Context, rawToken string) (*AuthResult, error) {
	token, err := NormalizeReviewToken(rawToken)
	if err != nil {
		return nil, err
	}
	invite, err := s.queries.GetActiveReviewInviteByTokenHash(ctx, HashSessionToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewInviteInvalid
		}
		return nil, fmt.Errorf("lookup review invite: %w", err)
	}
	if invite.RevokedAt.Valid {
		return nil, ErrReviewInviteRevoked
	}
	if !invite.ExpiresAt.Valid || !invite.ExpiresAt.Time.After(time.Now()) {
		return nil, ErrReviewInviteExpired
	}

	user, err := s.queries.GetUserByID(ctx, invite.UserID)
	if err != nil {
		return nil, fmt.Errorf("get review user: %w", err)
	}
	if user.Status != "active" {
		return nil, ErrUnauthorized
	}

	sellerAccount, err := s.loadSellerAccount(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if sellerAccount == nil || sellerAccount.ID != invite.SellerAccountID {
		return nil, ErrReviewInviteInvalid
	}

	return &AuthResult{
		User:           user,
		SellerAccount:  sellerAccount,
		ReadOnly:       true,
		IsReviewMode:   true,
		SessionExpires: invite.ExpiresAt.Time.UTC(),
		ReviewInviteID: invite.ID,
	}, nil
}

func (s *Service) RedeemReviewInvite(ctx context.Context, rawToken string) (*AuthResult, error) {
	result, err := s.ResolveReviewInvite(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	rawSession, sessionHash, err := GenerateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate review session: %w", err)
	}

	sessionTTL := time.Until(result.SessionExpires)
	if sessionTTL > MaxReviewTTL {
		sessionTTL = MaxReviewTTL
	}
	if sessionTTL < time.Minute {
		return nil, ErrReviewInviteExpired
	}
	expiresAt := time.Now().UTC().Add(sessionTTL)

	_, err = s.queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:         result.User.ID,
		TokenHash:      sessionHash,
		ReadOnly:       true,
		SessionKind:    SessionKindReview,
		ReviewInviteID: pgtype.Int8{Int64: result.ReviewInviteID, Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create review session: %w", err)
	}

	result.SessionToken = rawSession
	result.SessionExpires = expiresAt
	return result, nil
}

func (s *Service) RevokeReviewInvite(ctx context.Context, inviteID int64) error {
	if inviteID <= 0 {
		return errors.New("invite id required")
	}
	if _, err := s.queries.RevokeReviewInvite(ctx, inviteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReviewInviteInvalid
		}
		return fmt.Errorf("revoke review invite: %w", err)
	}
	if err := s.queries.RevokeSessionsByReviewInviteID(ctx, pgtype.Int8{Int64: inviteID, Valid: true}); err != nil {
		return fmt.Errorf("revoke review sessions: %w", err)
	}
	return nil
}

func (s *Service) ListActiveReviewInvites(ctx context.Context) ([]dbgen.ReviewInvite, error) {
	return s.queries.ListActiveReviewInvites(ctx)
}
