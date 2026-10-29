package auth

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type contextKey string

const (
	UserContextKey           contextKey = "auth_user"
	SellerAccountContextKey  contextKey = "auth_seller_account"
	ReadOnlyContextKey       contextKey = "auth_read_only"
	ReviewModeContextKey     contextKey = "auth_review_mode"
	LiveReviewModeContextKey contextKey = "auth_live_review_mode"
	ReviewInviteIDContextKey contextKey = "auth_review_invite_id"
	liveReviewConfigKey      contextKey = "auth_live_review_config"
)

func WithAuthContext(ctx context.Context, user dbgen.User, sellerAccount *dbgen.SellerAccount) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, user)
	if sellerAccount != nil {
		ctx = context.WithValue(ctx, SellerAccountContextKey, *sellerAccount)
	}
	return ctx
}

func WithSessionFlags(ctx context.Context, readOnly, reviewMode bool) context.Context {
	ctx = context.WithValue(ctx, ReadOnlyContextKey, readOnly)
	ctx = context.WithValue(ctx, ReviewModeContextKey, reviewMode)
	return ctx
}

func WithLiveReviewMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, LiveReviewModeContextKey, enabled)
}

func WithLiveReviewConfig(ctx context.Context, cfg *LiveReviewConfig) context.Context {
	if cfg == nil {
		return ctx
	}
	return context.WithValue(ctx, liveReviewConfigKey, cfg)
}

func WithReviewInviteID(ctx context.Context, inviteID int64) context.Context {
	if inviteID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, ReviewInviteIDContextKey, inviteID)
}

func UserFromContext(ctx context.Context) (dbgen.User, bool) {
	v := ctx.Value(UserContextKey)
	user, ok := v.(dbgen.User)
	return user, ok
}

func SellerAccountFromContext(ctx context.Context) (dbgen.SellerAccount, bool) {
	v := ctx.Value(SellerAccountContextKey)
	sellerAccount, ok := v.(dbgen.SellerAccount)
	return sellerAccount, ok
}

func ReadOnlyFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(ReadOnlyContextKey).(bool)
	return v
}

func ReviewModeFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(ReviewModeContextKey).(bool)
	return v
}

func LiveReviewModeFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(LiveReviewModeContextKey).(bool)
	return v
}

func liveReviewConfigFromContext(ctx context.Context) *LiveReviewConfig {
	v, _ := ctx.Value(liveReviewConfigKey).(*LiveReviewConfig)
	return v
}

func ReviewInviteIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(ReviewInviteIDContextKey).(int64)
	return v
}
