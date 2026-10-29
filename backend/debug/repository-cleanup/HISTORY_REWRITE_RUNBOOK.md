# Git history rewrite — owner runbook

## Preconditions

- [x] Bundle backup: `C:/Users/oskol/aivion-before-secret-cleanup.bundle` (verified)
- [ ] Team notified; no concurrent pushes
- [ ] OpenAI key `sk-proj-...zpsA` revoked (or revoke immediately after rewrite starts)
- [ ] `gh auth login` / push credentials available
- [ ] Confirm protected-branch rules allow temporary force-push to `main` (or temporarily relax)

## Branches known to contain `.env.local.bak-proxy`

- `main` / `origin/main`
- `cursor/dashboard-demo-readiness` / `origin/cursor/dashboard-demo-readiness`
- `cursor/security-repository-cleanup` (local)

No tags currently present in the backup inventory.

## Commands

```bash
cd /path/to/Aivion
git fetch --all --prune

# Optional: refresh bundle
git bundle create ../aivion-before-secret-cleanup-$(date +%Y%m%d).bundle --all

git filter-repo --path .env.local.bak-proxy --invert-paths

# filter-repo removes origin; re-add
git remote add origin https://github.com/Oskolkin/Aivion.git

gitleaks detect --source=. --config=.gitleaks.toml --log-opts="--all" --redact
test -z "$(git log --all --full-history -- .env.local.bak-proxy)"

git push --force-with-lease origin --all
# if tags ever exist: git push --force-with-lease origin --tags
```

## After push

1. Each developer: fresh clone (preferred) or `reset --hard origin/main`
2. Rotate OpenAI / any real DB credentials from the leaked file
3. Confirm `secret-scan` workflow green on new `main` SHA
4. Invalidate any GitHub Actions caches if they might contain checkout artifacts of the old blob (usually N/A for this path-only leak)
