import { sanitizeDashboardSummaryText } from "@/lib/account-summary-display";
import type { StatusPillVariant } from "@/components/ui/status-pill";
import type { DashboardHomeResponse, RiskDriver, TopActionItem } from "@/lib/dashboard-home-types";
import { isInappropriateImprovementSummary, sanitizeSellerFacingText } from "@/lib/seller-facing-text";

const scoreFormatter = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
});

const DRIVER_LIMIT = 4;

export type HealthSeverityTone = "critical" | "high" | "medium" | "attention" | "low" | "unknown";

export type HealthDriverIconCategory =
  | "advertising"
  | "stock"
  | "funnel"
  | "sales"
  | "cancellations"
  | "risk"
  | "neutral";

export type HealthDriverView = {
  title: string;
  reason: string;
  iconCategory: HealthDriverIconCategory;
};

export type AccountHealthDisplayInput = {
  accountSummary: DashboardHomeResponse["ai_diagnostics"]["account_summary"];
  whatChanged: DashboardHomeResponse["ai_diagnostics"]["what_changed"];
  stockCoverage: DashboardHomeResponse["ai_diagnostics"]["stock_coverage"];
  adEfficiency: DashboardHomeResponse["ai_diagnostics"]["ad_efficiency"];
  topActions: TopActionItem[];
};

export type HealthImprovementItemView = {
  title: string;
  explanation: string;
  factorLabel?: string;
  actionHref?: string;
  expectedDirection?: string;
};

export type AccountHealthDisplayView = {
  scoreLabel: string | null;
  scorePercent: number;
  severityLabel: string;
  severityTone: HealthSeverityTone;
  severityPillVariant: StatusPillVariant;
  deltaText: string;
  drivers: HealthDriverView[];
  /** @deprecated use improvementIntro + improvementItems */
  improvementSummary: string;
  improvementIntro: string;
  improvementItems: HealthImprovementItemView[];
};

export function formatHealthScore(score?: number | null): string | null {
  const clamped = clampHealthScore(score);
  if (clamped == null) {
    return null;
  }
  return `${scoreFormatter.format(clamped)} / 100`;
}

export function getHealthSeverityLabel(severity?: string | null, statusLabel?: string | null): string {
  const fromBackend = (statusLabel ?? "").trim();
  if (fromBackend) {
    return fromBackend;
  }
  switch ((severity ?? "").trim().toLowerCase()) {
    case "critical":
      return "Критическое состояние";
    case "high":
    case "warning":
    case "risk":
      return "Зона риска";
    case "medium":
      return "Требует внимания";
    case "attention":
      return "Есть зоны внимания";
    case "low":
    case "stable":
      return "Стабильное состояние";
    default:
      return "Статус не определён";
  }
}

export function getHealthSeverityTone(severity?: string | null): HealthSeverityTone {
  switch ((severity ?? "").trim().toLowerCase()) {
    case "critical":
      return "critical";
    case "high":
    case "warning":
    case "risk":
      return "high";
    case "medium":
      return "medium";
    case "attention":
      return "attention";
    case "low":
    case "stable":
      return "low";
    default:
      return "unknown";
  }
}

export function getHealthSeverityPillVariant(severity?: string | null): StatusPillVariant {
  switch (getHealthSeverityTone(severity)) {
    case "critical":
    case "high":
      return "danger";
    case "medium":
      return "warning";
    case "attention":
      return "info";
    case "low":
      return "success";
    default:
      return "neutral";
  }
}

export function getHealthScorePercent(score?: number | null): number {
  return clampHealthScore(score) ?? 0;
}

export function buildHealthDeltaText(delta?: number | null): string {
  if (!Number.isFinite(delta)) {
    return "Динамика появится после следующей сводки.";
  }
  if (delta === 0) {
    return "Без изменений к прошлой сводке.";
  }
  return `${delta! > 0 ? "+" : ""}${Math.round(delta!)} пунктов к прошлой сводке`;
}

