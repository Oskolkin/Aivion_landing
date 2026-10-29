# UI primitives

Lightweight, Tailwind-based foundation. Compose with `cn` and `class-variance-authority`.
Import directly via the `@/components/ui/*` alias.

## Components

- **`InfoNote`** (`info-note.tsx`) — explanations, analysis limitations, informational / warning notes.
  Variants: `neutral` | `info` | `warning`. Renders a leading lucide icon.
- **`EmptyNote`** (`empty-note.tsx`) — compact empty / placeholder states inside cards and sections.
  Variants: `neutral` | `dashed` | `soft`. Short text, optional icon, no large title.
- **`EmptyState`** (`empty-state.tsx`) — large, centered empty states for full panels / screens
  (`title` required, `message` optional). Do **not** use inside dense cards — use `EmptyNote` there.
- **`StatusPill`** (`status-pill.tsx`) — the visual primitive for status, severity, evidence, state.
  cva-based, variants: `success` | `info` | `warning` | `neutral` | `danger`; sizes `sm` | `xs`.
  This is the **only** shared `StatusPill`.
- **`Badge`** (`badge.tsx`) — generic chip / tag, **not** a status. Neutral labels, metric chips, counts.
- **`StatusBadge`** (`status-badge.tsx`) — business mapper that turns a raw domain status / severity
  into a localized label + color. Keep as-is; do **not** mechanically replace it with `StatusPill`.
- **`MetricBlock`** (`metric-block.tsx`) — `label` (small uppercase, muted) + `value` inside analytical cards.
- **`SectionHeader`** (`page-header.tsx`) — page/dashboard section header (`h2`, `text-lg`) with optional
  subtitle and actions.
- **`SectionHeading`** (`section-heading.tsx`) — small in-card section title with optional description.
- **`cn`** (`cn.ts`) — the single helper for className composition (`clsx` + `tailwind-merge`).

## Conventions

- **Icons:** use [`lucide-react`](https://lucide.dev) SVG icons. Never use unicode glyphs
  (e.g. `ⓘ`, `⚠`, `✓`) for notes / statuses / icons.
- **Status vs chip vs mapper:** `StatusPill` = visual status/severity/evidence/state primitive;
  `Badge` = neutral chip/tag (never a status); `StatusBadge` = business mapper for domain statuses.
- **Empty states:** `EmptyNote` for compact in-card placeholders; `EmptyState` for large empty panels.
- **Palette:** keep the existing slate / `aivion-*` palette; no new bright colors, heavy shadows, or animations.
