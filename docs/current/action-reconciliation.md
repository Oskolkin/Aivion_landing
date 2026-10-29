# Action Reconciliation (Prompt №13)

Status: **implemented** — Stage 14 `PLAN_ACTION_RECONCILIATION` produces an immutable plan only (no Action Case SQL, no Publication insert, no OpenAI/Ozon).

Package: `internal/analysis/actionreconcile`.

## Inputs

Accepted `DEEP_VALIDATED_OUTPUT`, Deep Action Menu, Continuity Context, `base_publication_id`, persisted signature snapshot (when present), Action Catalog, versioned policies.

## Plan invariants

- `deep_action_proposal_count == disposition_count`
- every previous atomic Action has exactly one Deep assessment outcome
- JOIN+LEAVE on the same SKU+promotion is Stage 14 FAIL
- duplicate identical signatures among new proposals FAIL
- backend does **not** invent `REQUIRED_SUPPORT` from Action Type

## Dispositions

`VISIBLE_NEW`, `REAFFIRMED_EXISTING`, `SUPPRESSED_USER_DISMISSED`, `SUPPRESSED_RECURRENCE_COOLDOWN` (cooldown unused while recurrence policy is `DISABLED_UNTIL_CALIBRATED`).

## Identity

`action_identity_policy_v1`: seller + action_type + target + normalized_action_context + intent_signature → canonical JSON → SHA-256.

Normalized context: campaign_ref for bid/participation; promotion_ref for join/leave/adjust promotion price; empty for ordinary SKU price.

Intent: PRICE INCREASE/DECREASE; JOIN/LEAVE; catalog/campaign state keys; etc.

## Recurrence

User DISMISSED = 30 **calendar** days on the same signature only.

System recurrence: `action_recurrence_policy_v1` + `DISABLED_UNTIL_CALIBRATED` (no invented cooldown days).
