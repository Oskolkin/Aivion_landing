DROP TRIGGER IF EXISTS search_sku_summary_scope ON search_sku_summaries;
DROP TABLE IF EXISTS search_sku_summaries;
DROP TABLE IF EXISTS search_query_versions;

ALTER TABLE search_query_details
  DROP COLUMN IF EXISTS sort_by,
  DROP COLUMN IF EXISTS sort_dir,
  DROP COLUMN IF EXISTS limit_by_sku,
  DROP COLUMN IF EXISTS page,
  DROP COLUMN IF EXISTS page_size;
