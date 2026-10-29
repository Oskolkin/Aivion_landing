DROP TRIGGER IF EXISTS funnel_observations_scope_trigger ON funnel_observations;
DROP FUNCTION IF EXISTS enforce_funnel_scope();
DROP TABLE IF EXISTS funnel_observations;

DELETE FROM collection_authoritative_heads WHERE domain IN ('FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY');
DELETE FROM data_quality_issues WHERE domain IN ('FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY');
DELETE FROM business_capability_assessments WHERE domain IN ('FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY');
DELETE FROM domain_watermarks WHERE domain IN ('FUNNEL','FINANCE','SEARCH','PROMOTIONS','ACCOUNT_HEALTH','SUPPLY');

ALTER TABLE collection_authoritative_heads DROP CONSTRAINT collection_authoritative_heads_domain_check;
ALTER TABLE collection_authoritative_heads ADD CONSTRAINT collection_authoritative_heads_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));

ALTER TABLE data_quality_issues DROP CONSTRAINT data_quality_issues_domain_check;
ALTER TABLE data_quality_issues ADD CONSTRAINT data_quality_issues_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));

ALTER TABLE business_capability_assessments DROP CONSTRAINT business_capability_assessments_domain_check;
ALTER TABLE business_capability_assessments ADD CONSTRAINT business_capability_assessments_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));

ALTER TABLE domain_watermarks DROP CONSTRAINT domain_watermarks_domain_check;
ALTER TABLE domain_watermarks ADD CONSTRAINT domain_watermarks_domain_check
  CHECK (domain IN ('CATALOG','SALES_DEMAND','POSTINGS','RETURNS','STOCK','PRICE','ADVERTISING'));
