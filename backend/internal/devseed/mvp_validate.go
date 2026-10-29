package devseed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ParseMVPAnchorDate resolves --anchor-date: empty or "today" uses ref's UTC calendar date.
func ParseMVPAnchorDate(raw string, ref time.Time) (time.Time, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" || s == "today" {
		t := ref.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	d, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("anchor-date: use YYYY-MM-DD, \"today\", or leave empty: %w", err)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), nil
}

// ValidateMVPSeedOptions checks CLI-derived options before touching the database.
func ValidateMVPSeedOptions(opts MVPSeedOptions) error {
	validateModes := 0
	if opts.ValidateOnly {
		validateModes++
	}
	if opts.ValidateAlertGeneration {
		validateModes++
	}
	if opts.ValidateRecommendationGeneration {
		validateModes++
	}
	if opts.ValidateDerived {
		validateModes++
	}
	if validateModes > 1 {
		return fmt.Errorf("choose at most one of --validate-only, --validate-alert-generation, --validate-recommendation-generation, --validate-derived")
	}
	if opts.ValidateAlertGeneration {
		if opts.SellerAccountID <= 0 {
			return fmt.Errorf("validate-alert-generation requires --seller-account-id > 0")
		}
		return nil
	}
	if opts.ValidateRecommendationGeneration {
		if opts.SellerAccountID <= 0 {
			return fmt.Errorf("validate-recommendation-generation requires --seller-account-id > 0")
		}
		return nil
	}
	if opts.ValidateDerived {
		if opts.SellerAccountID <= 0 {
			return fmt.Errorf("validate-derived requires --seller-account-id > 0")
		}
		return nil
	}
	if opts.SellerAccountID <= 0 {
		if err := auth.ValidateEmail(opts.Email); err != nil {
			return err
		}
	}
	if opts.Days <= 0 {
		return fmt.Errorf("days must be > 0")
	}
	if opts.ProductsTarget <= 0 {
		return fmt.Errorf("products must be > 0")
	}
	if opts.AnchorDate.IsZero() {
		return fmt.Errorf("anchor date must be set")
	}
	if err := auth.ValidatePassword(opts.Password); err != nil {
		return err
	}
	if opts.WithAdminUser {
		if err := auth.ValidateEmail(opts.AdminEmail); err != nil {
			return fmt.Errorf("admin email: %w", err)
		}
	}
	if opts.ValidateOnly {
		return nil
	}
	if len(opts.EncryptionKey) != 32 {
		return fmt.Errorf("encryption key must be exactly 32 bytes (set ENCRYPTION_KEY to match the API server)")
	}
	return nil
}

// ValidateMVPDatabase checks connectivity and, when opts.SellerAccountID > 0, that the seller account exists.
func ValidateMVPDatabase(ctx context.Context, pool *pgxpool.Pool, opts MVPSeedOptions) error {
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	if opts.SellerAccountID <= 0 {
		return nil
	}
	q := dbgen.New(pool)
	if _, err := q.GetSellerAccountByID(ctx, opts.SellerAccountID); err != nil {
		return fmt.Errorf("seller account %d: %w", opts.SellerAccountID, err)
	}
	return nil
}
