-- name: UpsertStockAnalyticsSnapshot :one
INSERT INTO stock_analytics_snapshots (
    seller_account_id,
    snapshot_date,
    sku,
    offer_id,
    product_name,
    warehouse_id,
    warehouse_name,
    cluster_id,
    cluster_name,
    macrolocal_cluster_id,
    available_stock_count,
    excess_stock_count,
    ads,
    ads_cluster,
    idc,
    idc_cluster,
    turnover_grade,
    turnover_grade_cluster,
    days_without_sales,
    days_without_sales_cluster,
    requested_stock_count,
    transit_stock_count,
    valid_stock_count,
    waiting_docs_stock_count,
    return_from_customer_stock_count,
    return_to_seller_stock_count,
    expiring_stock_count,
    other_stock_count,
    stock_defect_stock_count,
    transit_defect_stock_count,
    item_tags_json,
    captured_at,
    raw_response_json,
    raw_payload_id,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, NOW()
)
ON CONFLICT (seller_account_id, snapshot_date, sku, warehouse_id, cluster_id)
DO UPDATE SET
    offer_id = EXCLUDED.offer_id,
    product_name = EXCLUDED.product_name,
    warehouse_name = EXCLUDED.warehouse_name,
    cluster_name = EXCLUDED.cluster_name,
    macrolocal_cluster_id = EXCLUDED.macrolocal_cluster_id,
    available_stock_count = EXCLUDED.available_stock_count,
    excess_stock_count = EXCLUDED.excess_stock_count,
    ads = EXCLUDED.ads,
    ads_cluster = EXCLUDED.ads_cluster,
    idc = EXCLUDED.idc,
    idc_cluster = EXCLUDED.idc_cluster,
    turnover_grade = EXCLUDED.turnover_grade,
    turnover_grade_cluster = EXCLUDED.turnover_grade_cluster,
    days_without_sales = EXCLUDED.days_without_sales,
    days_without_sales_cluster = EXCLUDED.days_without_sales_cluster,
    requested_stock_count = EXCLUDED.requested_stock_count,
    transit_stock_count = EXCLUDED.transit_stock_count,
    valid_stock_count = EXCLUDED.valid_stock_count,
    waiting_docs_stock_count = EXCLUDED.waiting_docs_stock_count,
    return_from_customer_stock_count = EXCLUDED.return_from_customer_stock_count,
    return_to_seller_stock_count = EXCLUDED.return_to_seller_stock_count,
    expiring_stock_count = EXCLUDED.expiring_stock_count,
    other_stock_count = EXCLUDED.other_stock_count,
    stock_defect_stock_count = EXCLUDED.stock_defect_stock_count,
    transit_defect_stock_count = EXCLUDED.transit_defect_stock_count,
    item_tags_json = EXCLUDED.item_tags_json,
    captured_at = EXCLUDED.captured_at,
    raw_response_json = COALESCE(EXCLUDED.raw_response_json, stock_analytics_snapshots.raw_response_json),
    raw_payload_id = COALESCE(EXCLUDED.raw_payload_id, stock_analytics_snapshots.raw_payload_id),
    updated_at = NOW()
RETURNING *;

-- name: ListStockAnalyticsSnapshotsBySellerAndDate :many
SELECT *
FROM stock_analytics_snapshots
WHERE seller_account_id = $1
  AND snapshot_date = $2
ORDER BY sku ASC, warehouse_id ASC;

-- name: GetLatestStockAnalyticsSnapshotDateBySeller :one
SELECT MAX(snapshot_date)::date AS snapshot_date
FROM stock_analytics_snapshots
WHERE seller_account_id = $1;
