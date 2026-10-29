package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

func runReviewInvite(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: admin review-invite <create|revoke|list>")
		return 2
	}
	switch args[0] {
	case "create":
		return runReviewInviteCreate(args[1:])
	case "revoke":
		return runReviewInviteRevoke(args[1:])
	case "list":
		return runReviewInviteList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown review-invite subcommand %q\n", args[0])
		return 2
	}
}

func runReviewInviteCreate(args []string) int {
	fs := flag.NewFlagSet("review-invite create", flag.ContinueOnError)
	sellerID := fs.Int64("seller-account-id", 1, "seller account id bound to the review session")
	hours := fs.Int("hours", 24, "invite TTL in hours (max 24)")
	baseURL := fs.String("base-url", "https://aivion.pro", "public base URL for the review link")
	outPath := fs.String("out", "", "write full review URL to this file (0600); required")
	label := fs.String("label", "external-auditor", "optional non-secret label")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(os.Stderr, "--out is required (do not print the token to stdout)")
		return 2
	}
	if *hours <= 0 || *hours > 24 {
		fmt.Fprintln(os.Stderr, "--hours must be between 1 and 24")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
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

	q := dbgen.New(postgres.Pool)
	seller, err := q.GetSellerAccountByID(ctx, *sellerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			fmt.Fprintf(os.Stderr, "seller_account_id=%d not found\n", *sellerID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "lookup seller: %v\n", err)
		return 1
	}

	svc := auth.NewService(postgres.Pool, time.Duration(cfg.Auth.SessionTTLHours)*time.Hour, cfg.Admin.Emails)
	created, err := svc.CreateReviewInvite(ctx, auth.CreateReviewInviteInput{
		SellerAccountID: seller.ID,
		UserID:          seller.UserID,
		TTL:             time.Duration(*hours) * time.Hour,
		Label:           *label,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create invite: %v\n", err)
		return 1
	}

	url := strings.TrimRight(strings.TrimSpace(*baseURL), "/") + "/review/" + created.RawToken
	absOut, err := filepath.Abs(*outPath)
	if err != nil {
		absOut = *outPath
	}
	if err := os.WriteFile(absOut, []byte(url+"\n"), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write out file: %v\n", err)
		return 1
	}
	_ = os.Chmod(absOut, 0o600)

	fmt.Fprintf(os.Stdout, "review_invite_id=%d\n", created.InviteID)
	fmt.Fprintf(os.Stdout, "seller_account_id=%d\n", seller.ID)
	fmt.Fprintf(os.Stdout, "expires_at_utc=%s\n", created.ExpiresAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "url_file=%s\n", absOut)
	fmt.Fprintln(os.Stdout, "token_omitted=true")
	return 0
}

func runReviewInviteRevoke(args []string) int {
	fs := flag.NewFlagSet("review-invite revoke", flag.ContinueOnError)
	inviteID := fs.Int64("id", 0, "review invite id")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inviteID <= 0 {
		fmt.Fprintln(os.Stderr, "--id is required")
		return 2
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
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
	svc := auth.NewService(postgres.Pool, time.Duration(cfg.Auth.SessionTTLHours)*time.Hour, cfg.Admin.Emails)
	if err := svc.RevokeReviewInvite(ctx, *inviteID); err != nil {
		fmt.Fprintf(os.Stderr, "revoke: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "revoked_invite_id=%d\n", *inviteID)
	return 0
}

func runReviewInviteList(args []string) int {
	fs := flag.NewFlagSet("review-invite list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	_ = fs.Parse(args)
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
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
	svc := auth.NewService(postgres.Pool, time.Duration(cfg.Auth.SessionTTLHours)*time.Hour, cfg.Admin.Emails)
	items, err := svc.ListActiveReviewInvites(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		return 1
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stdout, "active_invites=0")
		return 0
	}
	for _, item := range items {
		label := ""
		if item.Label.Valid {
			label = item.Label.String
		}
		fmt.Fprintf(os.Stdout, "id=%d seller=%d user=%d expires=%s label=%q\n",
			item.ID,
			item.SellerAccountID,
			item.UserID,
			item.ExpiresAt.Time.UTC().Format(time.RFC3339),
			label,
		)
	}
	return 0
}
