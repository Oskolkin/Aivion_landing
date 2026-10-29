# Canonical Factual API

The in-process `internal/factualapi` contract is the semantic firewall for new canonical business readers. Consumers receive typed entities and facts and never select revisions, inspect canonical tables, or call Ozon. Legacy Dashboard/Chat paths remain explicitly outside this migration stage.

Every call requires an immutable `FactualReadContext`. `LATEST_STABLE` freezes seller, analysis date, one clock-derived cutoff, policy versions, authoritative domain runs and historical-query capability assessments. Every versioned query applies `observed_at <= cutoff`; stable identities first observed later are invisible. `SEALED_UNIVERSE` (Prompt №8.1) loads a validated Factual Universe binding only: optional domain-run / assessment provenance may be nil/0 (never fabricated), and no latest capability/DQ/watermark re-resolution is allowed. Page tokens and FactRefs bind `ContextHash` + `UniverseID`. See `docs/current/factual-universe.md`.

### Fact-family bindings (Prompt №8.2)

Domain bindings are keyed by `FactFamilyKey{Domain, FactFamily}` — never overwritten by domain name alone. Lookup APIs:

- `Binding(domain, factFamily)` / `HasBinding` — exact pair
- `BindingsForDomain` / `AllBindings` — enumerate without collapsing
- `DomainBinding(domain)` — succeeds only when the domain has exactly one family; multi-family domains (e.g. `ADVERTISING.TRAFFIC` + `ADVERTISING.ATTRIBUTION`) return `ok=false`
- `UniqueDomainBinding` — same uniqueness rule, error on absent/ambiguous

Capability→family routing uses the versioned explicit registry `FACT_FAMILY_CAPABILITY_REGISTRY_V1` (`FactFamilyForCapability` / `RelatedCapabilityKeys`). No prefix or fuzzy matching: each capability key maps to at most one fact family; source state still comes from watermark/run evidence, not from picking a capability.

The v1 API implements Account, SKU/category, Order, Posting, Return, lifecycle-event, Account/SKU/category demand and daily series reads. Sales contains only `ordered_amount` and `ordered_units`. Category demand intersects each Sales business day with historical SKU assignment intervals; absent, post-dated or intra-day ambiguous mappings remain `UNMAPPED`.

`Value[T]` distinguishes known zero from unavailable, unknown, not-applicable and no-denominator states. `FactMeta` carries frozen readiness, period/as-of semantics, coverage, structured limitations and an opaque signed `FactRef`. `TraceFact` replays semantic provenance without returning payload bodies, PII or secrets.

Large lists use signed context/filter-bound keyset tokens. Batch SKU and demand reads are set-based and return per-item FOUND/NOT_FOUND/LIMITED results. There is no generic metric string, raw filter SQL, arbitrary aggregation, HTTP sidecar, Redis fact cache, AI or seller action.

The field-by-field cutoff review is recorded in
[`prompt-5-1-future-leak-audit.md`](prompt-5-1-future-leak-audit.md).

The `factual_api_v2` correctness policy applies day-level reconciliation to
every requested Sales period: EXPLAINED_GAP is PARTIAL, while missing,
UNEXPLAINED_GAP and NOT_COMPARABLE cannot be READY. Mixed currencies are never
reported as one Money aggregate. Category identifiers explicitly distinguish
SellerCategoryID, ProviderCategoryID and ParentProviderCategoryID. Posting and
Return lists use bounded set queries. TraceFact exposes persisted semantic
revision hashes and fails closed when deterministic result re-execution no
longer matches the signed FactRef hash.

`factual_api_v3` adds typed Stock, Price and Advertising reads. Stock exposes positions/snapshots/history and exact coverage using canonical Sales demand; ambiguous pools are excluded from known totals; missing-provider-response absences remain UNKNOWN with explicit reason codes. Price exposes current/as-of/history/index observations with exact Money and field-specific zero states. Advertising exposes campaigns, target intervals, campaign/day traffic, DRR, account paid spend, Sales-denominated spend share, and only direct SKU/CPO facts. Paid and bonus spend remain separate, attribution kinds remain labelled and are not cross-summed into one attributed amount, mixed currencies are not aggregated, and pending source semantics surface as structured limitations. Historical stock/price and CPO FactRefs re-execute the exact signed observation / latest revision as-known-at cutoff rather than a mutable latest projection.

