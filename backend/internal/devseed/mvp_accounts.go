package devseed

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// mvpUserPasswordWritePlan applies when the user row already exists: whether to overwrite password_hash
// and whether the seed summary should report a password update for that account.
func mvpUserPasswordWritePlan(resetPassword bool) (writeNewBcryptHash bool, reportPasswordUpdated bool) {
	if resetPassword {
		return true, true
	}
	return false, false
}

// ResolveSellerForMVP returns seller_account_id and demo user email for the result summary.
func ResolveSellerForMVP(ctx context.Context, q *dbgen.Queries, opts MVPSeedOptions) (sellerID int64, demoEmail string, demoPasswordUpdated bool, err error) {
	if opts.SellerAccountID > 0 {
		sa, err := q.GetSellerAccountByID(ctx, opts.SellerAccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, "", false, fmt.Errorf("seller account %d not found", opts.SellerAccountID)
			}
			return 0, "", false, fmt.Errorf("get seller account: %w", err)
		}
		u, err := q.GetUserByID(ctx, sa.UserID)
		if err != nil {
			return 0, "", false, fmt.Errorf("get seller user: %w", err)
		}
		if opts.ResetPassword {
			if err := updateUserPasswordHash(ctx, q, u.ID, opts.Password); err != nil {
				return 0, "", false, err
			}
			demoPasswordUpdated = true
		}
		return sa.ID, u.Email, demoPasswordUpdated, nil
	}

	email := auth.NormalizeEmail(opts.Email)
	user, updated, err := ensureUserWithPassword(ctx, q, email, opts.Password, opts.ResetPassword)
	if err != nil {
		return 0, "", false, err
	}

	sa, err := ensureSellerAccount(ctx, q, user.ID, opts.SellerName)
	if err != nil {
		return 0, "", false, err
	}
	return sa.ID, user.Email, updated, nil
}

// EnsureAdminUserIfRequested creates the admin login user when enabled (same password as demo for local use).
func EnsureAdminUserIfRequested(ctx context.Context, q *dbgen.Queries, opts MVPSeedOptions) (adminEmail string, adminPasswordUpdated bool, err error) {
	if !opts.WithAdminUser {
		return "", false, nil
	}
	email := auth.NormalizeEmail(opts.AdminEmail)
	_, updated, err := ensureUserWithPassword(ctx, q, email, opts.Password, opts.ResetPassword)
	if err != nil {
		return "", false, err
	}
	return email, updated, nil
}

func ensureUserWithPassword(ctx context.Context, q *dbgen.Queries, email, password string, resetPassword bool) (dbgen.User, bool, error) {
	u, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		write, report := mvpUserPasswordWritePlan(resetPassword)
		if !write {
			return u, report, nil
		}
		if err := updateUserPasswordHash(ctx, q, u.ID, password); err != nil {
			return dbgen.User{}, false, err
		}
		u2, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			return dbgen.User{}, false, fmt.Errorf("reload user %s: %w", email, err)
		}
		return u2, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dbgen.User{}, false, fmt.Errorf("get user %s: %w", email, err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return dbgen.User{}, false, fmt.Errorf("hash password: %w", err)
	}

	created, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Status:       "active",
	})
	if err != nil {
		return dbgen.User{}, false, fmt.Errorf("create user %s: %w", email, err)
	}
	return created, true, nil
}

func updateUserPasswordHash(ctx context.Context, q *dbgen.Queries, userID int64, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := q.UpdateUserPasswordHashByUserID(ctx, dbgen.UpdateUserPasswordHashByUserIDParams{
		PasswordHash: hash,
		ID:           userID,
	}); err != nil {
		return fmt.Errorf("update user %d password: %w", userID, err)
	}
	return nil
}

func ensureSellerAccount(ctx context.Context, q *dbgen.Queries, userID int64, sellerName string) (dbgen.SellerAccount, error) {
	sa, err := q.GetSellerAccountByUserID(ctx, userID)
	if err == nil {
		return sa, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dbgen.SellerAccount{}, fmt.Errorf("get seller account for user %d: %w", userID, err)
	}

	name := strings.TrimSpace(sellerName)
	if name == "" {
		name = "Seller"
	}

	created, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: userID,
		Name:   name,
		Status: "active",
	})
	if err != nil {
		return dbgen.SellerAccount{}, fmt.Errorf("create seller account: %w", err)
	}
	return created, nil
}

// SeedSellerBillingSupportStub upserts internal billing placeholder (not a full billing implementation).
func SeedSellerBillingSupportStub(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, productsCount int) error {
	skuLimit := int64(500)
	if productsCount > 0 {
		skuLimit = int64(productsCount)*3 + 200
	}
	notes := fmt.Sprintf("sku_limit=%d (demo, >catalog). Demo support state. Full Billing stage is intentionally not implemented.", skuLimit)
	_, err := q.UpsertSellerBillingState(ctx, dbgen.UpsertSellerBillingStateParams{
		SellerAccountID:      sellerAccountID,
		PlanCode:             "internal_demo",
		Status:               "internal",
		TrialEndsAt:          pgtype.Timestamptz{},
		CurrentPeriodStart:   pgtype.Timestamptz{},
		CurrentPeriodEnd:     pgtype.Timestamptz{},
		AiTokensLimitMonth:   pgtype.Int8{Int64: 2_500_000, Valid: true},
		AiTokensUsedMonth:    12_400,
		EstimatedAiCostMonth: mustNumeric("0.042"),
		Notes:                pgtype.Text{String: notes, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("upsert seller billing state: %w", err)
	}
	return nil
}

func mustNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(s)
	return n
}
