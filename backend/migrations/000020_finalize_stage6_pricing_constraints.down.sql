DROP INDEX IF EXISTS idx_pricing_constraint_rules_seller_scope_kind;

ALTER TABLE sku_effective_constraints
ALTER COLUMN reference_margin_percent TYPE NUMERIC(8,2);

ALTER TABLE pricing_constraint_rules
ALTER COLUMN reference_margin_percent TYPE NUMERIC(8,2);

ALTER TABLE pricing_constraint_rules
DROP CONSTRAINT IF EXISTS chk_pricing_constraint_rules_scope_target_kind;

ALTER TABLE pricing_constraint_rules
DROP COLUMN IF EXISTS scope_target_kind;
