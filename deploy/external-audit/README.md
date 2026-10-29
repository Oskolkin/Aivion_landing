# External audit static bundle tooling

Runtime capture/publish helpers for temporary auditor links under `/external-audit/<bundle-id>/`.

## Commands (VPS)

```bash
sudo bash /opt/aivion/repo/deploy/scripts/create-external-audit.sh --seller-id=1 --ttl-hours=24
sudo bash /opt/aivion/repo/deploy/scripts/refresh-external-audit.sh <bundle-id>
sudo bash /opt/aivion/repo/deploy/scripts/validate-external-audit.sh <bundle-id>
sudo bash /opt/aivion/repo/deploy/scripts/delete-external-audit.sh <bundle-id>
```

URL is written to `/etc/aivion/secrets/external_audit_url` (mode 0600) and must never be committed.

## Local unit tests

```bash
cd deploy/external-audit && npm ci && npm test
```

Capture requires Playwright browsers and the demo password file; it is not run in CI.
