# Publication projections (Prompt №13)

Typed keys (not EAV widgets):

- `OVERVIEW` — exactly 3 KPIs: `ORDERED_AMOUNT`, `ORDERED_UNITS`, `AD_SHARE`. Missing ≠ Zero. No AOV in the KPI row. Ad Share uses sealed `GetAdvertisingSpendShare` (paid spend / Canonical Sales ordered amount). AI summary copied from validated Deep (no new LLM).
- `FUNNEL` — `READY` | `PARTIAL` | `UNAVAILABLE` (artifact always present)
- `STOCK` / `ADVERTISING` — findings/recs/action refs from the same candidate
- `ACTIONS_SNAPSHOT` — groups + singles by `display_order` (no priority/severity)
- `CHAT_CONTEXT` — compact index for future Chat (no chat AI)
- row sets: `STOCK_SKU_ROWS`, `AD_CAMPAIGN_ROWS`, `SKU_FOCUS_ROWS` via List*/pagination (no N+1)

Dashboard Categories / standalone SKU tabs are not projection keys.
