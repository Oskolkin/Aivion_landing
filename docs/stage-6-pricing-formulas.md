## Stage 6 Pricing Formulas

### Margin interpretation

- Margin is always defined as: `(price - cost) / price`
- Margin is stored as decimal fraction:
  - `0.25` means 25%
  - string percent format like `"25%"` is not used

### Core formulas

- Reference price at input moment: `P_ref`
- Input margin at current price: `M_ref`
- Implied cost:
  - `C_est = P_ref * (1 - M_ref)`
- Expected margin at new price `P_new`:
  - `ExpectedMargin(P_new) = (P_new - C_est) / P_new`

### Domain validation baseline

- `min_price <= max_price` (when both provided)
- `reference_price > 0`
- `implied_cost >= 0`
- `new_price > 0` for expected margin calculation
- margin range is validated in decimal fraction bounds
- division-by-zero is blocked via `new_price > 0` checks

### Explainability basis

For future resolver/effective constraints flow, domain structures explicitly carry:

- source scope type (`global_default`, `category_rule`, `sku_override`)
- rule source identity (`rule_id`, optional scope targets)
- reference inputs (`reference_price`, `reference_margin_percent`)
- computed output (`implied_cost`)