Prompt №7 extends the same cutoff, capability, pagination and FactRef rules to Funnel periods, Finance summaries, Search queries, Promotions, Account Health ratings, and inbound Supply/SKU facts. Optional-domain errors produce typed limitations or unavailable values; they do not weaken tenant isolation or AS-KNOWN-AT selection.

Prompt №7.1 adds Search SKU-grain summary reads (`GetSKUSearchSummary`, `BatchGetSKUSearchSummary`, `ListSKUSearchSummary`, `GetSearchSummarySeries`), promotion membership/eligibility helpers (`GetSKUPromotions`, `GetPromotionMembershipHistory`, `GetPromotionEligibility`), and Supply acceptance/state-history surfaces (`GetSupplyAcceptance`, `ListSupplyAcceptanceItems`, `GetSupplyStateHistory`). Search seller summary is SKU×period (not legacy query-primary).

## Source provenance (Prompt №9.2)

Source identity fields are never aliased:

| Field | Meaning |
|-------|---------|
| `SourceContractKey` / `SourceContractVersion` / `SourceEndpointFamily` | Collection contract identity |
| `SourceContractHash` | `StableHash(key, version, endpoint)` — contract only |
| `SemanticMappingVersion` / `SemanticMappingHash` | Accepted semantic mapping revision |
| `SourceRevisionHash` | Provider/source revision (distinct from contract + mapping) |
| `WatermarkHash` | Frozen watermark state hash at seal |

`DeriveSourceContractHash` never reuses `SemanticMappingHash` or `SourceRevisionHash`. A syntactically valid SHA-256 does not prove semantic role.

SKU price history: `BatchGetSKUPriceLastObservedChange` exposes previous/current Money, absolute/percent change when valid, and `observed_at`. Supply: set-based reads include canonical `OVERDUE` state counts (open inbound ≠ overdue). Advertising: `ListSKUCampaignPerformance` returns direct SKU↔Campaign relationships only (no account-wide allocation).

## Factual API V4 (Prompt №9.6)

Current production policy is `factual_api_v5`. `factual_api_v4` / `factual_api_v3` remain legacy compatibility surfaces.

### Period completeness (Prompt №9.8)

Account Demand already counts distinct canonical `sales_account_daily` business dates and fail-closes when `days != requested period length` (`UNKNOWN` + `MISSING_DAILY_FACT`). That invariant is unchanged.

Category Demand completeness uses the **same source/account Sales days**, not the count of category rows. An incomplete seller period cannot be labelled a known 7-day Category total even if every observed day has category rows. A complete seller period with zero category demand rows is a **known zero**.

`Missing != Zero`. Partial sums are never returned as KNOWN full-period totals.

Traffic and Attribution are physically separate:

- separate fact families (`ADVERTISING.TRAFFIC` vs `ADVERTISING.ATTRIBUTION`)
- separate capabilities (`ADVERTISING.CAMPAIGN_TRAFFIC` / `ADVERTISING.SKU_TRAFFIC` vs `ADVERTISING.CAMPAIGN_ATTRIBUTION` / `ADVERTISING.SKU_ATTRIBUTION`)
- separate source-native periods
- separate readiness
- separate FactMeta provenance

V4 campaign methods:

- `BatchGetCampaignTrafficPerformance` — impressions, clicks, paid/bonus spend, CTR/CPC/CPM. CPC/CPM/CTR use `ValueNoDenominator` when the denominator is known zero, never a fabricated zero ratio.
- `BatchGetCampaignAttributionPerformance` — attributed amount/units and raw `ProviderDRRPct`. Calculated DRR is **not** in this DTO.
- `BatchGetCampaignTargetingSummary` — campaign-specific observed SKU performance in the Traffic period (not account-wide SKU ads).
- `BatchGetSKUAdvertisingTraffic` / `BatchGetSKUAdvertisingAttribution`

V3 mixed methods `BatchGetCampaignPeriodPerformance` and `BatchGetSKUAdvertising` remain for legacy callers only. V4 Landscape production must not call them.

Category Detail V4 is set-based:

- `BatchGetCategoryFunnelPeriod`
- `BatchGetCategorySearchDistribution` (stable mapping over the Search period, `nearest_rank_quantile_v1`)
- `BatchGetCategoryPromotionSummary` (snapshot/as-of; no fabricated cross-domain sales exposure)

Batch item status distinguishes `FOUND`, `FOUND_EMPTY`, and `NOT_FOUND`. Primary selected entities that return `NOT_FOUND` are integrity failures, not empty relationships.
