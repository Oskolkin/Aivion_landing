package adminbootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAlreadyExists = errors.New("admin user already exists")
)

type EnsureResult struct {
	Email   string
	UserID  int64
	Created bool
	Updated bool
}

// EnsureAdminUser creates an active admin login user idempotently.
// If the email already exists and resetPassword is false, it returns ErrAlreadyExists
// without modifying the password.
// If resetPassword is true, it updates the password hash for the existing user.
func EnsureAdminUser(ctx context.Context, pool *pgxpool.Pool, email, password string, resetPassword bool) (*EnsureResult, error) {
	email = auth.NormalizeEmail(email)
	if err := auth.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return nil, err
	}

	q := dbgen.New(pool)
	existing, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		if !resetPassword {
			return &EnsureResult{
				Email:   existing.Email,
				UserID:  existing.ID,
				Created: false,
				Updated: false,
			}, ErrAlreadyExists
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		if err := q.UpdateUserPasswordHashByUserID(ctx, dbgen.UpdateUserPasswordHashByUserIDParams{
			PasswordHash: hash,
			ID:           existing.ID,
		}); err != nil {
			return nil, fmt.Errorf("update password: %w", err)
		}
		return &EnsureResult{
			Email:   existing.Email,
			UserID:  existing.ID,
			Created: false,
			Updated: true,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	created, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Status:       "active",
	})
	if err != nil {
		if isUniqueViolation(err) {
			u, getErr := q.GetUserByEmail(ctx, email)
			if getErr != nil {
				return nil, ErrAlreadyExists
			}
			return &EnsureResult{Email: u.Email, UserID: u.ID}, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &EnsureResult{
		Email:   created.Email,
		UserID:  created.ID,
		Created: true,
	}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
