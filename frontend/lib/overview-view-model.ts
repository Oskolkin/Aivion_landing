import { DASHBOARD_ROUTES, actionsListHref, actionDetailHref } from "@/lib/dashboard-routes";
import { domainIcon } from "@/lib/domain-presentation";
import { formatDelta, formatMetric, kpiKind } from "@/lib/metric-format";
import type { FindingView, KPI, PublicationDashboardResponse } from "@/lib/publication-dashboard";
import { presentationFor } from "@/lib/action-presentation-registry";
import type { WhatChangedTone } from "@/lib/what-changed-view";

export const OVERVIEW_FOCUS_ACTIONS_LIMIT = 5;

export type OverviewPublicationIdentity = {
  publicationId?: string;
  metricDate: string;
};

export type OverviewPeriodControlsView = {
  groups: Array<{ label: string; value: string }>;
  analysisPeriod: string | null;
  comparisonPeriod: string | null;
  updatedAt: string | null;
  hasComparison: boolean;
};

export type OverviewKpiCardView = {
  key: string;
  label: string;
  valueDisplay: string;
  deltaText: string | null;
  deltaMode: "positive" | "negative" | "warning" | "neutral" | "missing";
  sparkline: Array<{ date: string; value?: number | null }>;
  hasSparkline: boolean;
  sparklineStroke: string;
  hint?: string | null;
  limited: boolean;
};

export type OverviewAiPointView = {
  id: string;
  title: string;
  detail?: string;
  category?: string;
};

export type OverviewAiSummaryView = {
  headline: string | null;
  points: OverviewAiPointView[];
  limited: boolean;
  limitedMessage?: string;
};

export type OverviewFocusActionView = {
  id: string;
  order: number;
  title: string;
  categoryLabel: string;
  href: string;
};

export type OverviewFocusActionsView = {
  actions: OverviewFocusActionView[];
  empty: boolean;
  emptyMessage: string;
  allActionsHref: string;
};

export type OverviewWhatChangedTabId = "all" | "worsened" | "new_risks" | "unchanged_risks" | "improved";

export type OverviewWhatChangedTabView = {
  id: OverviewWhatChangedTabId;
  label: string;
  count: number;
};

export type OverviewWhatChangedRowView = {
  id: string;
  title: string;
  context?: string;
  tone: WhatChangedTone;
  statusLabel?: string;
  href?: string;
};

export type OverviewWhatChangedView = {
  tabs: OverviewWhatChangedTabView[];
  rowsByTab: Record<OverviewWhatChangedTabId, OverviewWhatChangedRowView[]>;
  unavailable: boolean;
  unavailableMessage?: string;
};

export type OverviewFunnelStageView = {
  key: string;
  label: string;
  valueLabel: string;
  widthPercent: number;
  conversionLabel: string | null;
};

export type OverviewFunnelPreviewView = {
  conversionLabel: string;
  conversionValue: string | null;
  deltaText: string | null;
  deltaPositive: boolean | null;
  stages: OverviewFunnelStageView[];
  limited: boolean;
  limitedMessage?: string;
  href: string;
};

export type OverviewStockBucketView = {
  key: string;
  label: string;
  count: number;
  tone: "critical" | "warning" | "success" | "neutral" | "muted";
};

export type OverviewStockPreviewView = {
  summary: string;
  buckets: OverviewStockBucketView[];
  totalSkus: number | null;
  limited: boolean;
  limitedMessage?: string;
  href: string;
};

export type OverviewAdvertisingPreviewView = {
  weekMetricLabel: string;
  weekValueDisplay: string;
  weekDeltaText: string | null;
  weekDeltaTone: "warning" | "success" | "neutral";
  yesterdayMetricLabel: string;
  yesterdayValueDisplay: string;
  yesterdayDeltaText: string | null;
  yesterdayDeltaTone: "warning" | "success" | "neutral";
  yesterdayDate: string | null;
  previousDayDate: string | null;
  chartPoints: Array<{ date: string; value: number | null }>;
  hasChart: boolean;
  limited: boolean;
  limitedMessage?: string;
  href: string;
};

export type OverviewViewModel = {
  publication: OverviewPublicationIdentity;
  period: OverviewPeriodControlsView;
  kpis: OverviewKpiCardView[];
  aiSummary: OverviewAiSummaryView;
  focusActions: OverviewFocusActionsView;
  whatChanged: OverviewWhatChangedView;
  funnel: OverviewFunnelPreviewView;
  stock: OverviewStockPreviewView;
  advertising: OverviewAdvertisingPreviewView;
};

