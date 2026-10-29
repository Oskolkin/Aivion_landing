# Action Cases (Prompt №13 + 13.1)

New tables (not `action_items` / `action_generation_runs`):

- `action_signatures` / `action_signature_state`
- `action_cases` / `action_case_state` (system_state and user_state are separate)
- `action_case_proposals` / `action_proposal_supports` (`revalidation_json` on supports — migration `000092`)
- `action_case_assessments` / `action_support_assessments` — written on **Publish** from Deep `PreviousOutcomes` (not inferred from support statuses)
- `action_user_events` / `action_system_events`

DONE closes the occurrence; a later FULLY_SUPPORTED Deep opens a **new** occurrence immediately (no 30-day suppress).

DISMISSED sets `suppressed_until = event_time + 30 calendar days` on the signature. Opposite intent is a different signature.

Service primitive: `Repository.ApplyUserEvent` (HTTP is Prompt №14).

Groups are publication-local projections (`publication_action_groups` + members), not Cases.
