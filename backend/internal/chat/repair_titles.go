package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RepairChatTitlesResult struct {
	DryRun         bool     `json:"dry_run"`
	Scanned        int      `json:"scanned"`
	CorruptedFound int      `json:"corrupted_found"`
	Repaired       int      `json:"repaired"`
	SkippedCorrect int      `json:"skipped_correct"`
	Unrecoverable  int      `json:"unrecoverable"`
	Examples       []string `json:"examples,omitempty"`
}

type sessionTitleRow struct {
	ID              int64
	SellerAccountID int64
	Title           string
	ActionKey       *string
}

// RepairChatTitles rewrites corrupted session titles using current publishable action titles or first user messages.
func RepairChatTitles(ctx context.Context, pool *pgxpool.Pool, dryRun bool) (*RepairChatTitlesResult, error) {
	return RepairChatTitlesForSeller(ctx, pool, 0, dryRun, 0)
}

// RepairChatTitlesForSeller repairs titles for one seller (sellerID=0 means all sellers).
func RepairChatTitlesForSeller(ctx context.Context, pool *pgxpool.Pool, sellerID int64, dryRun bool, batchLimit int) (*RepairChatTitlesResult, error) {
	query := `
SELECT id, seller_account_id, title, action_key
FROM chat_sessions
WHERE deleted_at IS NULL`
	args := []any{}
	if sellerID > 0 {
		query += ` AND seller_account_id = $1`
		args = append(args, sellerID)
	}
	query += ` ORDER BY id`
	if batchLimit > 0 {
		if sellerID > 0 {
			query += ` LIMIT $2`
			args = append(args, batchLimit)
		} else {
			query += fmt.Sprintf(" LIMIT %d", batchLimit)
		}
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat sessions: %w", err)
	}
	defer rows.Close()

	result := &RepairChatTitlesResult{DryRun: dryRun}
	for rows.Next() {
		var row sessionTitleRow
		var actionKey *string
		if err := rows.Scan(&row.ID, &row.SellerAccountID, &row.Title, &actionKey); err != nil {
			return nil, fmt.Errorf("scan chat session: %w", err)
		}
		row.ActionKey = actionKey
		result.Scanned++

		if !IsCorruptedChatTitle(row.Title) {
			result.SkippedCorrect++
			continue
		}
		result.CorruptedFound++

		nextTitle, err := resolveRepairTitle(ctx, pool, row)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(nextTitle) == "" || IsCorruptedChatTitle(nextTitle) {
			nextTitle = "Новый чат"
			result.Unrecoverable++
		}

		if len(result.Examples) < 8 {
			result.Examples = append(result.Examples,
				fmt.Sprintf("session_id=%d old=%q new=%q", row.ID, row.Title, nextTitle))
		}

		if dryRun {
			continue
		}

		if strings.TrimSpace(nextTitle) == strings.TrimSpace(row.Title) {
			result.SkippedCorrect++
			continue
		}

		_, err = pool.Exec(ctx, `
UPDATE chat_sessions
SET title = $3, updated_at = NOW()
WHERE seller_account_id = $1 AND id = $2`, row.SellerAccountID, row.ID, nextTitle)
		if err != nil {
			return nil, fmt.Errorf("update session title id=%d: %w", row.ID, err)
		}
		result.Repaired++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveRepairTitle(ctx context.Context, pool *pgxpool.Pool, row sessionTitleRow) (string, error) {
	actionKey := strings.TrimSpace(ptrString(row.ActionKey))
	if actionKey != "" {
		var actionTitle string
		err := pool.QueryRow(ctx, `
SELECT ai.title
FROM action_items ai
INNER JOIN action_generation_runs agr
    ON agr.id = ai.run_id
    AND agr.seller_account_id = ai.seller_account_id
WHERE ai.seller_account_id = $1
  AND ai.action_key = $2
  AND agr.status = 'completed'
  AND COALESCE(agr.final_status, 'completed') IN (
    'completed', 'completed_repaired', 'completed_partial', 'completed_after_retry', 'empty'
  )
ORDER BY agr.metric_date DESC NULLS LAST, agr.created_at DESC, agr.id DESC, ai.created_at DESC, ai.id DESC
LIMIT 1`, row.SellerAccountID, actionKey).Scan(&actionTitle)
		if err == nil {
			title := BuildChatTitle(actionTitle, "", "Новый чат")
			if !IsCorruptedChatTitle(title) {
				return title, nil
			}
		}
	}

	var firstUserMessage string
	err := pool.QueryRow(ctx, `
SELECT content
FROM chat_messages
WHERE seller_account_id = $1
  AND session_id = $2
  AND role = 'user'
ORDER BY created_at ASC, id ASC
LIMIT 1`, row.SellerAccountID, row.ID).Scan(&firstUserMessage)
	if err == nil {
		title := BuildChatTitle("", firstUserMessage, "Новый чат")
		if !IsCorruptedChatTitle(title) {
			return title, nil
		}
	}

	return "Новый чат", nil
}

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
