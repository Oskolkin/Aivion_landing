-- name: CountChatUserQuestionsSince :one
SELECT COUNT(*)::bigint
FROM chat_messages
WHERE seller_account_id = $1
  AND role = 'user'
  AND message_type = 'question'
  AND created_at >= $2;

-- name: CountRecommendationRunsSince :one
SELECT COUNT(*)::bigint
FROM recommendation_runs
WHERE seller_account_id = $1
  AND started_at >= $2;

-- name: CountAIDailySummaryRunsSince :one
SELECT COUNT(*)::bigint
FROM ai_daily_summary_runs
WHERE seller_account_id = $1
  AND created_at >= $2;
