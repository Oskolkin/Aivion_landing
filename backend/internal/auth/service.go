package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrSellerAccountRequired = errors.New("seller account required")
)

type Service struct {
	db          *pgxpool.Pool
	queries     *dbgen.Queries
	sessionTTL  time.Duration
	adminEmails []string
}

func NewService(db *pgxpool.Pool, sessionTTL time.Duration, adminEmails []string) *Service {
	return &Service{
		db:          db,
		queries:     dbgen.New(db),
		sessionTTL:  sessionTTL,
		adminEmails: adminEmails,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	email := NormalizeEmail(input.Email)

	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return nil, err
	}
	if input.Password != input.PasswordConfirm {
		return nil, errors.New("passwords do not match")
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	user, err := qtx.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		Status:       "active",
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	sellerAccount, err := qtx.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: user.ID,
		Name:   "My account",
		Status: "onboarding",
	})
	if err != nil {
		return nil, fmt.Errorf("create seller account: %w", err)
	}

	rawToken, tokenHash, err := GenerateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	_, err = qtx.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:      user.ID,
		TokenHash:   tokenHash,
		ReadOnly:    false,
		SessionKind: "user",
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(s.sessionTTL),
			Valid: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &AuthResult{
		User:          user,
		SellerAccount: &sellerAccount,
		SessionToken:  rawToken,
	}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	email := NormalizeEmail(input.Email)

	if err := ValidateEmail(email); err != nil {
		return nil, err
	}

	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	if err := ComparePassword(user.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.Status != "active" {
		return nil, ErrUnauthorized
	}

	sellerAccount, err := s.loadSellerAccount(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if sellerAccount == nil && !IsAdminUser(&user, s.adminEmails) {
		return nil, ErrSellerAccountRequired
	}

	rawToken, tokenHash, err := GenerateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	_, err = s.queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:      user.ID,
		TokenHash:   tokenHash,
		ReadOnly:    false,
		SessionKind: "user",
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(s.sessionTTL),
			Valid: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &AuthResult{
		User:          user,
		SellerAccount: sellerAccount,
		SessionToken:  rawToken,
	}, nil
}

func (s *Service) GetCurrentUser(ctx context.Context, rawSessionToken string) (*AuthResult, error) {
	if rawSessionToken == "" {
		return nil, ErrUnauthorized
	}

	tokenHash := HashSessionToken(rawSessionToken)

	session, err := s.queries.GetActiveSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("get active session: %w", err)
	}

	// If this is a review session, ensure the invite is still active.
	if session.SessionKind == "review" && session.ReviewInviteID.Valid {
		invite, inviteErr := s.queries.GetReviewInviteByID(ctx, session.ReviewInviteID.Int64)
		if inviteErr != nil || invite.RevokedAt.Valid || (invite.ExpiresAt.Valid && !invite.ExpiresAt.Time.After(time.Now())) {
			_ = s.queries.RevokeSessionByTokenHash(ctx, tokenHash)
			return nil, ErrUnauthorized
		}
	}

	user, err := s.queries.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	if user.Status != "active" {
		return nil, ErrUnauthorized
	}

	sellerAccount, err := s.loadSellerAccount(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	result := &AuthResult{
		User:          user,
		SellerAccount: sellerAccount,
		SessionToken:  "",
		ReadOnly:      session.ReadOnly || session.SessionKind == "review",
		IsReviewMode:  session.SessionKind == "review",
	}
	if session.ExpiresAt.Valid {
		result.SessionExpires = session.ExpiresAt.Time
	}
	return result, nil
}

func (s *Service) loadSellerAccount(ctx context.Context, userID int64) (*dbgen.SellerAccount, error) {
	sellerAccount, err := s.queries.GetSellerAccountByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get seller account: %w", err)
	}
	return &sellerAccount, nil
}

func (s *Service) Logout(ctx context.Context, rawSessionToken string) error {
	if rawSessionToken == "" {
		return ErrUnauthorized
	}

	tokenHash := HashSessionToken(rawSessionToken)

	if err := s.queries.RevokeSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func (s *Service) DeleteExpiredSessions(ctx context.Context) error {
	if err := s.queries.DeleteExpiredSessions(ctx); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
