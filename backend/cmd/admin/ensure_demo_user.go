package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adminbootstrap"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/demouser"
)

func runEnsureDemoUser(args []string) int {
	fs := flag.NewFlagSet("ensure-demo-user", flag.ContinueOnError)
	email := fs.String("email", "demo@example.com", "demo user email")
	sellerID := fs.Int64("seller-account-id", 1, "existing seller_account.id to bind")
	resetPassword := fs.Bool("reset-password", false, "update password if user already exists")
	transfer := fs.Bool("transfer-seller-ownership", false, "rebind seller_accounts.user_id to the demo user")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	password, err := adminbootstrap.ReadPasswordInteractively()
	if err != nil {
		fmt.Fprintf(os.Stderr, "password error: %v\n", err)
		return 1
	}

	normalized := auth.NormalizeEmail(*email)
	if emailInAdminList(normalized, cfg.Admin.Emails) {
		fmt.Fprintln(os.Stderr, "refusing: demo email is listed in ADMIN_EMAILS")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		return 1
	}
	defer postgres.Pool.Close()

	result, err := demouser.EnsureDemoUser(ctx, postgres.Pool, demouser.EnsureRequest{
		Email:                   normalized,
		Password:                password,
		SellerAccountID:         *sellerID,
		ResetPassword:           *resetPassword,
		TransferSellerOwnership: *transfer,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ensure-demo-user failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "demo user ok: email=%s user_id=%d seller_account_id=%d created=%s password_updated=%s transferred=%s\n",
		result.Email,
		result.UserID,
		result.SellerAccountID,
		strconv.FormatBool(result.CreatedUser),
		strconv.FormatBool(result.UpdatedPassword),
		strconv.FormatBool(result.TransferredSeller),
	)
	if result.PreviousOwnerEmail != "" {
		fmt.Fprintf(os.Stdout, "previous_owner_email=%s (now admin-only if listed in ADMIN_EMAILS)\n", result.PreviousOwnerEmail)
	}
	return 0
}
