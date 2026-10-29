# UI snapshots

Static HTML artifacts for review (no browser required).

| File | How to refresh |
|------|----------------|
| `funnel-ai-analysis-collapsed.html` | From `frontend/`: `npm run snapshot:funnel` |
| `funnel-ai-analysis-expanded.html` | Same command |

Snapshots use `renderToStaticMarkup` with the AI-first structured fixture (weakest card, bar comparisons, drop factor cards, directions, `details_sections` in expanded view).
