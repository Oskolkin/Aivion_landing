## Stage 6 Pricing Constraints Validation

### 1) Global default only

1. `PUT /api/v1/pricing-constraints/global` with `min_price`, `max_price`, `reference_margin_percent`.
2. Recompute is triggered by API response path.
3. `GET /api/v1/pricing-constraints/effective?limit=20&offset=0` -> rows should use `resolved_from_scope_type=global_default`.

### 2) Category overrides global

1. Keep global default active.
2. Create category rule via `POST /api/v1/pricing-constraints/category-rules`.
3. Check products in that `description_category_id` in effective list.
4. For those products, `resolved_from_scope_type` should be `category_rule`.

### 3) SKU override overrides category/global

1. Keep global + category active.
2. Create SKU override by `sku` / `product_id` / `offer_id` via `POST /api/v1/pricing-constraints/sku-overrides`.
3. Re-read effective row for that item.
4. `resolved_from_scope_type` should be `sku_override`.

### 4) Preview formulas

Example:
- `reference_price=1000`
- `reference_margin_percent=0.25`
- `input_price=1200`

Expected:
- `implied_cost=750`
- `expected_margin_at_input_price=0.375`

Check via `POST /api/v1/pricing-constraints/preview`.

### 5) Delete/deactivate flow

1. Deactivate category rule: `POST /api/v1/pricing-constraints/category-rules/deactivate`.
2. Deactivate SKU override: `POST /api/v1/pricing-constraints/sku-overrides/deactivate`.
3. Recompute runs automatically.
4. Verify effective rows move to next precedence source (`category_rule` -> `global_default`, `sku_override` -> lower level).

### 6) Effective lookup

Validate all variants:
- `GET /api/v1/pricing-constraints/effective?sku=<sku>`
- `GET /api/v1/pricing-constraints/effective?product_id=<id>`
- `GET /api/v1/pricing-constraints/effective?limit=20&offset=0`

Confirm responses include source, prices, implied cost and computed timestamp.
