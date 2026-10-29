-- name: UpsertDescriptionCategory :exec
INSERT INTO description_categories (
    description_category_id,
    parent_id,
    category_name,
    path,
    updated_at
) VALUES (
    $1, $2, $3, $4, NOW()
)
ON CONFLICT (description_category_id) DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    category_name = EXCLUDED.category_name,
    path = EXCLUDED.path,
    updated_at = NOW();

-- name: GetDescriptionCategoryByID :one
SELECT *
FROM description_categories
WHERE description_category_id = $1;

-- name: CountDescriptionCategories :one
SELECT COUNT(*)::bigint
FROM description_categories;
