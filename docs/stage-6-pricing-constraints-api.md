## Stage 6 Pricing Constraints API

Minimal backend API surface:

- `GET /api/v1/pricing-constraints`
- `PUT /api/v1/pricing-constraints/global`
- `POST /api/v1/pricing-constraints/category-rules`
- `POST /api/v1/pricing-constraints/sku-overrides`
- `GET /api/v1/pricing-constraints/effective`
- `POST /api/v1/pricing-constraints/preview`

Notes:

- all endpoints are scoped to current authenticated seller account
- upsert endpoints trigger account-level effective constraints recompute
- effective endpoint reads from `sku_effective_constraints` storage layer
- preview endpoint uses domain formulas via pricing constraints service
