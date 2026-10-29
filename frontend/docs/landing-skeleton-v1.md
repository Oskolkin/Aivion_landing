# Aivion Landing Skeleton v1

Status: desktop interactive prototype / product visuals are placeholders.

## Normative basis

The implementation follows the approved landing Content Contract plus the later UX Skeleton v1 and Visual Wireframe v1. The current prototype intentionally does **not** lock Canonical Demo Account values, final product screens, pricing, customer proof, legal links, lead backend, or mobile behavior.

## Public routing

- `/` — public Aivion landing.
- `/login` — existing authentication entry point.
- `/app/*` — existing authenticated product area, still protected by `app/app/layout.tsx`.

The former `/ -> /login` redirect was removed so the marketing page can live at the public root.

## Implemented

- Header with anchors and one Demo conversion intent.
- Approved Hero content and Aivion brand system.
- Large marketing product placeholder.
- Scroll-driven Product Flow with five states.
- Positioning section: existing analytics tools -> Aivion analytical work layer.
- Second scroll-driven AI Chat scene with four use-case states.
- Actions split composition.
- Dashboard tab showcase: Overview / Funnel / Stocks / Advertising placeholders.
- Audience + business-effect block.
- Ozon connection/trust block.
- Demo block and local prototype modal.
- FAQ accordion with eight approved questions.
- Final CTA and Footer skeleton.
- `prefers-reduced-motion` fallback.

## Intentionally not implemented yet

- Canonical Demo Account and real metrics/SKUs/campaigns.
- Final target product screenshots/projections.
- Real XLSX export content.
- Real lead submission / CRM integration.
- Final legal/privacy links and wording.
- Pricing.
- Customer proof.
- Mobile/responsive redesign.

## Local run

From `frontend/`:

```bash
npm ci
npm run dev
```

Then open `http://localhost:3000`.

Production validation before merge should include:

```bash
npm run lint
npm run build
```

If dependencies are already installed and match `package-lock.json`, `npm run dev` is enough for the first visual review.

## Hero visual polish v1.1

The first screen was refined after browser review against the approved hero mockup:

- exact user-provided Aivion logo lockup is used on the public Header and marketing product frame;
- redundant AI/Ozon eyebrow was removed;
- Hero proof line is one calm text row instead of four outlined pills;
- product frame is larger and appears earlier in the first viewport;
- branded geometric planes derived from the Aivion mark were added as the hero visual motif;
- demo KPI values and explanatory AI-summary copy are explicitly illustrative marketing content and are not yet the Canonical Demo Account;
- product semantics remain aligned with the approved model: three Overview KPI slots and no synthetic HIGH/MEDIUM/LOW priority scoring.

## Demo modal visual polish v1.2

The Demo conversion modal was refined against the approved modal mockup:

- exact Aivion logo lockup is shown at the top of the modal;
- the generic `DEMO` eyebrow and the separate `30 минут` badge were removed;
- visible modal copy now uses `Демо` in Russian;
- a short reassurance line makes it explicit that nothing needs to be connected or prepared in advance;
- the optional company marker is aligned on the same row as the field label;
- input typography, spacing, close control, overlay blur and card proportions were refined;
- the primary button uses the Aivion blue-to-cyan gradient and includes the forward arrow;
- the legal note is shorter and styled as a visible privacy-policy link placeholder;
- a subtle geometric Aivion motif is used in the modal background;
- the local success state now uses user-facing copy rather than prototype/production implementation language.

The form is still local-only in this prototype. Lead creation, CRM delivery, the final privacy route and approved legal wording remain follow-up work before production release.

## Product Flow visual polish v1.3

The Product Flow section was updated to match the approved visual mock more closely while preserving the canonical Aivion product semantics:

- headline and explanatory copy are now a single left-aligned narrative block;
- the five-step rail is closer to the product stage and includes concise descriptions;
- the product proof is larger, denser and visually dominant;
- AI summary and "Focus today" receive stronger visual emphasis than KPI context;
- cool blue/cyan Aivion geometry and atmospheric depth connect the section to the Hero identity;
- the existing five-state scroll story remains unchanged: Overview → situation → Action → Chat → result.

## Product Flow + Demo modal viewport hardening v1.4

Two browser-visible layout defects were corrected after desktop review:

- Product Flow intro (`КАК ЭТО РАБОТАЕТ`, H2 and subtitle) now scrolls normally and is no longer part of the pinned viewport;
- only the five-step rail and the product stage are sticky during the 1→5 story;
- sticky geometry accounts for the 78px public Header and scales to the available `svh` height;
- all five timeline steps remain visible while the right-hand product screen is kept fully inside the desktop viewport;
- lower desktop heights receive reduced scene padding rather than clipping the stage;
- Demo modal close control is explicitly positioned in the top-right corner and no longer participates in normal layout flow;
- desktop modal no longer has internal horizontal or vertical scrollbars and the page behind it is scroll-locked while open;
- a compact-height desktop variant keeps the full form visible on 768px-high screens;
- mobile retains vertical modal scrolling only when the physical screen is too short, while horizontal overflow remains disabled.


## Positioning visual polish v1.5

The “Почему Aivion” section now uses the approved three-part semantic structure: existing data tools on the left, the analytical-work gap in the center, and the work Aivion performs on the right. The background is a calm near-white data field with local blue/cyan atmospheric glows and only a faint cropped brand geometry at the periphery; no visual arrows imply data ingestion from MPSTATS/Excel/BI into Aivion.
