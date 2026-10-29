import {
  buildDashboardQualityBanners,
  classifyDashboardQualityNote,
  classifyDashboardQualityNotes,
  findReadyUnifiedContractViolations,
  isReadyUnifiedDashboard,
} from "@/lib/dashboard-quality-notes";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const HISTORICAL_NOTE = "Исторический анализ по данным на 08.07.2026";
const STALE_NOTE = "Данные не обновлялись 12 дн.";
const OTHER_NOTE = "Не удалось загрузить опубликованный анализ.";

// Lightweight checks runnable via: npx tsx lib/dashboard-quality-notes.test.ts
function run() {
  assert(classifyDashboardQualityNote(HISTORICAL_NOTE) === "historical", "historical note classified");
  assert(classifyDashboardQualityNote(STALE_NOTE) === "stale", "stale note classified");
  assert(classifyDashboardQualityNote(OTHER_NOTE) === "other", "unknown note classified as other");
  assert(classifyDashboardQualityNote("") === "other", "empty note classified as other");

  const classified = classifyDashboardQualityNotes([HISTORICAL_NOTE, STALE_NOTE, OTHER_NOTE, "", "  "]);
  assert(classified.historical.length === 1, "one historical note grouped");
  assert(classified.stale.length === 1, "one stale note grouped");
  assert(classified.other.length === 1, "one other note grouped");

  assert(isReadyUnifiedDashboard({ status: "ready" }), "status ready => ready unified");
  assert(
    isReadyUnifiedDashboard({ dashboard_content_status: "ready" }),
    "dashboard_content_status ready => ready unified",
  );
  assert(
    isReadyUnifiedDashboard({ status: "ready", analysis_source: "unified" }),
    "ready status with unified source stays ready unified",
  );
  assert(
    !isReadyUnifiedDashboard({ analysis_source: "unified", status: "partial" }),
    "unified source alone without ready status is not ready unified",
  );
  assert(!isReadyUnifiedDashboard({ status: "not_enough_data" }), "not_enough_data => not ready unified");
  assert(!isReadyUnifiedDashboard({ status: "partial" }), "partial legacy => not ready unified");

  const historicalOnly = buildDashboardQualityBanners([HISTORICAL_NOTE], { readyUnified: true });
  assert(historicalOnly.length === 1, "historical-only note yields exactly one banner");
  assert(historicalOnly[0].key === "historical", "historical banner key");
  assert(historicalOnly[0].tone === "info", "historical banner is informational, not a warning");
  assert(historicalOnly[0].title === "Исторический анализ", "historical banner uses neutral title");
  assert(!historicalOnly[0].title.includes("ещё загружается"), "historical banner never uses loading title");
  assert(historicalOnly[0].message === HISTORICAL_NOTE, "historical banner message preserves note text");

  const staleOnly = buildDashboardQualityBanners([STALE_NOTE], { readyUnified: true });
  assert(staleOnly.length === 0, "stale-only note never yields a seller-facing banner");
  assert(
    !staleOnly.some((b) => b.key === "stale" || b.title.includes("Источник данных")),
    "stale banner title never appears",
  );

  const mixed = buildDashboardQualityBanners([HISTORICAL_NOTE, STALE_NOTE, OTHER_NOTE], {
    readyUnified: true,
  });
  assert(mixed.length === 2, "mixed notes omit stale and keep historical+other");
  assert(mixed.every((b) => b.key !== "stale"), "stale key absent from banners");
  assert(mixed.find((b) => b.key === "other")?.title === "Часть данных ещё загружается", "other banner keeps loading title");
  assert(mixed.every((b) => b.message.length > 0), "every banner has a message");
  assert(
    !mixed.some((b) => (b.message ?? "").includes("не обновлялись")),
    "stale note text never resurfaces under another banner",
  );

  const emptyBanners = buildDashboardQualityBanners([], { readyUnified: true });
  assert(emptyBanners.length === 0, "no notes => no banners");

  const nullBanners = buildDashboardQualityBanners(null, { readyUnified: true });
  assert(nullBanners.length === 0, "null notes => no banners");

  const legacy = buildDashboardQualityBanners([HISTORICAL_NOTE, OTHER_NOTE], { readyUnified: false });
  assert(legacy.length === 1, "legacy state collapses notes into one banner");
  assert(legacy[0].title === "Часть данных ещё загружается", "legacy banner keeps historical loading title");
  assert(legacy[0].message.includes(HISTORICAL_NOTE), "legacy banner message includes all notes");
  assert(legacy[0].message.includes(OTHER_NOTE), "legacy banner message includes all notes");

  assert(
    findReadyUnifiedContractViolations({
      analysis_source: "unified",
      status: "ready",
      ai_diagnostics: {
        account_summary: { health_score: 70 },
        what_changed: { status: "available", groups: [{ key: "worsened" }] },
        category_summaries: [{ category_score: 7, headline: "Category ok" }],
        ad_efficiency: {
          status: "available",
          ad_efficiency_narrative: {
            status: "available",
            checklist: [
              { title: "Pause high DRR", text: "Pause campaigns over 100% DRR." },
              { title: "Reallocate budget", text: "Move spend to converting campaigns." },
            ],
          },
        },
      },
      sku_focus: {
        order_drop: [
          {
            sparkline_7d: [
              { has_data: true },
              { has_data: true },
            ],
          },
        ],
      },
    }).length === 0,
    "complete ready unified has no violations",
  );
  assert(
    findReadyUnifiedContractViolations({
      analysis_source: "unified",
      status: "ready",
      ai_diagnostics: {
        account_summary: {},
        what_changed: { status: "source_not_available", groups: [] },
      },
    }).includes("missing_health_score"),
    "missing health_score is a contract violation",
  );
  assert(
    findReadyUnifiedContractViolations({
      analysis_source: "unified",
      status: "ready",
      ai_diagnostics: {
        account_summary: { health_score: 55 },
        what_changed: { status: "source_not_available", groups: [] },
      },
    }).includes("what_changed_not_available"),
    "broken what_changed is a contract violation",
  );

  console.log("dashboard-quality-notes.test.ts: ok");
}

run();
