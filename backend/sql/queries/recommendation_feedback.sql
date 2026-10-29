-- name: CreateRecommendationFeedback :one
INSERT INTO recommendation_feedback (
    recommendation_id,
    seller_account_id,
    rating,
    comment
)
VALUES (
    sqlc.arg(recommendation_id),
    sqlc.arg(seller_account_id),
    sqlc.arg(rating),
    sqlc.narg(comment)
)
ON CONFLICT (seller_account_id, recommendation_id)
DO UPDATE SET
    rating = EXCLUDED.rating,
    comment = EXCLUDED.comment,
    created_at = NOW()
RETURNING *;

-- name: GetRecommendationFeedbackByRecommendationID :one
SELECT *
FROM recommendation_feedback
WHERE seller_account_id = $1
  AND recommendation_id = $2;

-- name: ListRecommendationFeedbackBySeller :many
SELECT *
FROM recommendation_feedback
WHERE seller_account_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
OFFSET $3;
