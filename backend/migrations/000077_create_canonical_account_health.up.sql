CREATE TABLE account_health_metrics (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_rating_key TEXT NOT NULL CHECK (btrim(provider_rating_key) <> ''),
  first_observed_at TIMESTAMPTZ NOT NULL,
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider,provider_rating_key),
  UNIQUE (id,seller_account_id),
  CHECK (last_observed_at >= first_observed_at)
);

CREATE TABLE account_health_metric_versions (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  account_health_metric_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  canonical_metric_key TEXT NOT NULL,
  mapping_status TEXT NOT NULL CHECK (mapping_status IN ('MAPPED','UNMAPPED')),
  provider_group_name TEXT NULL,
  provider_display_name TEXT NULL,
  value_type TEXT NULL,
  rating_direction TEXT NULL,
  canonical_family TEXT NULL,
  known_calculation_window TEXT NULL,
  history_supported BOOLEAN NULL,
  value_unit TEXT NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (account_health_metric_id,version_no),
  UNIQUE (id,seller_account_id),
  FOREIGN KEY (account_health_metric_id,seller_account_id) REFERENCES account_health_metrics(id,seller_account_id)
);
CREATE INDEX idx_account_health_metric_versions_cutoff ON account_health_metric_versions(account_health_metric_id,observed_at DESC,version_no DESC,id DESC);

CREATE TABLE account_health_snapshot_runs (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  observed_at TIMESTAMPTZ NOT NULL,
  completeness_status TEXT NOT NULL CHECK (completeness_status IN ('COMPLETE','PARTIAL')),
  rows_received BIGINT NOT NULL DEFAULT 0 CHECK (rows_received >= 0),
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,collection_domain_run_id),
  UNIQUE (id,seller_account_id)
);

CREATE TABLE account_rating_snapshots (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  snapshot_run_id BIGINT NOT NULL,
  account_health_metric_id BIGINT NOT NULL,
  provider_value NUMERIC(30,6) NULL,
  provider_previous_value NUMERIC(30,6) NULL,
  provider_status TEXT NULL,
  provider_change_direction TEXT NULL,
  provider_change_meaning TEXT NULL,
  penalty_amount NUMERIC(30,6) NULL,
  penalty_currency_code VARCHAR(3) NULL CHECK (penalty_currency_code IS NULL OR penalty_currency_code ~ '^[A-Z]{3}$'),
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (snapshot_run_id,account_health_metric_id),
  FOREIGN KEY (snapshot_run_id,seller_account_id) REFERENCES account_health_snapshot_runs(id,seller_account_id) ON DELETE CASCADE,
  FOREIGN KEY (account_health_metric_id,seller_account_id) REFERENCES account_health_metrics(id,seller_account_id)
);

CREATE TABLE account_rating_period_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  account_health_metric_id BIGINT NOT NULL,
  period_from DATE NOT NULL,
  period_to DATE NOT NULL,
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  provider_value NUMERIC(30,6) NULL,
  provider_status TEXT NULL,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (account_health_metric_id,period_from,period_to,revision_no),
  FOREIGN KEY (account_health_metric_id,seller_account_id) REFERENCES account_health_metrics(id,seller_account_id),
  CHECK (period_to >= period_from)
);
CREATE INDEX idx_account_rating_period_facts_cutoff ON account_rating_period_facts(account_health_metric_id,period_from,period_to,observed_at DESC,revision_no DESC);

CREATE TABLE account_rating_threshold_observations (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  account_health_metric_id BIGINT NOT NULL,
  version_no BIGINT NOT NULL CHECK (version_no > 0),
  provider_status TEXT NOT NULL,
  threshold_from NUMERIC(30,9) NULL,
  threshold_to NUMERIC(30,9) NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (account_health_metric_id,provider_status,version_no),
  FOREIGN KEY (account_health_metric_id,seller_account_id) REFERENCES account_health_metrics(id,seller_account_id)
);
CREATE INDEX idx_account_rating_thresholds_cutoff ON account_rating_threshold_observations(account_health_metric_id,observed_at DESC,version_no DESC);

CREATE TABLE account_penalty_event_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_event_key TEXT NOT NULL,
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  account_health_metric_id BIGINT NULL,
  occurred_at TIMESTAMPTZ NULL,
  penalty_amount NUMERIC(30,6) NULL,
  currency_code VARCHAR(3) NULL CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$'),
  description TEXT NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider_event_key,revision_no),
  FOREIGN KEY (account_health_metric_id,seller_account_id) REFERENCES account_health_metrics(id,seller_account_id)
);

CREATE TABLE fbs_error_index_facts (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  observed_at TIMESTAMPTZ NOT NULL,
  provider_period_from DATE NULL,
  provider_period_to DATE NULL,
  index_value NUMERIC(30,9) NULL,
  provider_status TEXT NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,collection_domain_run_id),
  CHECK ((provider_period_from IS NULL)=(provider_period_to IS NULL)),
  CHECK (provider_period_to IS NULL OR provider_period_to >= provider_period_from)
);

CREATE TABLE fbs_posting_error_events (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
  provider_event_key TEXT NOT NULL,
  revision_no BIGINT NOT NULL CHECK (revision_no > 0),
  canonical_posting_id BIGINT NULL,
  source_posting_number TEXT NULL,
  posting_resolution_status TEXT NOT NULL CHECK (posting_resolution_status IN ('RESOLVED','UNRESOLVED','CONFLICT')),
  error_code TEXT NULL,
  error_message TEXT NULL,
  occurred_at TIMESTAMPTZ NULL,
  provider_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  state_hash TEXT NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
  observed_at TIMESTAMPTZ NOT NULL,
  source_payload_id BIGINT NOT NULL REFERENCES source_payloads(id) ON DELETE RESTRICT,
  collection_domain_run_id BIGINT NOT NULL REFERENCES collection_domain_runs(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seller_account_id,provider_event_key,revision_no),
  FOREIGN KEY (canonical_posting_id,seller_account_id) REFERENCES sales_postings(id,seller_account_id),
  CHECK ((posting_resolution_status='RESOLVED')=(canonical_posting_id IS NOT NULL))
);

CREATE OR REPLACE FUNCTION enforce_canonical_account_health_scope() RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM collection_domain_runs dr WHERE dr.id=NEW.collection_domain_run_id AND dr.seller_account_id=NEW.seller_account_id AND dr.domain='ACCOUNT_HEALTH') THEN
    RAISE EXCEPTION 'canonical account health domain-run seller mismatch' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER account_health_metric_versions_scope BEFORE INSERT OR UPDATE ON account_health_metric_versions FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER account_health_snapshot_runs_scope BEFORE INSERT OR UPDATE ON account_health_snapshot_runs FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER account_rating_period_facts_scope BEFORE INSERT OR UPDATE ON account_rating_period_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER account_rating_thresholds_scope BEFORE INSERT OR UPDATE ON account_rating_threshold_observations FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER account_penalty_events_scope BEFORE INSERT OR UPDATE ON account_penalty_event_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER fbs_error_index_scope BEFORE INSERT OR UPDATE ON fbs_error_index_facts FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
CREATE TRIGGER fbs_posting_error_events_scope BEFORE INSERT OR UPDATE ON fbs_posting_error_events FOR EACH ROW EXECUTE FUNCTION enforce_canonical_account_health_scope();
