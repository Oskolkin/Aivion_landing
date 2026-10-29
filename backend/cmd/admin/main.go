package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adminbootstrap"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "create":
		os.Exit(runCreate(os.Args[2:]))
	case "ensure-demo-user":
		os.Exit(runEnsureDemoUser(os.Args[2:]))
	case "review-invite":
		os.Exit(runReviewInvite(os.Args[2:]))
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  admin create --email <email> [--reset-password]
  admin ensure-demo-user [--email demo@example.com] [--seller-account-id 1]
                         [--reset-password] [--transfer-seller-ownership]
  admin review-invite create --out <path> [--seller-account-id 1] [--hours 24]
  admin review-invite revoke --id <invite_id>
  admin review-invite list

Creates an admin login user, or ensures the canonical non-admin demo user,
or manages temporary external review invites.

Password is never accepted as a CLI argument.
Provide it interactively, or via AIVION_ADMIN_PASSWORD_FILE (file contents only).

Admin create:
  - email must also be listed in ADMIN_EMAILS for admin middleware access
  - idempotent create / optional password reset

Demo user:
  - must NOT be listed in ADMIN_EMAILS
  - binds an existing seller_account (default id=1)
  - --transfer-seller-ownership rebinds seller ownership from a prior owner
    (typical: admin becomes admin-only; demo owns seller 1)

Review invite:
  - cryptographically random token; raw URL written only to --out (mode 0600)
  - never prints the token; max TTL 24h; revoke invalidates invite + sessions`)
}

func runCreate(args []string) int {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	email := fs.String("email", "", "admin email (required)")
	resetPassword := fs.Bool("reset-password", false, "update password if user already exists")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(os.Stderr, "--email is required")
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		return 1
	}
	defer postgres.Pool.Close()

	result, err := adminbootstrap.EnsureAdminUser(ctx, postgres.Pool, *email, password, *resetPassword)
	if err != nil {
		if errors.Is(err, adminbootstrap.ErrAlreadyExists) {
			fmt.Fprintf(os.Stdout, "admin user already exists: email=%s user_id=%d (no changes)\n", result.Email, result.UserID)
			return 0
		}
		fmt.Fprintf(os.Stderr, "create admin failed: %v\n", err)
		return 1
	}

	switch {
	case result.Created:
		fmt.Fprintf(os.Stdout, "admin user created: email=%s user_id=%d\n", result.Email, result.UserID)
	case result.Updated:
		fmt.Fprintf(os.Stdout, "admin user password updated: email=%s user_id=%d\n", result.Email, result.UserID)
	default:
		fmt.Fprintf(os.Stdout, "admin user ok: email=%s user_id=%d\n", result.Email, result.UserID)
	}

	if !emailInAdminList(*email, cfg.Admin.Emails) {
		fmt.Fprintln(os.Stderr, "warning: email is not present in ADMIN_EMAILS; add it before using /api/v1/admin routes")
	}
	return 0
}

func emailInAdminList(email string, admins []string) bool {
	normalized := auth.NormalizeEmail(email)
	for _, a := range admins {
		if a == normalized {
			return true
		}
	}
	return false
}
