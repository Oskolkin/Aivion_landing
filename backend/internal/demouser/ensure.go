package demouser

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adminbootstrap"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureRequest configures idempotent demo user + seller membership binding.
type EnsureRequest struct {
	Email           string
	Password        string
	SellerAccountID int64
	ResetPassword   bool
	// TransferSellerOwnership moves seller_accounts.user_id to the demo user
	// when the seller currently belongs to a different user (e.g. admin).
	TransferSellerOwnership bool
}

// EnsureResult summarizes ensure-demo-user outcome (no secrets).
type EnsureResult struct {
	Email              string
	UserID             int64
	SellerAccountID    int64
	CreatedUser        bool
	UpdatedPassword    bool
	TransferredSeller  bool
	PreviousOwnerEmail string
}

// EnsureDemoUser creates/updates the demo login user and binds an existing
// seller_account row to that user. The seller must already exist (seeded).
func EnsureDemoUser(ctx context.Context, pool *pgxpool.Pool, req EnsureRequest) (*EnsureResult, error) {
	email := auth.NormalizeEmail(req.Email)
	if err := auth.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		return nil, err
	}
	if req.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller_account_id must be > 0")
	}

	adminRes, err := adminbootstrap.EnsureAdminUser(ctx, pool, email, req.Password, req.ResetPassword)
	if err != nil && !errors.Is(err, adminbootstrap.ErrAlreadyExists) {
		return nil, err
	}
	if adminRes == nil {
		return nil, fmt.Errorf("ensure user returned empty result")
	}
	out := &EnsureResult{
		Email:           adminRes.Email,
		UserID:          adminRes.UserID,
		CreatedUser:     adminRes.Created,
		UpdatedPassword: adminRes.Updated,
		SellerAccountID: req.SellerAccountID,
	}

	q := dbgen.New(pool)
	seller, err := q.GetSellerAccountByID(ctx, req.SellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("seller_account_id=%d not found; seed the demo seller before ensure-demo-user", req.SellerAccountID)
		}
		return nil, fmt.Errorf("load seller: %w", err)
	}

	if seller.UserID == out.UserID {
		return out, nil
	}
	if !req.TransferSellerOwnership {
		return nil, fmt.Errorf("seller %d owned by user_id=%d; pass --transfer-seller-ownership to rebind", seller.ID, seller.UserID)
	}

	prev, pErr := q.GetUserByID(ctx, seller.UserID)
	if pErr == nil {
		out.PreviousOwnerEmail = prev.Email
	}

	tag, err := pool.Exec(ctx, `
UPDATE seller_accounts
SET user_id=$1, updated_at=now()
WHERE id=$2`, out.UserID, seller.ID)
	if err != nil {
		return nil, fmt.Errorf("transfer seller ownership: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return nil, fmt.Errorf("transfer seller ownership: expected 1 row, got %d", tag.RowsAffected())
	}
	out.TransferredSeller = true
	return out, nil
}
