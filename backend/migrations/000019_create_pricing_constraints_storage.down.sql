DROP INDEX IF EXISTS idx_sku_effective_constraints_rule_id;
DROP INDEX IF EXISTS idx_sku_effective_constraints_seller_computed;
DROP INDEX IF EXISTS idx_sku_effective_constraints_seller_offer;
DROP INDEX IF EXISTS idx_sku_effective_constraints_seller_sku;

DROP TABLE IF EXISTS sku_effective_constraints;

DROP INDEX IF EXISTS idx_pricing_constraint_rules_scope_target_code;
DROP INDEX IF EXISTS idx_pricing_constraint_rules_scope_target_id;
DROP INDEX IF EXISTS idx_pricing_constraint_rules_seller_scope;
DROP INDEX IF EXISTS idx_pricing_constraint_rules_seller_account;

DROP TABLE IF EXISTS pricing_constraint_rules;
