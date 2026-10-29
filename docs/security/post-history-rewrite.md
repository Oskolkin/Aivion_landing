# Post-history-rewrite local sync

After Git history rewrite and force-push, old local clones retain the leaked blob in their object database. Do not push from them.

## Preferred recovery for the project owner (Windows)

```bat
cd C:\Users\oskol
ren marketplace-ai-mvp marketplace-ai-mvp-before-history-rewrite
git clone <repository-url> marketplace-ai-mvp
cd marketplace-ai-mvp
git checkout main
```

Keep the renamed directory until the new clone is verified. Do **not** push from the old directory.

## Preferred recovery (Unix)

```bash
cd ..
mv marketplace-ai-mvp marketplace-ai-mvp-before-history-rewrite
git clone <repository-url> marketplace-ai-mvp
cd marketplace-ai-mvp
git checkout main
```

## If you have uncommitted / unpushed work

1. Do **not** push the old branch.
2. Export changes: `git format-patch origin/main --stdout > /tmp/my-work.patch` or `git diff > /tmp/my-work.diff`.
3. Fresh-clone as above.
4. Inspect the patch for secrets before applying.
5. Apply only needed changes onto a new branch from the rewritten history.
6. Push the new branch.

## Verification after fresh clone

```bash
git status --short
git log --all --full-history -- .env.local.bak-proxy   # expect empty
gitleaks detect --log-opts="--all"
```

## Warning

A normal `git push` of an old local branch can reintroduce the secret history into `origin`. Prefer fresh clones after rewrite.
