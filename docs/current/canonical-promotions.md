# Canonical Promotions

`PROMOTIONS` keeps provider promotion definitions, candidate eligibility, and observed SKU membership as distinct evidence. A candidate is not membership, and absence from a candidate response does not close authoritative membership.

Ozon and seller-created promotions retain their source kind, lifecycle state, validity interval, discount evidence, and canonical SKU resolution. Finalization is append-only and closes intervals only from an authoritative membership set.

## Membership and factual reads (Prompt 7.1)

Membership intervals use open/close **AS-KNOWN-AT** observation time (`known_from` / `known_to`). A **partial** participant collection does not close open intervals — closure requires a complete authoritative participant enumeration (including an authoritative empty set).

Factual API additions:

- `GetSKUPromotions` — promotions for a SKU as known at the context cutoff
- `GetPromotionMembershipHistory` — membership intervals (open/closed) for a promotion
- `GetPromotionEligibility` — candidate eligibility evidence (not membership)

The factual API lists promotions under a cutoff-bound `FactualReadContext`, preserving AS-KNOWN-AT state.

Readiness retains definition, membership, and eligibility family keys and adds source-specific Ozon/seller keys plus history. Promotion availability is optional to analysis-date selection.
