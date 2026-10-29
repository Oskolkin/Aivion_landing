# Canonical Finance

`FINANCE` is an append-only ledger of Ozon accrual evidence. Provider day totals are authoritative for account net accrual; child operation rows explain a parent and must not be added to it when computing account net.

Collection uses seller-bound private OAuth and retains exact decimal amounts, currencies, provider operation types, posting links, SKU/category attribution, settlement evidence, and balance snapshots. Unknown operation types remain stored and degrade only affected semantic capabilities.

## Day revisions (Prompt 7.1)

Day revisions support **A→B→A**: uniqueness is `(seller_account_id, business_date, revision_no)`, not `state_hash`. Application-level latest-hash dedupe skips identical successive days (A→A); returning to a prior semantic day after a change still appends a new revision.

`AccountNet` sums parent `total_amount_signed` only — never parent amounts plus breakdown children.

Equal provider rows keep distinct multiplicity via `row_ordinal` (and participate in the day semantic hash). Source fragments, payload ids, `revision_no`, and `observed_at` are excluded from `state_hash`.

The factual API provides cutoff-bound account summaries and breakdowns. Latest revisions are selected AS-KNOWN-AT; later corrections do not change an existing `FactualReadContext`.

Readiness covers daily ledger, account net, breakdown, dictionary and balance families plus account-net-accrual, type, SKU/category direct, posting detail, settlement reconciliation, and history keys. Finance does not control the Catalog + Sales analysis date.