export function buildHealthImprovementSummary(input: AccountHealthDisplayInput): string {
  const structured = buildHealthImprovementView(input);
  if (structured.items.length > 0) {
    return [structured.intro, ...structured.items.map((item) => `${item.title}: ${item.explanation}`)]
      .filter(Boolean)
      .join(" ");
  }
  return structured.intro;
}

export function buildHealthImprovementView(input: AccountHealthDisplayInput): {
  intro: string;
  items: HealthImprovementItemView[];
} {
  const introRaw = sanitizeHealthText(input.accountSummary.improvement_summary);
  const intro =
    introRaw && !isInappropriateImprovementSummary(introRaw) && !/^что улучшит индекс$/i.test(introRaw)
      ? introRaw
      : "";

  const actionHrefByKey = new Map<string, string>();
  for (const action of input.topActions ?? []) {
    const href = actionHrefFromTopAction(action);
    if (!href) continue;
    if (action.action_key) actionHrefByKey.set(String(action.action_key), href);
    if (action.id) actionHrefByKey.set(String(action.id), href);
    if (action.action_id != null) actionHrefByKey.set(String(action.action_id), href);
  }

  const items = (input.accountSummary.improvement_levers ?? [])
    .map((lever): HealthImprovementItemView | null => {
      const title = sanitizeHealthText(lever.title);
      const explanation = sanitizeHealthText(lever.explanation);
      if (!title && !explanation) return null;
      const linked = (lever.linked_action_id ?? "").trim();
      const actionHref = linked
        ? actionHrefByKey.get(linked) ?? `/app/actions?action=${encodeURIComponent(linked)}`
        : undefined;
      return {
        title: title || "Фактор улучшения",
        explanation: explanation || title,
        factorLabel: factorLabelFromID(lever.factor_id),
        expectedDirection: sanitizeHealthText(lever.expected_direction),
        actionHref,
      };
    })
    .filter((item): item is HealthImprovementItemView => item != null)
    .slice(0, 4);

  return { intro, items };
}

function factorLabelFromID(factorID?: string | null): string | undefined {
  const raw = (factorID ?? "").trim();
  if (!raw) return undefined;
  const id = raw.toLowerCase();
  if (/^risk[_-]?\d+$/i.test(id) || /^driver[_-]?\d+$/i.test(id)) {
    return undefined;
  }
  // Never surface snake_case technical factor codes to sellers.
  if (/^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(id)) {
    const map: Record<string, string> = {
      high_drr_campaign: "Кампании с высокой ДРР",
      sales_decline: "Падение продаж",
      no_sales_with_stock: "Остаток без продаж",
      stock_shortage: "Дефицит остатков",
      stock_critical: "Критический остаток",
      wasted_ad_spend: "Неэффективные расходы на рекламу",
      funnel_weak_stage: "Слабый переход воронки",
    };
    return map[id];
  }
  const map: Record<string, string> = {
    sales: "Продажи",
    advertising: "Реклама",
    ads: "Реклама",
    stock: "Остатки",
    funnel: "Воронка",
    cancellations: "Отмены",
    risk: "Риски",
  };
  return map[id];
}

function actionHrefFromTopAction(action: TopActionItem): string | undefined {
  if (action.action_key) {
    return `/app/actions?action=${encodeURIComponent(action.action_key)}`;
  }
  if (action.action_id != null) {
    return `/app/actions?action=${encodeURIComponent(String(action.action_id))}`;
  }
  if (action.id) {
    return `/app/actions?action=${encodeURIComponent(action.id)}`;
  }
  return undefined;
}