const KPI_LABELS: Record<string, string> = {
  ORDERED_AMOUNT: "Заказано на сумму",
  ORDERED_UNITS: "Заказано товаров",
  AD_SHARE: "Доля рекламы",
};

const BAND_LABEL: Record<string, string> = {
  out_of_stock: "Нет остатка",
  coverage_0_3: "0–3 дня",
  coverage_4_7: "4–7 дней",
  coverage_8_14: "8–14 дней",
};

const STAGE_LABEL: Record<string, string> = {
  PDP_IMPRESSIONS: "Показы карточки",
  PDP_CART_ADDS: "В корзину",
  SEARCH_IMPRESSIONS: "Показы в поиске",
  SEARCH_CART_ADDS: "В корзину из поиска",
};

export function buildOverviewPeriodControls(data: PublicationDashboardResponse): OverviewPeriodControlsView {
  const pub = data.publication;
  if (!pub) {
    return { groups: [], analysisPeriod: null, comparisonPeriod: null, updatedAt: null, hasComparison: false };
  }
  return {
    groups: [{ label: "Период анализа", value: pub.analysis_date }],
    analysisPeriod: pub.analysis_date,
    comparisonPeriod: "предыдущие 7 дней",
    updatedAt: pub.published_at.slice(0, 16).replace("T", " "),
    hasComparison: true,
  };
}

function mapKpi(k: KPI): OverviewKpiCardView {
  const kind = kpiKind(k.key, k.unit);
  const limited = k.status !== "READY";
  const sparkline = (k.trend ?? []).map((p) => ({
    date: p.date,
    value: p.amount != null ? Number(p.amount) : null,
  }));
  const deltaN = k.delta != null ? Number(k.delta) : NaN;
  let deltaMode: OverviewKpiCardView["deltaMode"] = "missing";
  if (limited) deltaMode = "missing";
  else if (!Number.isFinite(deltaN)) deltaMode = "neutral";
  else if (deltaN > 0) deltaMode = "positive";
  else if (deltaN < 0) deltaMode = "negative";
  else deltaMode = "neutral";
  return {
    key: k.key,
    label: KPI_LABELS[k.key] ?? k.key,
    valueDisplay: limited ? "—" : formatMetric(kind, k.amount ?? k.current, k.unit),
    deltaText: limited ? null : formatDelta(kind, k.delta) || null,
    deltaMode,
    sparkline,
    hasSparkline: sparkline.some((p) => p.value != null),
    sparklineStroke: "#0873f6",
    limited,
  };
}

function bucketTab(bucket: string): OverviewWhatChangedTabId | null {
  switch (bucket) {
    case "WORSENED":
      return "worsened";
    case "NEW_RISK":
      return "new_risks";
    case "ACTIVE_RISK":
      return "unchanged_risks";
    case "IMPROVED":
      return "improved";
    default:
      return null;
  }
}

function findingRow(f: FindingView, i: number): OverviewWhatChangedRowView {
  const tab = bucketTab(f.overview_bucket);
  const tone: WhatChangedTone =
    tab === "improved" ? "positive" : tab === "worsened" || tab === "new_risks" ? "negative" : "neutral";
  return { id: f.ref ?? String(i), title: f.title, context: f.statement, tone, statusLabel: f.overview_bucket };
}

