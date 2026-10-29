CREATE TABLE pricing_constraint_rules (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL CHECK (scope_type IN ('global_default', 'category_rule', 'sku_override')),
    scope_target_id BIGINT,
    scope_target_code TEXT,
    min_price NUMERIC(18,2),
    max_price NUMERIC(18,2),
    reference_margin_percent NUMERIC(8,2),
    reference_price NUMERIC(18,2),
    implied_cost NUMERIC(18,2),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_pricing_constraint_rules_price_range
        CHECK (
            min_price IS NULL
            OR max_price IS NULL
            OR min_price <= max_price
        )
);

CREATE INDEX idx_pricing_constraint_rules_seller_account
    ON pricing_constraint_rules(seller_account_id);

CREATE INDEX idx_pricing_constraint_rules_seller_scope
    ON pricing_constraint_rules(seller_account_id, scope_type, is_active);

CREATE INDEX idx_pricing_constraint_rules_scope_target_id
    ON pricing_constraint_rules(seller_account_id, scope_target_id);

CREATE INDEX idx_pricing_constraint_rules_scope_target_code
    ON pricing_constraint_rules(seller_account_id, scope_target_code);

CREATE TABLE sku_effective_constraints (
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    ozon_product_id BIGINT NOT NULL,
    sku BIGINT,
    offer_id TEXT,
    resolved_from_scope_type TEXT NOT NULL CHECK (resolved_from_scope_type IN ('global_default', 'category_rule', 'sku_override')),
    rule_id BIGINT NOT NULL REFERENCES pricing_constraint_rules(id) ON DELETE RESTRICT,
    effective_min_price NUMERIC(18,2),
    effective_max_price NUMERIC(18,2),
    reference_price NUMERIC(18,2),
    reference_margin_percent NUMERIC(8,2),
    implied_cost NUMERIC(18,2),
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (seller_account_id, ozon_product_id),
    CONSTRAINT chk_sku_effective_constraints_price_range
        CHECK (
            effective_min_price IS NULL
            OR effective_max_price IS NULL
            OR effective_min_price <= effective_max_price
        )
);

CREATE INDEX idx_sku_effective_constraints_seller_sku
    ON sku_effective_constraints(seller_account_id, sku);

CREATE INDEX idx_sku_effective_constraints_seller_offer
    ON sku_effective_constraints(seller_account_id, offer_id);

CREATE INDEX idx_sku_effective_constraints_seller_computed
    ON sku_effective_constraints(seller_account_id, computed_at DESC);

CREATE INDEX idx_sku_effective_constraints_rule_id
    ON sku_effective_constraints(rule_id);
