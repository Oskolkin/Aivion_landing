package cleanup

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ArchiveStaleActiveChatSessionsSQL is the maintenance UPDATE (contract tests rely on it).
const ArchiveStaleActiveChatSessionsSQL = `
UPDATE chat_sessions
SET status = 'archived', updated_at = NOW()
WHERE status = 'active'
  AND updated_at < NOW() - ($1::bigint * INTERVAL '1 day')
`

// ArchiveStaleActiveChatSessions sets status=archived for active sessions whose updated_at
// is older than retentionDays. Does not delete messages or business tables.
func ArchiveStaleActiveChatSessions(ctx context.Context, pool *pgxpool.Pool, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("retentionDays must be > 0")
	}
	tag, err := pool.Exec(ctx, strings.TrimSpace(ArchiveStaleActiveChatSessionsSQL), int64(retentionDays))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
