-- Prompt №9 Materiality Engine registry + run/selection persistence.
-- Lane identity is (lane_key, entity_kind): the same lane_key may exist for SKU and Category.

CREATE TABLE materiality_lane_definitions (
    lane_key TEXT NOT NULL,
    entity_kind TEXT NOT NULL,
    domain TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lane_key, entity_kind),
    CHECK (entity_kind IN ('ACCOUNT', 'CATEGORY', 'SKU', 'CAMPAIGN')),
    CHECK (lane_key <> ''),
    CHECK (domain <> '')
);

CREATE TABLE materiality_lane_versions (
    lane_key TEXT NOT NULL,
    entity_kind TEXT NOT NULL,
    lane_version TEXT NOT NULL,
    ranking_semantic_key TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('ASC', 'DESC')),
    required_capability TEXT NOT NULL,
    allowed_readiness JSONB NOT NULL DEFAULT '["READY"]'::jsonb,
    exposure_policy_key TEXT NOT NULL,
    candidate_limit INTEGER NOT NULL CHECK (candidate_limit > 0),
    weight DOUBLE PRECISION NOT NULL CHECK (weight > 0),
    typed_impl_key TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    config_hash TEXT NOT NULL CHECK (config_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lane_key, entity_kind, lane_version),
    FOREIGN KEY (lane_key, entity_kind)
        REFERENCES materiality_lane_definitions (lane_key, entity_kind) ON DELETE RESTRICT,
    CHECK (typed_impl_key <> ''),
    CHECK (ranking_semantic_key <> '')
);

