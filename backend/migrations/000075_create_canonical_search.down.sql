DROP TRIGGER IF EXISTS search_market_scope ON search_market_facts;
DROP TRIGGER IF EXISTS search_detail_scope ON search_query_details;
DROP TRIGGER IF EXISTS search_summary_scope ON search_seller_summaries;
DROP FUNCTION IF EXISTS enforce_canonical_search_scope();
DROP TABLE IF EXISTS search_market_facts;
DROP TABLE IF EXISTS search_query_details;
DROP TABLE IF EXISTS search_seller_summaries;
DROP TABLE IF EXISTS search_queries;
