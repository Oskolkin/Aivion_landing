package chat

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
)

type chatexportAsOfResolver struct {
	data *chatexport.DataService
}

func NewChatexportAsOfResolver(data *chatexport.DataService) *chatexportAsOfResolver {
	return &chatexportAsOfResolver{data: data}
}

func (r *chatexportAsOfResolver) ResolveAsOfDate(ctx context.Context, sellerAccountID int64, requested *time.Time) (time.Time, error) {
	if r == nil || r.data == nil {
		return time.Now().UTC().Truncate(24 * time.Hour), nil
	}
	return r.data.ResolveAsOfDate(ctx, sellerAccountID, requested)
}
