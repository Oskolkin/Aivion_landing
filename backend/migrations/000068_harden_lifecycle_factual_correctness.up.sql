-- Version every posting attribute which may change after the stable identity is created.
ALTER TABLE sales_posting_versions
    ADD COLUMN canonical_order_id BIGINT NULL,
    ADD COLUMN provider_order_number TEXT NULL,
    ADD COLUMN fulfillment_scheme TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN order_resolution_status TEXT NOT NULL DEFAULT 'UNRESOLVED',
    ADD CONSTRAINT sales_posting_versions_order_fk
        FOREIGN KEY (canonical_order_id,seller_account_id) REFERENCES sales_orders(id,seller_account_id) ON DELETE RESTRICT,
    ADD CONSTRAINT sales_posting_versions_scheme_check
        CHECK (fulfillment_scheme IN ('FBO','FBS','RFBS','UNKNOWN')),
    ADD CONSTRAINT sales_posting_versions_order_resolution_check
        CHECK (order_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
    ADD CONSTRAINT sales_posting_versions_order_resolution_consistency
        CHECK ((order_resolution_status='RESOLVED')=(canonical_order_id IS NOT NULL));

-- Establish the formerly-stable projection as new knowledge at migration
-- acceptance time. Earlier cutoffs keep UNKNOWN/UNRESOLVED from the ALTER
-- defaults; no current value is backdated.
INSERT INTO sales_posting_versions(
    seller_account_id,canonical_posting_id,version_no,collection_domain_run_id,
    source_payload_id,source_received_at,observed_at,provider_status,
    provider_substatus,provider_created_at,in_process_at,delivering_date,
    shipment_date,cancel_reason_id,cancel_reason,cancellation_type,
    cancellation_initiator,cancelled_after_ship,warehouse_id,provider_id,
    state_hash,canonical_order_id,provider_order_number,fulfillment_scheme,order_resolution_status)
SELECT v.seller_account_id,v.canonical_posting_id,v.version_no+1,
       v.collection_domain_run_id,v.source_payload_id,v.source_received_at,NOW(),
       v.provider_status,v.provider_substatus,v.provider_created_at,v.in_process_at,
       v.delivering_date,v.shipment_date,v.cancel_reason_id,v.cancel_reason,
       v.cancellation_type,v.cancellation_initiator,v.cancelled_after_ship,
       v.warehouse_id,v.provider_id,v.state_hash,p.canonical_order_id,o.order_number,
       p.fulfillment_scheme,p.order_resolution_status
FROM sales_posting_versions v
JOIN sales_postings p ON p.id=v.canonical_posting_id
LEFT JOIN sales_orders o ON o.id=p.canonical_order_id
WHERE v.version_no=(SELECT max(x.version_no) FROM sales_posting_versions x WHERE x.canonical_posting_id=v.canonical_posting_id);

INSERT INTO sales_posting_products(
    seller_account_id,posting_version_id,line_no,canonical_sku_id,source_sku,
    sku_resolution_status,offer_id,quantity,unit_price,currency_code,
    product_name,source_payload_id)
SELECT pp.seller_account_id,newv.id,pp.line_no,pp.canonical_sku_id,pp.source_sku,
       pp.sku_resolution_status,pp.offer_id,pp.quantity,pp.unit_price,
       pp.currency_code,pp.product_name,pp.source_payload_id
FROM sales_posting_versions newv
JOIN sales_posting_versions oldv
  ON oldv.canonical_posting_id=newv.canonical_posting_id
 AND oldv.version_no=newv.version_no-1
JOIN sales_posting_products pp ON pp.posting_version_id=oldv.id
WHERE newv.created_at=transaction_timestamp()
  AND newv.observed_at=transaction_timestamp();

-- The pre-000068 stable columns are retained as a current projection only.  We do
-- not backdate their current values into historical versions.
ALTER TABLE sales_posting_products
    ALTER COLUMN source_sku DROP NOT NULL,
    DROP CONSTRAINT sales_posting_products_source_sku_check,
    ADD CONSTRAINT sales_posting_products_source_sku_check
        CHECK (source_sku IS NULL OR source_sku > 0),
    ADD CONSTRAINT sales_posting_products_missing_sku_check
        CHECK (source_sku IS NOT NULL OR sku_resolution_status='UNRESOLVED');

CREATE TABLE sales_posting_relation_versions (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    posting_relation_id BIGINT NOT NULL REFERENCES sales_posting_relations(id) ON DELETE RESTRICT,
    revision_no BIGINT NOT NULL CHECK (revision_no > 0),
    parent_posting_id BIGINT NULL,
    resolution_status TEXT NOT NULL CHECK (resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
    observed_at TIMESTAMPTZ NOT NULL,
    source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
    collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
    state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(posting_relation_id,revision_no),
    UNIQUE(id,seller_account_id),
    FOREIGN KEY(parent_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id) ON DELETE RESTRICT,
    CHECK ((resolution_status='RESOLVED')=(parent_posting_id IS NOT NULL))
);
CREATE INDEX idx_sales_posting_relation_versions_cutoff
    ON sales_posting_relation_versions(posting_relation_id,observed_at DESC,revision_no DESC);

-- Existing mutable relation state becomes known only when this migration is
-- accepted.  It is deliberately not made visible to earlier cutoffs.
INSERT INTO sales_posting_relation_versions(
    seller_account_id,posting_relation_id,revision_no,parent_posting_id,
    resolution_status,observed_at,source_payload_id,collection_domain_run_id,state_hash)
SELECT seller_account_id,id,1,parent_posting_id,resolution_status,NOW(),
       source_payload_id,collection_domain_run_id,
       md5(id::text || ':' || COALESCE(parent_posting_id::text,'') || ':' || resolution_status) ||
       md5(resolution_status || ':' || id::text)
FROM sales_posting_relations;

CREATE TRIGGER sales_posting_relation_versions_scope
BEFORE INSERT OR UPDATE ON sales_posting_relation_versions
FOR EACH ROW EXECUTE FUNCTION enforce_sales_lifecycle_domain_scope();
