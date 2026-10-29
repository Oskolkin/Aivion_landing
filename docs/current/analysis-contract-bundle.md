# Analysis Contract Bundle

Status: **Prompt №13** — `analysis_orchestration_policy_v7`. Deep four fields bumped for Support Basis (output/prompt/validation/pack schema v2). Action Reconciliation / Publication / Projection / PublicationValidation / Publish / PostPublish are composite seals (not `NOT_IMPLEMENTED`). `BundleVersionV1` unchanged.

Immutable versioned seal (`contract_bundle_v1`) covering:

- orchestration / temporal / capability / DQ / factual API
- universe + resolved context manifests
- Prompt №8.2: `fact_family_binding`, `watermark_policy`, `capability_registry` (NAME#hash seals)
- sales/stock/price/advertising maturity & attribution policies
- Prompt №7 canonical registries: funnel metrics, finance types, search collection policy, promotions semantic registry, account health metrics, supply state mapping, advertising attribution semantics
- Prompt №9 seals: `continuity_contract` = `CONTINUITY_INITIAL_NO_BASE_V1`, `landscape_core` / `landscape_detail` profile seals, `materiality` = NAME#hash Materiality policy seal
- Prompt №10.1 Discovery composite seals: `discovery_pack`, `discovery_prompt`, `discovery_output`, `discovery_validation` (pack/menu/budget/request/validation versions bumped; output schema + prompt/model/attempt policy unchanged)
- Prompt №12 Deep composite seals: `deep_pack`, `deep_prompt`, `deep_output`, `deep_validation` (Action Catalog/Menu versions are included in the Deep pack composite, not as extra bundle slots)
- explicit composite seals for Action Reconciliation / Publication / Projection / Publish / PostPublish

Persisted in `analysis_contract_bundles` keyed by SHA-256 of canonical JSON. Each Analysis Run freezes `contract_bundle_hash` and must not re-read “latest policy” per stage.

HMAC signing secrets are configuration only and never enter the Contract Bundle hash.

Changing any included registry/policy version changes the bundle hash and therefore invalidates NO_OP against an older analytical contract for the same Universe.
