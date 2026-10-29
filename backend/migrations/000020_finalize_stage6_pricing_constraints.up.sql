ALTER TABLE pricing_constraint_rules
ADD COLUMN scope_target_kind TEXT;

ALTER TABLE pricing_constraint_rules
ADD CONSTRAINT chk_pricing_constraint_rules_scope_target_kind
CHECK (
    scope_target_kind IS NULL
    OR scope_target_kind IN ('category_id', 'sku', 'product_id', 'offer_id')
);

UPDATE pricing_constraint_rules
SET scope_target_kind = 'category_id'
WHERE scope_type = 'category_rule';

UPDATE pricing_constraint_rules
SET scope_target_kind = 'offer_id'
WHERE scope_type = 'sku_override'
  AND scope_target_code IS NOT NULL
  AND scope_target_kind IS NULL;

UPDATE pricing_constraint_rules
SET scope_target_kind = 'product_id'
WHERE scope_type = 'sku_override'
  AND scope_target_kind IS NULL;

ALTER TABLE pricing_constraint_rules
ALTER COLUMN reference_margin_percent TYPE NUMERIC(8,4);

ALTER TABLE sku_effective_constraints
ALTER COLUMN reference_margin_percent TYPE NUMERIC(8,4);

CREATE INDEX idx_pricing_constraint_rules_seller_scope_kind
    ON pricing_constraint_rules(seller_account_id, scope_type, scope_target_kind, is_active);
