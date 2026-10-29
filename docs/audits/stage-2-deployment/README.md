# Stage 2 deployment evidence index

Evidence for Stage 2 lives primarily on the server after deployment:

- `/opt/aivion/releases/` — release metadata, SHA, digests
- `/opt/aivion/backups/` — postgres/minio/config backups
- `/opt/aivion/backups/evidence/` — smoke/acceptance notes (no secrets)

## Repository checkpoint (pre-merge)

| Item | Value |
|------|--------|
| Branch | `cursor/stage-2-production-deployment` |
| Commit | `a5628ddd02fcdfa993d23b6cf17b2c75f198e277` |
| PR | https://github.com/Oskolkin/Aivion/pull/4 |
| Base | `main` @ `41bec42d8240b8364939e4a2448abca71deaedff` |

### GitHub Actions (PR #4) — all SUCCESS

| Workflow | Run ID |
|----------|--------|
| Backend CI | 30852259584 / 30852246590 |
| Frontend CI | 30852259678 / 30852246610 |
| Docker CI | 30852259687 / 30852246734 |
| Secret Scan | 30852259682 / 30852246664 |

### Accepted exception (do not hide)

- Old compromised OpenAI key not yet revoked by owner decision.
- `security_release_gate = PASS_WITH_ACCEPTED_EXCEPTION`
- `real_seller_onboarding_gate = BLOCKED_UNTIL_OLD_KEY_REVOKED`

### Not done yet (requires owner approval)

- Merge to `main`
- Server bootstrap
- GHCR publish of merged SHA
- Staging deployment / acceptance
- Production promotion of `app.aivion.pro`
