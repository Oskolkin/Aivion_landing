DROP INDEX IF EXISTS uq_search_seller_summaries_revision;
DROP INDEX IF EXISTS uq_search_query_details_revision;

-- Funnel/Finance uniqueness is intentionally not restored: it was a correctness defect.
-- Search state_hash uniqueness is not restored here either.
