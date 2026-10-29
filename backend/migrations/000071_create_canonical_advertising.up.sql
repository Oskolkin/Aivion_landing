CREATE TABLE advertising_campaigns (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_campaign_id BIGINT NOT NULL CHECK (provider_campaign_id > 0),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider_campaign_id),
  UNIQUE (id,seller_account_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE advertising_campaign_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  advertising_campaign_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  title TEXT NULL,
  provider_state TEXT NOT NULL,
  payment_type TEXT NULL,
  adv_object_type TEXT NULL,
  campaign_family TEXT NOT NULL CHECK (campaign_family IN ('PRODUCT_CPC','SEARCH_PROMO_CPO','MEDIA_CPM','UNKNOWN')),
  placement_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  budget NUMERIC(30,6) NULL CHECK (budget IS NULL OR budget >= 0),
  weekly_budget NUMERIC(30,6) NULL CHECK (weekly_budget IS NULL OR weekly_budget >= 0),
  currency_code VARCHAR(3) NULL CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),
  from_date DATE NULL,
  to_date DATE NULL,
  strategy TEXT NULL,
  product_autopilot_strategy TEXT NULL,
  is_autocreated BOOLEAN NULL,
  autostop_status TEXT NULL,
  target_scope_type TEXT NOT NULL CHECK (target_scope_type IN ('EXPLICIT_SKU_SET','ALL_ELIGIBLE_SKUS','NON_SKU_OBJECTS','UNKNOWN')),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (advertising_campaign_id,version_no),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (advertising_campaign_id,seller_account_id) REFERENCES advertising_campaigns(id,seller_account_id)
);
CREATE INDEX idx_ad_campaign_versions_cutoff ON advertising_campaign_versions(advertising_campaign_id,observed_at DESC,version_no DESC);

