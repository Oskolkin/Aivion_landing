import { asArray } from "@/lib/as-array";
import type { NullableArray } from "@/lib/dashboard-home-types";

/**
 * `data_quality_notes` mixes several unrelated seller-facing meanings under one
 * array. A historical/fixed-date analysis always carries a neutral
 * "Исторический анализ по данным на DD.MM.YYYY" note even when every widget is
 * fully rendered — that must never be presented as "part of the data is still
 * loading". This module classifies each note so the UI can pick an accurate
 * tone/title per kind instead of lumping everything into one loading banner.
 */
export type DashboardQualityNoteKind = "historical" | "stale" | "other";

const HISTORICAL_NOTE_PATTERN = /^Исторический анализ по данным на /;
const STALE_NOTE_PATTERN = /^Данные не обновлялись /;

export function classifyDashboardQualityNote(note: string): DashboardQualityNoteKind {
  const text = (note ?? "").trim();
  if (HISTORICAL_NOTE_PATTERN.test(text)) return "historical";
  if (STALE_NOTE_PATTERN.test(text)) return "stale";
  return "other";
}

export type ClassifiedDashboardQualityNotes = {
  historical: string[];
  stale: string[];
  other: string[];
};

export function classifyDashboardQualityNotes(
  notes: NullableArray<string>,
): ClassifiedDashboardQualityNotes {
  const result: ClassifiedDashboardQualityNotes = { historical: [], stale: [], other: [] };
  for (const raw of asArray(notes)) {
    const text = (raw ?? "").trim();
    if (!text) continue;
    result[classifyDashboardQualityNote(text)].push(text);
  }
  return result;
}

/** Ready Unified dashboards must never show a generic "loading" banner for
 * notes that only describe historical/stale source freshness.
 *
 * `analysis_source` is accepted for callers that want to log/branch on it,
 * but readiness itself is fully determined by `status`/`dashboard_content_status`
 * — the backend always sets `dashboard_content_status` to the final `status`
 * on the success path (see dashboardhome.Service), so a "ready" dashboard
 * sourced from Unified Analysis is already covered by that check alone.
 */
export function isReadyUnifiedDashboard(data: {
  status?: string;
  dashboard_content_status?: string;
  analysis_source?: string;
}): boolean {
  return data.status === "ready" || data.dashboard_content_status === "ready";
}

/**
 * Contract check for a published ready Unified dashboard. If the API incorrectly
 * returns ready while required analytical widgets are broken, the UI must show
 * an explicit technical error — never mask it with a loading/partial banner.
 */
export function findReadyUnifiedContractViolations(data: {
  analysis_source?: string;
  status?: string;
  dashboard_content_status?: string;
  ai_diagnostics?: {
    account_summary?: { health_score?: number | null; status?: string };
    what_changed?: { status?: string; groups?: unknown[] | null };
    category_summaries?: Array<{ category_score?: number | null; headline?: string | null } | null> | null;
    ad_efficiency?: {
      status?: string;
      ad_efficiency_narrative?: {
        status?: string;
        checklist?: Array<{ title?: string; text?: string } | null> | null;
      } | null;
    };
  };
  sku_focus?: {
    order_drop?: Array<{ sparkline_7d?: Array<{ has_data?: boolean } | null> | null } | null> | null;
    key_skus?: Array<{ revenue_trend_7d?: unknown[] | null; sparkline_7d?: unknown[] | null } | null> | null;
  };
}): string[] {
  if (data.analysis_source !== "unified" || !isReadyUnifiedDashboard(data)) {
    return [];
  }
  const violations: string[] = [];
  const health = data.ai_diagnostics?.account_summary?.health_score;
  if (health == null || Number.isNaN(Number(health))) {
    violations.push("missing_health_score");
  }
  const wc = data.ai_diagnostics?.what_changed;
  if (!wc || wc.status !== "available") {
    violations.push("what_changed_not_available");
  } else if (!Array.isArray(wc.groups) || wc.groups.length === 0) {
    violations.push("what_changed_groups_empty");
  }
  const cats = Array.isArray(data.ai_diagnostics?.category_summaries)
    ? data.ai_diagnostics!.category_summaries!
    : [];
  if (cats.length === 0) {
    violations.push("missing_category_summaries");
  } else {
    for (const card of cats) {
      if (card == null || card.category_score == null || Number.isNaN(Number(card.category_score))) {
        violations.push("incomplete_category_summary");
        break;
      }
      if (!(card.headline ?? "").trim()) {
        violations.push("incomplete_category_summary");
        break;
      }
    }
  }
  const adsStatus = data.ai_diagnostics?.ad_efficiency?.status;
  const adsReady = !adsStatus || adsStatus === "available" || adsStatus === "ok";
  if (adsReady) {
    const checklist = Array.isArray(data.ai_diagnostics?.ad_efficiency?.ad_efficiency_narrative?.checklist)
      ? data.ai_diagnostics!.ad_efficiency!.ad_efficiency_narrative!.checklist!
      : [];
    const usable = checklist.filter((item) => item && (item.title ?? "").trim() && (item.text ?? "").trim());
    if (usable.length < 2) {
      violations.push("missing_advertising_checklist");
    }
  }
  const orderDrop = Array.isArray(data.sku_focus?.order_drop) ? data.sku_focus!.order_drop! : [];
  for (const card of orderDrop) {
    const series = Array.isArray(card?.sparkline_7d) ? card!.sparkline_7d! : [];
    const points = series.filter((p) => p && p.has_data).length;
    if (points < 2) {
      violations.push("missing_sku_trend");
      break;
    }
  }
  return violations;
}

export type DashboardQualityBanner = {
  key: DashboardQualityNoteKind;
  tone: "info" | "warning";
  title: string;
  message: string;
};

/**
 * Builds the ordered set of banners to render for a given notes array.
 *
 * For ready Unified dashboards, notes are split by kind so a purely
 * historical/stale note never renders under a "part of the data is still
 * loading" title. For other dashboard states we keep the previous
 * conservative behaviour: any note is surfaced as a single loading warning,
 * since partial/legacy states may genuinely still be loading data.
 */
export function buildDashboardQualityBanners(
  notes: NullableArray<string>,
  options: { readyUnified: boolean },
): DashboardQualityBanner[] {
  const allNotes = asArray(notes)
    .map((note) => (note ?? "").trim())
    .filter(Boolean);
  if (allNotes.length === 0) return [];

  if (!options.readyUnified) {
    return [
      {
        key: "other",
        tone: "warning",
        title: "Часть данных ещё загружается",
        message: allNotes.join(" "),
      },
    ];
  }

  // Stale/freshness notes stay in API metadata for support/monitoring, but
  // never surface as a seller-facing warning banner (including fixed historical mode).
  const { historical, other } = classifyDashboardQualityNotes(notes);
  const banners: DashboardQualityBanner[] = [];

  if (other.length > 0) {
    banners.push({
      key: "other",
      tone: "warning",
      title: "Часть данных ещё загружается",
      message: other.join(" "),
    });
  }

  if (historical.length > 0) {
    banners.push({
      key: "historical",
      tone: "info",
      title: "Исторический анализ",
      message: historical.join(" "),
    });
  }

  return banners;
}