CREATE TABLE materiality_policy_versions (
    version TEXT PRIMARY KEY,
    policy_hash TEXT NOT NULL CHECK (policy_hash ~ '^[0-9a-f]{64}$'),
    policy_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE materiality_budget_profiles (
    version TEXT PRIMARY KEY,
    budget_hash TEXT NOT NULL CHECK (budget_hash ~ '^[0-9a-f]{64}$'),
    budget_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE materiality_runs (
    id BIGSERIAL PRIMARY KEY,
    analysis_run_id BIGINT NOT NULL REFERENCES analysis_runs (id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts (id) ON DELETE CASCADE,
    factual_universe_id UUID NOT NULL,
    factual_universe_manifest_hash TEXT NOT NULL CHECK (factual_universe_manifest_hash ~ '^[0-9a-f]{64}$'),
    landscape_core_profile_version TEXT NOT NULL,
    input_landscape_core_hash TEXT NOT NULL CHECK (input_landscape_core_hash ~ '^[0-9a-f]{64}$'),
    materiality_policy_version TEXT NOT NULL REFERENCES materiality_policy_versions (version) ON DELETE RESTRICT,
    materiality_policy_hash TEXT NOT NULL CHECK (materiality_policy_hash ~ '^[0-9a-f]{64}$'),
    budget_profile_version TEXT NOT NULL REFERENCES materiality_budget_profiles (version) ON DELETE RESTRICT,
    budget_profile_hash TEXT NOT NULL CHECK (budget_profile_hash ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED')),
    ranking_hash TEXT NULL CHECK (ranking_hash IS NULL OR ranking_hash ~ '^[0-9a-f]{64}$'),
    ranking_artifact_id BIGINT NULL REFERENCES analysis_artifacts (id) ON DELETE RESTRICT,
    selection_hash TEXT NULL CHECK (selection_hash IS NULL OR selection_hash ~ '^[0-9a-f]{64}$'),
    selection_artifact_id BIGINT NULL REFERENCES analysis_artifacts (id) ON DELETE RESTRICT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    UNIQUE (analysis_run_id),
    UNIQUE (id, analysis_run_id),
    FOREIGN KEY (factual_universe_id, seller_account_id)
        REFERENCES factual_universes (id, seller_account_id) ON DELETE RESTRICT,
    CHECK (completed_at IS NULL OR completed_at >= started_at)
);

CREATE INDEX idx_materiality_runs_seller ON materiality_runs (seller_account_id, started_at DESC);
CREATE INDEX idx_materiality_runs_universe ON materiality_runs (factual_universe_id);

CREATE TABLE materiality_selections (
    materiality_run_id BIGINT NOT NULL REFERENCES materiality_runs (id) ON DELETE CASCADE,
    entity_kind TEXT NOT NULL,
    semantic_entity_key TEXT NOT NULL,
    canonical_ref_id TEXT NULL,
    selection_order INTEGER NOT NULL CHECK (selection_order > 0),
    selected_as_anchor BOOLEAN NOT NULL DEFAULT FALSE,
    audit_reason_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    audit_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (materiality_run_id, entity_kind, semantic_entity_key),
    CHECK (entity_kind IN ('ACCOUNT', 'CATEGORY', 'SKU', 'CAMPAIGN')),
    CHECK (semantic_entity_key <> '')
);

CREATE INDEX idx_materiality_selections_order
    ON materiality_selections (materiality_run_id, entity_kind, selection_order);

-- Seed lane definitions (SKU / Category / Campaign). ACCOUNT has no lanes.
INSERT INTO materiality_lane_definitions (lane_key, entity_kind, domain) VALUES
    ('SALES_CURRENT_EXPOSURE', 'SKU', 'SALES'),
    ('SALES_30D_EXPOSURE', 'SKU', 'SALES'),
    ('SALES_ABSOLUTE_INCREASE', 'SKU', 'SALES'),
    ('SALES_ABSOLUTE_DECREASE', 'SKU', 'SALES'),
    ('SALES_RELATIVE_INCREASE', 'SKU', 'SALES'),
    ('SALES_RELATIVE_DECREASE', 'SKU', 'SALES'),
    ('SALES_FROM_ZERO', 'SKU', 'SALES'),
    ('SALES_TO_ZERO', 'SKU', 'SALES'),
    ('ZERO_SELLABLE_WITH_RECENT_DEMAND', 'SKU', 'STOCK'),
    ('STOCK_COVERAGE_ASC', 'SKU', 'STOCK'),
    ('AD_PAID_SPEND_EXPOSURE', 'SKU', 'ADVERTISING'),
    ('AD_PAID_SPEND_INCREASE', 'SKU', 'ADVERTISING'),
    ('AD_PAID_SPEND_DECREASE', 'SKU', 'ADVERTISING'),
    ('PRICE_ABSOLUTE_CHANGE', 'SKU', 'PRICE'),
    ('SEARCH_POSITION_CHANGE', 'SKU', 'SEARCH'),
    ('SUPPLY_OVERDUE_EXPOSURE', 'SKU', 'SUPPLY'),
    ('CURRENT_SALES_EXPOSURE', 'CATEGORY', 'SALES'),
    ('SALES_ABSOLUTE_INCREASE', 'CATEGORY', 'SALES'),
    ('SALES_ABSOLUTE_DECREASE', 'CATEGORY', 'SALES'),
    ('SALES_RELATIVE_CHANGE', 'CATEGORY', 'SALES'),
    ('ZERO_STOCK_SALES_EXPOSURE', 'CATEGORY', 'STOCK'),
    ('AD_PAID_SPEND_EXPOSURE', 'CATEGORY', 'ADVERTISING'),
    ('AD_PAID_SPEND_CHANGE', 'CATEGORY', 'ADVERTISING'),
    ('SUPPLY_OVERDUE_SALES_EXPOSURE', 'CATEGORY', 'SUPPLY'),
    ('PAID_SPEND_EXPOSURE', 'CAMPAIGN', 'ADVERTISING'),
    ('PAID_SPEND_INCREASE', 'CAMPAIGN', 'ADVERTISING'),
    ('PAID_SPEND_DECREASE', 'CAMPAIGN', 'ADVERTISING'),
    ('ATTRIBUTED_AMOUNT_EXPOSURE', 'CAMPAIGN', 'ADVERTISING'),
    ('ATTRIBUTED_AMOUNT_INCREASE', 'CAMPAIGN', 'ADVERTISING'),
    ('ATTRIBUTED_AMOUNT_DECREASE', 'CAMPAIGN', 'ADVERTISING'),
    ('DRR_ABSOLUTE_PP_CHANGE', 'CAMPAIGN', 'ADVERTISING'),
    ('CTR_ABSOLUTE_PP_CHANGE', 'CAMPAIGN', 'ADVERTISING'),
    ('PAID_SPEND_FROM_ZERO', 'CAMPAIGN', 'ADVERTISING'),
    ('PAID_SPEND_TO_ZERO', 'CAMPAIGN', 'ADVERTISING');

-- Placeholder config_hash (64 hex zeros) for seed rows; runtime CurrentPolicy().Hash() is authoritative.
-- Lane versions are seeded for registry presence; application code owns typed impl + live policy hash.
INSERT INTO materiality_lane_versions (
    lane_key, entity_kind, lane_version, ranking_semantic_key, direction,
    required_capability, allowed_readiness, exposure_policy_key, candidate_limit, weight,
    typed_impl_key, policy_version, config_hash
) VALUES
    ('SALES_CURRENT_EXPOSURE', 'SKU', 'v1', 'SALES_CURRENT_EXPOSURE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'sku.sales_current_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_30D_EXPOSURE', 'SKU', 'v1', 'SALES_30D_EXPOSURE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'sku.sales_30d_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_ABSOLUTE_INCREASE', 'SKU', 'v1', 'SALES_ABSOLUTE_INCREASE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'sku.sales_abs_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_ABSOLUTE_DECREASE', 'SKU', 'v1', 'SALES_ABSOLUTE_DECREASE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'sku.sales_abs_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_RELATIVE_INCREASE', 'SKU', 'v1', 'SALES_RELATIVE_INCREASE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'RELATIVE_SALES', 40, 1.0, 'sku.sales_rel_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_RELATIVE_DECREASE', 'SKU', 'v1', 'SALES_RELATIVE_DECREASE_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'RELATIVE_SALES', 40, 1.0, 'sku.sales_rel_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_FROM_ZERO', 'SKU', 'v1', 'SALES_FROM_ZERO_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'sku.sales_from_zero', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_TO_ZERO', 'SKU', 'v1', 'SALES_TO_ZERO_V1', 'DESC', 'SALES.SKU_DEMAND', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'sku.sales_to_zero', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('ZERO_SELLABLE_WITH_RECENT_DEMAND', 'SKU', 'v1', 'ZERO_SELLABLE_WITH_RECENT_DEMAND_V1', 'DESC', 'STOCK.SKU_CURRENT', '["READY"]', 'DEMAND_POSITIVE_SALES', 40, 1.0, 'sku.zero_sellable_recent_demand', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('STOCK_COVERAGE_ASC', 'SKU', 'v1', 'STOCK_COVERAGE_ASC_V1', 'ASC', 'STOCK.COVERAGE_7D', '["READY"]', 'DEMAND_POSITIVE_SALES', 40, 1.0, 'sku.stock_coverage_asc', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('AD_PAID_SPEND_EXPOSURE', 'SKU', 'v1', 'AD_PAID_SPEND_EXPOSURE_V1', 'DESC', 'ADVERTISING.SKU_TRAFFIC', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'sku.ad_paid_spend_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('AD_PAID_SPEND_INCREASE', 'SKU', 'v1', 'AD_PAID_SPEND_INCREASE_V1', 'DESC', 'ADVERTISING.SKU_TRAFFIC', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'sku.ad_paid_spend_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('AD_PAID_SPEND_DECREASE', 'SKU', 'v1', 'AD_PAID_SPEND_DECREASE_V1', 'DESC', 'ADVERTISING.SKU_TRAFFIC', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'sku.ad_paid_spend_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PRICE_ABSOLUTE_CHANGE', 'SKU', 'v1', 'PRICE_ABSOLUTE_CHANGE_V1', 'DESC', 'PRICE.CUSTOMER_PRICE', '["READY"]', 'PRICE_WITH_SALES', 40, 1.0, 'sku.price_abs_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SEARCH_POSITION_CHANGE', 'SKU', 'v1', 'SEARCH_POSITION_CHANGE_V1', 'DESC', 'SEARCH.SELLER_SUMMARY', '["READY","PARTIAL"]', 'SEARCH_WITH_SALES', 40, 1.0, 'sku.search_position_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SUPPLY_OVERDUE_EXPOSURE', 'SKU', 'v1', 'SUPPLY_OVERDUE_EXPOSURE_V1', 'DESC', 'SUPPLY.STATE', '["READY","PARTIAL"]', 'SUPPLY_OVERDUE_SALES', 40, 1.0, 'sku.supply_overdue_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('CURRENT_SALES_EXPOSURE', 'CATEGORY', 'v1', 'CURRENT_SALES_EXPOSURE_V1', 'DESC', 'SALES.CATEGORY_DEMAND', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'cat.current_sales_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_ABSOLUTE_INCREASE', 'CATEGORY', 'v1', 'SALES_ABSOLUTE_INCREASE_V1', 'DESC', 'SALES.CATEGORY_DEMAND', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'cat.sales_abs_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_ABSOLUTE_DECREASE', 'CATEGORY', 'v1', 'SALES_ABSOLUTE_DECREASE_V1', 'DESC', 'SALES.CATEGORY_DEMAND', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'cat.sales_abs_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SALES_RELATIVE_CHANGE', 'CATEGORY', 'v1', 'SALES_RELATIVE_CHANGE_V1', 'DESC', 'SALES.CATEGORY_DEMAND', '["READY","PARTIAL"]', 'RELATIVE_SALES', 40, 1.0, 'cat.sales_rel_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('ZERO_STOCK_SALES_EXPOSURE', 'CATEGORY', 'v1', 'ZERO_STOCK_SALES_EXPOSURE_V1', 'DESC', 'STOCK.SKU_CURRENT', '["READY"]', 'DEMAND_POSITIVE_SALES', 40, 1.0, 'cat.zero_stock_sales_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('AD_PAID_SPEND_EXPOSURE', 'CATEGORY', 'v1', 'AD_PAID_SPEND_EXPOSURE_V1', 'DESC', 'ADVERTISING.SKU_TRAFFIC', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'cat.ad_paid_spend_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('AD_PAID_SPEND_CHANGE', 'CATEGORY', 'v1', 'AD_PAID_SPEND_CHANGE_V1', 'DESC', 'ADVERTISING.SKU_TRAFFIC', '["READY","PARTIAL"]', 'RELATIVE_AD_SPEND', 40, 1.0, 'cat.ad_paid_spend_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('SUPPLY_OVERDUE_SALES_EXPOSURE', 'CATEGORY', 'v1', 'SUPPLY_OVERDUE_SALES_EXPOSURE_V1', 'DESC', 'SUPPLY.STATE', '["READY","PARTIAL"]', 'SUPPLY_OVERDUE_SALES', 40, 1.0, 'cat.supply_overdue_sales_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PAID_SPEND_EXPOSURE', 'CAMPAIGN', 'v1', 'PAID_SPEND_EXPOSURE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'camp.paid_spend_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PAID_SPEND_INCREASE', 'CAMPAIGN', 'v1', 'PAID_SPEND_INCREASE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'camp.paid_spend_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PAID_SPEND_DECREASE', 'CAMPAIGN', 'v1', 'PAID_SPEND_DECREASE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'camp.paid_spend_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('ATTRIBUTED_AMOUNT_EXPOSURE', 'CAMPAIGN', 'v1', 'ATTRIBUTED_AMOUNT_EXPOSURE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_ATTRIBUTION', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'camp.attr_amount_exposure', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('ATTRIBUTED_AMOUNT_INCREASE', 'CAMPAIGN', 'v1', 'ATTRIBUTED_AMOUNT_INCREASE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_ATTRIBUTION', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'camp.attr_amount_increase', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('ATTRIBUTED_AMOUNT_DECREASE', 'CAMPAIGN', 'v1', 'ATTRIBUTED_AMOUNT_DECREASE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_ATTRIBUTION', '["READY","PARTIAL"]', 'ABSOLUTE_SALES', 40, 1.0, 'camp.attr_amount_decrease', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('DRR_ABSOLUTE_PP_CHANGE', 'CAMPAIGN', 'v1', 'DRR_ABSOLUTE_PP_CHANGE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_DRR', '["READY","PARTIAL"]', 'CAMPAIGN_METRIC_MIN_SPEND', 40, 1.0, 'camp.drr_abs_pp_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('CTR_ABSOLUTE_PP_CHANGE', 'CAMPAIGN', 'v1', 'CTR_ABSOLUTE_PP_CHANGE_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'CAMPAIGN_METRIC_MIN_SPEND', 40, 1.0, 'camp.ctr_abs_pp_change', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PAID_SPEND_FROM_ZERO', 'CAMPAIGN', 'v1', 'PAID_SPEND_FROM_ZERO_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'camp.paid_spend_from_zero', 'MATERIALITY_POLICY_V1', repeat('0', 64)),
    ('PAID_SPEND_TO_ZERO', 'CAMPAIGN', 'v1', 'PAID_SPEND_TO_ZERO_V1', 'DESC', 'ADVERTISING.CAMPAIGN_TRAFFIC', '["READY","PARTIAL"]', 'NONE', 40, 1.0, 'camp.paid_spend_to_zero', 'MATERIALITY_POLICY_V1', repeat('0', 64));

-- Policy / budget JSON is a structural seed; live hash must match CurrentPolicy()/DiscoveryStandardBudget() in Go.
-- Inserts use a fixed placeholder hash; application upserts authoritative hashes on first use if needed.
INSERT INTO materiality_policy_versions (version, policy_hash, policy_json) VALUES (
    'MATERIALITY_POLICY_V1',
    repeat('0', 64),
    jsonb_build_object(
        'version', 'MATERIALITY_POLICY_V1',
        'rrf_k', 60,
        'anchor_coverage_target', 0.70,
        'exposure_floors', jsonb_build_object(
            'relative_share_floor', 0.005,
            'absolute_min_sales', 1000,
            'absolute_min_ad_spend', 500
        ),
        'continuity_profile', 'CONTINUITY_INITIAL_NO_BASE_V1'
    )
);

INSERT INTO materiality_budget_profiles (version, budget_hash, budget_json) VALUES (
    'DISCOVERY_STANDARD_V1',
    repeat('0', 64),
    jsonb_build_object(
        'version', 'DISCOVERY_STANDARD_V1',
        'max_categories', 15,
        'max_skus', 40,
        'max_campaigns', 15,
        'anchor_max_categories', 8,
        'anchor_max_skus', 20,
        'anchor_max_campaigns', 8
    )
);
