# Pre-E2E hardening — MVP alignment

## Alert generation

| Type / group | Pipeline | Seller UI |
|--------------|----------|-----------|
| `sku_orders_drop` | **Active** | Просадка заказов |
| `sales_orders_drop`, stock, advertising alerts | **Active** | As labeled |
| `sku_revenue_drop` | **Legacy** (not generated) | «Устаревший тип» if shown |
| `price_*`, `margin_*`, `missing_pricing_constraints_*` | **Disabled** | Hidden from filters; legacy → «Устаревший тип» |
| `price_economics` group | **Disabled** | Not in seller group filter |

`RunForAccount` runs sales, stock, advertising only. `RunPriceEconomicsAlerts` returns empty when disabled.

## Recommendations (AI)

| Type | AI prompt / validator | Seller UI |
|------|----------------------|-----------|
| `investigate_sales_drop`, `replenish_sku`, ad/stock types, `discount_overstock`, … | **Active** | Normal labels |
| `review_price_margin`, `review_price_floor` | **Legacy** (reject on generate) | «Устаревший тип» |

## Dashboard risk buckets

Mutually exclusive keys: `stock`, `surplus`, `advertising`, `sales`, `critical_sku`, `other`.  
`fbo_surplus_storage_risk` → `surplus` only; `stock_critical_sku_low_stock` → `critical_sku` only.

## AI daily summaries (dev)

Production: worker task `ai.generate_daily_summaries` after post-sync recalc.  
Local demo: `go run ./cmd/dev-generate-ai-summaries --seller-account-id=1` from `backend/` (`OPENAI_API_KEY` required). See `docs/dev-seed-mvp.md` §2d.

## Migrations

None added for this hardening pass.