CREATE TABLE advertising_target_intervals (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  advertising_campaign_id BIGINT NOT NULL,
  canonical_sku_id BIGINT NULL,
  source_sku BIGINT NULL CHECK (source_sku IS NULL OR source_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','NOT_APPLICABLE','CONFLICT')),
  assignment_state TEXT NOT NULL,
  bid NUMERIC(30,6) NULL CHECK (bid IS NULL OR bid >= 0),
  bid_unit TEXT NULL,
  target_cir NUMERIC(30,6) NULL,
  top_position TEXT NULL,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_from TIMESTAMPTZ NOT NULL,
  observed_to TIMESTAMPTZ NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (advertising_campaign_id,seller_account_id) REFERENCES advertising_campaigns(id,seller_account_id),
  FOREIGN KEY (canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK (observed_to IS NULL OR observed_to >= observed_from),
  CHECK ((sku_resolution_status='RESOLVED')=(canonical_sku_id IS NOT NULL))
);
CREATE UNIQUE INDEX advertising_target_open_identity ON advertising_target_intervals(advertising_campaign_id,COALESCE(source_sku,0)) WHERE observed_to IS NULL;

CREATE TABLE advertising_daily_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  advertising_campaign_id BIGINT NOT NULL,
  business_date DATE NOT NULL,
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  impressions BIGINT NULL CHECK (impressions IS NULL OR impressions >= 0),
  clicks BIGINT NULL CHECK (clicks IS NULL OR clicks >= 0),
  paid_spend_amount NUMERIC(30,6) NULL CHECK (paid_spend_amount IS NULL OR paid_spend_amount >= 0),
  bonus_spend_amount NUMERIC(30,6) NULL CHECK (bonus_spend_amount IS NULL OR bonus_spend_amount >= 0),
  attributed_order_units BIGINT NULL CHECK (attributed_order_units IS NULL OR attributed_order_units >= 0),
  attributed_order_amount NUMERIC(30,6) NULL CHECK (attributed_order_amount IS NULL OR attributed_order_amount >= 0),
  attribution_kind TEXT NOT NULL CHECK (attribution_kind IN ('CLICK','VIEW','MODEL','PROVIDER_DEFAULT','UNKNOWN')),
  provider_drr NUMERIC(30,6) NULL CHECK (provider_drr IS NULL OR provider_drr >= 0),
  currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  traffic_state TEXT NOT NULL CHECK (traffic_state IN ('USABLE','PARTIAL','UNAVAILABLE')),
  attribution_state TEXT NOT NULL CHECK (attribution_state IN ('MATURE','PROVISIONAL','PENDING_ACCEPTANCE','UNAVAILABLE')),
  normalized_day_hash TEXT NOT NULL CHECK (normalized_day_hash ~ '^[0-9a-f]{64}$'),
  source_contract_key TEXT NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (advertising_campaign_id,business_date,attribution_kind,revision_no),
  FOREIGN KEY (advertising_campaign_id,seller_account_id) REFERENCES advertising_campaigns(id,seller_account_id)
);
CREATE INDEX idx_ad_daily_cutoff ON advertising_daily_facts(seller_account_id,business_date,observed_at DESC,id DESC);

CREATE TABLE advertising_product_daily_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  advertising_campaign_id BIGINT NOT NULL,
  business_date DATE NOT NULL,
  promoted_canonical_sku_id BIGINT NULL,
  source_promoted_sku BIGINT NOT NULL CHECK (source_promoted_sku > 0),
  sku_resolution_status TEXT NOT NULL CHECK (sku_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  impressions BIGINT NULL CHECK (impressions IS NULL OR impressions >= 0),
  clicks BIGINT NULL CHECK (clicks IS NULL OR clicks >= 0),
  paid_spend_amount NUMERIC(30,6) NULL CHECK (paid_spend_amount IS NULL OR paid_spend_amount >= 0),
  bonus_spend_amount NUMERIC(30,6) NULL CHECK (bonus_spend_amount IS NULL OR bonus_spend_amount >= 0),
  attributed_order_units BIGINT NULL CHECK (attributed_order_units IS NULL OR attributed_order_units >= 0),
  attributed_order_amount NUMERIC(30,6) NULL CHECK (attributed_order_amount IS NULL OR attributed_order_amount >= 0),
  attribution_kind TEXT NOT NULL CHECK (attribution_kind IN ('CLICK','VIEW','MODEL','PROVIDER_DEFAULT','UNKNOWN')),
  currency_code VARCHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (advertising_campaign_id,business_date,source_promoted_sku,attribution_kind,revision_no),
  FOREIGN KEY (advertising_campaign_id,seller_account_id) REFERENCES advertising_campaigns(id,seller_account_id),
  FOREIGN KEY (promoted_canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  CHECK ((sku_resolution_status='RESOLVED')=(promoted_canonical_sku_id IS NOT NULL))
);

CREATE TABLE advertising_cpo_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  advertising_campaign_id BIGINT NOT NULL,
  business_date DATE NOT NULL,
  promoted_canonical_sku_id BIGINT NULL,
  ordered_canonical_sku_id BIGINT NULL,
  canonical_order_id BIGINT NULL,
  source_promoted_sku BIGINT NULL,
  source_ordered_sku BIGINT NULL,
  source_order_id BIGINT NULL,
  promoted_sku_resolution_status TEXT NOT NULL,
  ordered_sku_resolution_status TEXT NOT NULL,
  order_resolution_status TEXT NOT NULL,
  attributed_order_units BIGINT NULL CHECK (attributed_order_units IS NULL OR attributed_order_units >= 0),
  attributed_order_amount NUMERIC(30,6) NULL CHECK (attributed_order_amount IS NULL OR attributed_order_amount >= 0),
  currency_code VARCHAR(3) NULL CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),
  attribution_kind TEXT NOT NULL CHECK (attribution_kind IN ('CLICK','VIEW','MODEL','PROVIDER_DEFAULT','UNKNOWN')),
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  FOREIGN KEY (advertising_campaign_id,seller_account_id) REFERENCES advertising_campaigns(id,seller_account_id),
  FOREIGN KEY (promoted_canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  FOREIGN KEY (ordered_canonical_sku_id,seller_account_id) REFERENCES canonical_skus(id,seller_account_id),
  FOREIGN KEY (canonical_order_id,seller_account_id) REFERENCES sales_orders(id,seller_account_id)
);

CREATE TABLE advertising_spend_reconciliations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  business_date DATE NOT NULL,
  daily_paid_spend NUMERIC(30,6) NULL,
  expense_paid_spend NUMERIC(30,6) NULL,
  currency_code VARCHAR(3) NULL,
  reconciliation_status TEXT NOT NULL CHECK (reconciliation_status IN ('PASS','DIFFERENT','NOT_COMPARABLE','PENDING_ACCEPTANCE')),
  details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE advertising_report_requests (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  source_contract_key TEXT NOT NULL,
  request_fingerprint TEXT NOT NULL,
  provider_report_id TEXT NULL,
  status TEXT NOT NULL CHECK (status IN ('SUBMITTED','RUNNING','READY','FAILED','CANCELLED')),
  status_observed_at TIMESTAMPTZ NOT NULL,
  result_source_payload_id BIGINT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (collection_domain_run_id,request_fingerprint)
);

CREATE OR REPLACE FUNCTION enforce_canonical_ad_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='ADVERTISING') THEN
    RAISE EXCEPTION 'canonical advertising domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER advertising_campaign_versions_scope BEFORE INSERT OR UPDATE ON advertising_campaign_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_target_intervals_scope BEFORE INSERT OR UPDATE ON advertising_target_intervals FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_daily_facts_scope BEFORE INSERT OR UPDATE ON advertising_daily_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_product_daily_facts_scope BEFORE INSERT OR UPDATE ON advertising_product_daily_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_cpo_facts_scope BEFORE INSERT OR UPDATE ON advertising_cpo_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_spend_reconciliations_scope BEFORE INSERT OR UPDATE ON advertising_spend_reconciliations FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
CREATE TRIGGER advertising_report_requests_scope BEFORE INSERT OR UPDATE ON advertising_report_requests FOR EACH ROW EXECUTE FUNCTION enforce_canonical_ad_scope();