export function compactHealthDriverText(value: string): string {
  let text = value.trim().replace(/\s+/g, " ");
  if (!text) {
    return "";
  }
  text = text.replace(/«[^»]{24,}»/gu, (match) => {
    const inner = match.slice(1, -1);
    return `«${inner.slice(0, 20)}…»`;
  });
  const MAX = 110;
  if (text.length <= MAX) {
    return text;
  }
  const cut = text.slice(0, MAX).replace(/\s+\S*$/, "").trim();
  return cut ? `${cut}…` : `${text.slice(0, MAX)}…`;
}

export function buildHealthDriverViews(input: AccountHealthDisplayInput): HealthDriverView[] {
  return (input.accountSummary.risk_drivers ?? [])
    .slice(0, DRIVER_LIMIT)
    .map((driver) => {
      const reason = prepareHealthDriver(driver.reason);
      const title = prepareHealthDriver(driver.title);
      if (!reason && !title) {
        return null;
      }
      const resolvedTitle = title || "Требует внимания";
      const resolvedReason = reason || title;
      if (resolvedTitle === resolvedReason && resolvedReason.length > 80) {
        return {
          title: compactHealthDriverText(resolvedTitle),
          reason: resolvedReason,
          iconCategory: mapRiskDriverCategory(driver.category),
        };
      }
      return {
        title: resolvedTitle,
        reason: resolvedReason,
        iconCategory: mapRiskDriverCategory(driver.category),
      };
    })
    .filter((driver): driver is HealthDriverView => driver != null);
}

export function buildAccountHealthDisplayView(input: AccountHealthDisplayInput): AccountHealthDisplayView {
  const score = input.accountSummary.health_score;
  const severity =
    score != null && Number.isFinite(score)
      ? severityFromScore(score) || input.accountSummary.severity
      : input.accountSummary.severity;
  const improvement = buildHealthImprovementView(input);
  return {
    scoreLabel: formatHealthScore(score),
    scorePercent: getHealthScorePercent(score),
    severityLabel: getHealthSeverityLabel(severity, input.accountSummary.health_status_label),
    severityTone: getHealthSeverityTone(severity ?? input.accountSummary.health_status_code),
    severityPillVariant: getHealthSeverityPillVariant(severity),
    deltaText: buildHealthDeltaText(input.accountSummary.health_score_delta),
    drivers: buildHealthDriverViews(input),
    improvementSummary: buildHealthImprovementSummary(input),
    improvementIntro: improvement.intro,
    improvementItems: improvement.items,
  };
}

function severityFromScore(score: number): string {
  const clamped = Math.max(0, Math.min(100, Math.round(score)));
  if (clamped >= 85) return "low";
  if (clamped >= 70) return "medium";
  if (clamped >= 50) return "high";
  return "critical";
}

function clampHealthScore(score?: number | null): number | null {
  if (typeof score !== "number" || !Number.isFinite(score)) {
    return null;
  }
  return Math.max(0, Math.min(100, Math.round(score)));
}

function prepareHealthDriver(value?: string | null): string {
  return sanitizeHealthText(value);
}

function sanitizeHealthText(value?: string | null): string {
  const text = (value ?? "").trim();
  if (!text) {
    return "";
  }
  return sanitizeSellerFacingText(
    formatStandaloneMoney(sanitizeDashboardSummaryText(text)).replace(/\s{2,}/g, " "),
  );
}

function formatStandaloneMoney(text: string): string {
  return text.replace(/(^|[^\d])(\d{4,})\s*₽/gu, (match: string, prefix: string, digits: string) => {
    const formatted = `${scoreFormatter.format(Number(digits))} ₽`;
    return match.startsWith(prefix) ? `${prefix}${formatted}` : formatted;
  });
}

function mapRiskDriverCategory(category?: RiskDriver["category"]): HealthDriverIconCategory {
  switch (category) {
    case "advertising":
      return "advertising";
    case "stock":
    case "surplus":
    case "sku":
      return "stock";
    case "funnel":
      return "funnel";
    case "sales":
      return "sales";
    case "cancellations":
      return "cancellations";
    default:
      return "risk";
  }
}
