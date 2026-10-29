# Deferred OpenAI API key rotation

```text
status = DEFERRED_BY_OWNER
severity = P0_SECURITY_EXCEPTION
owner = project owner
required_before = real seller production onboarding
key_rotation_status = DEFERRED_BY_OWNER
owner_risk_acceptance = true
rotation_required_before_real_seller_production = true
old_key_revoked = false
security_incident_fully_closed = false
```

## Context

A previous commit history contained `.env.local.bak-proxy` with a leaked OpenAI API key (mask: `sk-proj-...zpsA`).

Git history cleanup removes the secret from the repository. **It does not revoke the key at OpenAI.**

The project owner explicitly deferred key rotation for this cleanup pass.

## Warning

> Очистка Git history не делает ранее опубликованный ключ безопасным. Ключ должен быть отозван отдельно.

## Required future actions (owner)

1. Check usage of the old key in OpenAI Platform.
2. Revoke the old key.
3. Create a new project-scoped key.
4. Place the new key outside Git (secret store / CI secrets / local env only).
5. Confirm the old key returns 401/403.
6. Confirm the new key succeeds for required calls.
7. Review logs and costs for unexpected usage.
8. Close this exception with separate evidence (`old_key_revoked = true`).

## Non-claims

Do **not** treat history rewrite or repository cleanliness as:

- `old_key_revoked = true`
- `security_incident_fully_closed = true`
- unconditional `security_repository_gate = PASS`

Until revocation is done, gates remain:

- `security_repository_gate = PASS_WITH_ACCEPTED_EXCEPTION`
- `repository_release_gate = CONDITIONAL_PASS`
- real seller production onboarding remains blocked
