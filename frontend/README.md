# Frontend

## Local development

Use **`npm run dev`**, not `npm start`.

| Command | Purpose |
|---------|---------|
| `npm run dev` | Development server with hot reload |
| `npm run dev:clean` | Delete `.next`, then `next dev` |
| `npm run build` + `npm start` | Production bundle (rebuild after every code change) |

1. Copy env: `copy .env.local.example .env.local` (API must match backend port, default **18081**).
2. From repo root, start API: `scripts/dev/start-backend.ps1`.
3. Here: `npm run dev:clean` or `scripts/dev/start-frontend.ps1`.
4. Open http://localhost:3000/dev/system — must show `http://localhost:18081`.
5. Dashboard funnel block: search DOM for `funnel-ai-skeleton-v3`.

Full guide: [../docs/local-dev-windows.md](../docs/local-dev-windows.md).

## Scripts

```bash
npm run test
npm run lint
npm run build
npm run snapshot:funnel
```
