package chat

import (
	"context"
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DuplicateActionChatCleanupResult struct {
	DuplicateGroups   int     `json:"duplicate_groups"`
	SessionsAffected  int     `json:"sessions_affected"`
	CanonicalSessions []int64 `json:"canonical_sessions"`
	MessagesToMove    int     `json:"messages_to_move"`
	AttachmentsToMove int     `json:"attachments_to_move"`
	DryRun            bool    `json:"dry_run"`
}

type duplicateSessionInfo struct {
	ID            int64
	MessageCount  int64
	LastMessageAt int64 // unix for sort
	CreatedAt     int64
}

func CleanupDuplicateActionChats(ctx context.Context, pool *pgxpool.Pool, dryRun bool) (DuplicateActionChatCleanupResult, error) {
	out := DuplicateActionChatCleanupResult{DryRun: dryRun}
	if pool == nil {
		return out, fmt.Errorf("postgres pool is required")
	}
	q := dbgen.New(pool)
	groups, err := q.ListDuplicateActionChatSessions(ctx)
	if err != nil {
		return out, err
	}
	out.DuplicateGroups = len(groups)
	for _, group := range groups {
		sessionIDs, err := parseInt64Array(group.SessionIds)
		if err != nil {
			return out, err
		}
		if len(sessionIDs) < 2 {
			continue
		}
		infos := make([]duplicateSessionInfo, 0, len(sessionIDs))
		for _, sid := range sessionIDs {
			msgCount, err := q.CountChatMessagesBySessionID(ctx, dbgen.CountChatMessagesBySessionIDParams{
				SellerAccountID: group.SellerAccountID,
				SessionID:       sid,
			})
			if err != nil {
				return out, err
			}
			sess, err := q.GetChatSessionByID(ctx, dbgen.GetChatSessionByIDParams{
				SellerAccountID: group.SellerAccountID,
				ID:              sid,
			})
			if err != nil {
				return out, err
			}
			var lastMsg int64
			if sess.LastMessageAt.Valid {
				lastMsg = sess.LastMessageAt.Time.Unix()
			}
			infos = append(infos, duplicateSessionInfo{
				ID:            sid,
				MessageCount:  msgCount,
				LastMessageAt: lastMsg,
				CreatedAt:     sess.CreatedAt.Time.Unix(),
			})
		}
		sort.SliceStable(infos, func(i, j int) bool {
			if infos[i].MessageCount != infos[j].MessageCount {
				return infos[i].MessageCount > infos[j].MessageCount
			}
			if infos[i].LastMessageAt != infos[j].LastMessageAt {
				return infos[i].LastMessageAt > infos[j].LastMessageAt
			}
			return infos[i].ID < infos[j].ID
		})
		canonical := infos[0].ID
		out.CanonicalSessions = append(out.CanonicalSessions, canonical)
		for _, dup := range infos[1:] {
			out.SessionsAffected++
			out.MessagesToMove += int(dup.MessageCount)
			if dryRun {
				continue
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				return out, err
			}
			tq := q.WithTx(tx)
			if err := tq.MoveChatMessagesToSession(ctx, dbgen.MoveChatMessagesToSessionParams{
				SellerAccountID: group.SellerAccountID,
				SessionID:       dup.ID,
				SessionID_2:     canonical,
			}); err != nil {
				_ = tx.Rollback(ctx)
				return out, err
			}
			if err := tq.MoveChatAttachmentsToSession(ctx, dbgen.MoveChatAttachmentsToSessionParams{
				SellerAccountID: group.SellerAccountID,
				ChatSessionID:   pgtype.Int8{Int64: dup.ID, Valid: true},
				ChatSessionID_2: pgtype.Int8{Int64: canonical, Valid: true},
			}); err != nil {
				_ = tx.Rollback(ctx)
				return out, err
			}
			if err := tq.MoveChatTracesToSession(ctx, dbgen.MoveChatTracesToSessionParams{
				SellerAccountID: group.SellerAccountID,
				SessionID:       dup.ID,
				SessionID_2:     canonical,
			}); err != nil {
				_ = tx.Rollback(ctx)
				return out, err
			}
			if _, err := tq.SoftDeleteChatSession(ctx, dbgen.SoftDeleteChatSessionParams{
				SellerAccountID: group.SellerAccountID,
				ID:              dup.ID,
			}); err != nil {
				_ = tx.Rollback(ctx)
				return out, err
			}
			if err := tx.Commit(ctx); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func parseInt64Array(v any) ([]int64, error) {
	switch t := v.(type) {
	case []int64:
		return append([]int64(nil), t...), nil
	case []int32:
		out := make([]int64, len(t))
		for i, n := range t {
			out[i] = int64(n)
		}
		return out, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported session id array type %T", v)
	}
}

func int64Slice(v []int64) []int64 {
	if len(v) == 0 {
		return nil
	}
	out := append([]int64(nil), v...)
	return out
}
