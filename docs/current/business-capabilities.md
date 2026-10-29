# Business capability assessments

A capability describes a business-semantic read ability, not an endpoint or subscription. The v3 registry covers Catalog, Sales Demand, lifecycle, Stock, Price and Advertising. Prompt №6 adds Stock current/location/history/coverage, Price core/customer/history/index, and Advertising campaign/traffic/attribution/DRR capabilities.

Each append-only assessment separates contract, access, source, semantic, coverage, temporal, history and population dimensions before deriving `READY`, `PARTIAL`, `STALE`, `UNAVAILABLE`, `FAILED` or `NOT_APPLICABLE`. `CURRENT_COLLECTION` and `HISTORICAL_QUERY` are independent: a failed latest collection does not erase previously accepted historical truth.

A complete empty Catalog has a READY SKU-universe capability with population `EMPTY`; Sales capabilities become not applicable. Pending non-empty real-seller semantics remain `PENDING_ACCEPTANCE`, so automated fixtures cannot silently confer production validation.

`capability_policy_v2` derives current access from persisted OAuth connection
state and exact machine-readable collection error codes. Unknown causes remain
UNKNOWN; forbidden is not guessed to mean subscription loss. Historical-query
access remains independent from a later current-collection auth/source failure.

Prompt №6 / 6.1 optional domains require seller-bound `PRIVATE_OAUTH` for current collection. Stock pool independence, Price field semantics and Advertising attribution/report-family mappings may remain `PENDING_ACCEPTANCE`; that state degrades only the affected capability. Optional-domain absence or failure does not block the Catalog + Sales analysis-date resolver. Direct SKU/CPO advertising capability is never inferred from campaign totals, attribution kinds are not cross-aggregated, and account attribution DRR remains partial until an aggregation contract is accepted.

Prompt №7 adds Funnel, Finance, Search, Promotions, Account Health and Supply families plus their minimum granular keys. Granular assessments inherit the closest implemented family evidence (for example Search position from query details and Supply route from bundle evidence), while retaining their own capability identity and semantic state. These domains remain optional to the Catalog + Sales analysis-date resolver.

Prompt №7.1: `SEARCH.SELLER_SUMMARY` is independent of `SEARCH.QUERY_DETAILS` (and Premium market search). Supply core capabilities (order universe, current state, planned content, route, timeslot, history) are independent of optional `SUPPLY.ACCEPTANCE_*` beta capabilities.