export function buildOverviewViewModel(data: PublicationDashboardResponse): OverviewViewModel | null {
  if (data.state !== "READY" || !data.overview) return null;
  const ov = data.overview;
  const kpis = (ov.kpis ?? []).slice(0, 3).map(mapKpi);
  const takeaways = (ov.takeaways ?? []).slice(0, 3);
  const findings = ov.findings ?? [];
  const allRows = findings.map(findingRow);
  const byTab: Record<OverviewWhatChangedTabId, OverviewWhatChangedRowView[]> = {
    all: allRows,
    worsened: [],
    new_risks: [],
    unchanged_risks: [],
    improved: [],
  };
  findings.forEach((f, i) => {
    const tab = bucketTab(f.overview_bucket);
    if (tab) byTab[tab].push(findingRow(f, i));
  });
  const focus = (data.focus ?? []).slice(0, OVERVIEW_FOCUS_ACTIONS_LIMIT);
  const stockBands = data.stock?.coverage_bands ?? [];
  const funnelStages = (data.funnel?.stages ?? []).map((s, i) => ({
    key: s.key,
    label: STAGE_LABEL[s.key] ?? s.key,
    valueLabel: s.amount != null ? formatMetric("COUNT", s.amount) : "—",
    widthPercent: Math.max(40, 100 - i * 18),
    conversionLabel: null,
  }));
  const share = data.advertising?.account_ad_spend_share;
  const yshare = data.advertising?.yesterday_ad_spend_share;
  return {
    publication: { publicationId: data.publication?.id, metricDate: data.publication?.analysis_date ?? "" },
    period: buildOverviewPeriodControls(data),
    kpis,
    aiSummary: {
      headline: ov.executive_headline ?? ov.executive_summary,
      points: takeaways.map((t, i) => ({
        id: String(i),
        title: t.statement,
        category: t.primary_domain,
      })),
      limited: takeaways.length === 0 && !ov.executive_headline,
      limitedMessage: "Сводка появится после публикации анализа",
    },
    focusActions: {
      actions: focus.map((g, i) => ({
        id: g.action_view_id,
        order: i + 1,
        title: g.display_title,
        categoryLabel: presentationFor(g.action_type)?.label ?? g.family,
        href: actionDetailHref(g.action_view_id),
      })),
      empty: focus.length === 0,
      emptyMessage: "Действий сейчас нет",
      allActionsHref: actionsListHref(),
    },
    whatChanged: {
      tabs: [
        { id: "all", label: "Все", count: byTab.all.length },
        { id: "worsened", label: "Ухудшилось", count: byTab.worsened.length },
        { id: "new_risks", label: "Новые риски", count: byTab.new_risks.length },
        { id: "unchanged_risks", label: "Актуальные риски", count: byTab.unchanged_risks.length },
        { id: "improved", label: "Улучшилось", count: byTab.improved.length },
      ],
      rowsByTab: byTab,
      unavailable: findings.length === 0,
      unavailableMessage: "Изменения появятся после публикации анализа",
    },
    funnel: {
      conversionLabel: "Воронка",
      conversionValue: data.funnel?.state ?? null,
      deltaText: null,
      deltaPositive: null,
      stages: funnelStages,
      limited: data.funnel?.state !== "READY" && data.funnel?.state !== "PARTIAL",
      limitedMessage: data.funnel?.reason_code,
      href: DASHBOARD_ROUTES.funnel,
    },
    stock: {
      summary: "Покрытие остатков",
      buckets: stockBands.map((b) => ({
        key: b.key === "coverage_0_3" ? "critical_0_3" : b.key === "coverage_4_7" ? "shortage_4_7" : b.key === "coverage_8_14" ? "planning_8_14" : b.key,
        label: BAND_LABEL[b.key] ?? b.key,
        count: b.count,
        tone: b.key === "out_of_stock" ? "critical" : "neutral",
      })),
      totalSkus: stockBands.reduce((n, b) => n + b.count, 0),
      limited: data.stock?.state === "UNAVAILABLE",
      href: DASHBOARD_ROUTES.stock,
    },
    advertising: {
      weekMetricLabel: "ДРР кабинета (7 дней)",
      weekValueDisplay: share && share.status === "READY" ? formatMetric("RATIO", share.amount) : "—",
      weekDeltaText: share?.delta ? formatDelta("RATIO", share.delta) : null,
      weekDeltaTone: "neutral",
      yesterdayMetricLabel: "ДРР кабинета вчера",
      yesterdayValueDisplay: yshare && yshare.status === "READY" ? formatMetric("RATIO", yshare.amount) : "—",
      yesterdayDeltaText: null,
      yesterdayDeltaTone: "neutral",
      yesterdayDate: null,
      previousDayDate: null,
      chartPoints: [],
      hasChart: false,
      limited: !share || share.status !== "READY",
      limitedMessage: "Доля рекламы = paid spend / заказано на сумму",
      href: DASHBOARD_ROUTES.advertising,
    },
  };
}

export type OverviewHealthIndexView = {
  limited: boolean;
  limitedMessage?: string;
  scoreLabel?: string | null;
  scoreValue?: number | null;
  scorePercent: number;
  severityPillVariant: "success" | "info" | "warning" | "neutral" | "danger";
  severityLabel: string;
  factors: Array<{ id: string; category: string | null; title: string }>;
};

export { domainIcon };
