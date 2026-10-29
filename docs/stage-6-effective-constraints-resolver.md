## Stage 6 Effective Constraints Resolver

### Precedence

Resolver applies strict priority:

1. `sku_override`
2. `category_rule`
3. `global_default`
4. if nothing found: explicit "constraints not found" result

### Resolver input

- `seller_account_id`
- product identity (`ozon_product_id`, optional `sku`, optional `offer_id`)
- `description_category_id`
- current `reference_price`

### Resolver output

- `effective_min_price`
- `effective_max_price`
- `effective_margin`
- `effective_implied_cost`
- `resolved_from_scope_type`
- `rule_id`
- explainability basis:
  - `reference_price`
  - `reference_margin_percent`
  - `implied_cost`
- preview helper for expected margin at any selected `new_price`

### Formulas used

- `C_est = P_ref * (1 - M_ref)`
- `ExpectedMargin(P_new) = (P_new - C_est) / P_new`
- margin semantics: `(price - cost) / price`, decimal fraction (e.g. `0.25`)

### No-rule behavior

If no matching active rule exists on all precedence levels, resolver returns a
normal explicit result with `has_constraints = false` (not an internal error).

### Service-level usage foundation

`pricingconstraints.Service` uses resolver as central precedence engine and adds:

- upsert methods for `global_default`, `category_rule`, `sku_override`
- account-level recompute:
  - read products for seller account
  - resolve effective constraints per product
  - materialize found constraints into `sku_effective_constraints`
- preview methods:
  - implied cost preview
  - expected margin preview
