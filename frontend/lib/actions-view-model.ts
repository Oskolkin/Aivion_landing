import {
  actionCategoryLabel,
  actionStatusLabel,
  areActionTextsNearDuplicate,
  buildDisplaySteps,
  formatActionMetricValue,
  formatCompletionConditionLine,
  formatMetricDate,
  formatMetricLabel,
  getActionCategoryTone,
  getActionStatusTone,
  normalizeActionReasonForDisplay,
  normalizeAffectedObjects,
  normalizeEntityId,
  resolveAffectedObjectCounts,
  type ActionTone,
} from "@/lib/actions-display";
import type {
  ActionCategory,
  ActionCTA,
  ActionItem,
  ActionPriority,
  ActionStatus,
  ActionsAIFocus,
  OperationalFact,
} from "@/lib/actions-types";

export type ActionEntityType =
  | "sku"
  | "campaign"
  | "category"
  | "product_card"
  | "funnel_stage"
  | "account"
  | "unknown";

export type ActionSortKey = "priority" | "newest" | "oldest" | "category" | "entities";

export type ActionStatusFilter = "active" | "completed" | "irrelevant";

export type ActionCategoryFilter =
  | "all"
  | "stock"
  | "slow_turnover"
  | "sku_sales"
  | "advertising"
  | "funnel"
  | "category";

export type ActionMetricIcon =
  | "packages"
  | "hourglass"
  | "calendar"
  | "clock"
  | "trend"
  | "wallet"
  | "megaphone"
  | "funnel"
  | "info";

export type ActionMetricTone = "neutral" | "info" | "warning" | "danger" | "success";

export type ActionMetricViewModel = {
  id: string;
  label: string;
  value: string;
  helper?: string;
  icon: ActionMetricIcon;
  tone?: ActionMetricTone;
  tooltip?: string;
};

export type ActionStepViewModel = {
  id: string;
  order: number;
  title: string;
  description?: string;
  status?: "not_started" | "in_progress" | "completed" | "blocked";
  resultLabel?: string;
  deepLink?: string;
};

export type CompletionCriterionViewModel = {
  id: string;
  label: string;
  status?: "met" | "not_met" | "unknown";
  technicalExplanation?: string;
};

export type ActionCTAViewModel = {
  id: string;
  label: string;
  hint?: string;
  href?: string;
  kind: "navigate" | "scroll" | "export" | "ask_ai" | "complete" | "dismiss" | "custom";
  exportType?: string;
};

export type AffectedEntityRowViewModel = {
  id: string;
  primaryLabel: string;
  secondaryLabel?: string;
  imageUrl?: string;
  entityType: ActionEntityType;
  cells: Array<{ id: string; label: string; value: string; tone?: ActionMetricTone }>;
  recommendation?: string;
  priorityLabel?: string;
  priorityTone?: ActionMetricTone;
};

export type ActionListCardViewModel = {
  id: string;
  actionKey: string;
  title: string;
  summary: string | null;
  priority: { value: ActionPriority; label: string; tone: ActionTone };
  category: { id: ActionCategory | string; label: string; tone: ActionTone };
  status: { value: ActionStatus; label: string; tone: ActionTone };
  showStatusBadge: boolean;
  mandatoryLabel: string | null;
  entitiesLabel: string | null;
  dateLabel: string | null;
  dueLabel: string | null;
};

export type ActionDetailViewModel = {
  id: string;
  actionKey: string;
  title: string;
  summary: string | null;
  priority: { value: ActionPriority; label: string; tone: ActionTone };
  category: { id: ActionCategory | string; label: string; tone: ActionTone };
  status: { value: ActionStatus; label: string; tone: ActionTone };
  createdAtLabel: string | null;
  dueAtLabel: string | null;
  mandatoryLabel: string | null;
  metrics: ActionMetricViewModel[];
  situation: { title: string; body: string; tone: ActionMetricTone } | null;
  importanceFacts: string[];
  importanceText: string | null;
  affectedEntities: {
    type: ActionEntityType;
    label: string;
    count: number;
    rows: AffectedEntityRowViewModel[];
    coverageNote: string | null;
  } | null;
  steps: ActionStepViewModel[];
  expectedResult: string | null;
  completionCriteria: CompletionCriterionViewModel[];
  primaryAction: ActionCTAViewModel | null;
  secondaryActions: ActionCTAViewModel[];
  aiContextAvailable: boolean;
  openEntitiesHref: string | null;
  openEntitiesLabel: string | null;
};

export type DailyFocusViewModel = {
  headline: string;
  recommendation: string | null;
  affectedLabel: string | null;
  riskLabel: string | null;
  riskTooltip: string | null;
  priorityLabel: string | null;
  priorityTone: ActionTone | null;
  ctaLabel: string;
  empty: boolean;
};

function pluralizeRu(count: number, one: string, few: string, many: string): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} ${one}`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} ${few}`;
  return `${count} ${many}`;
}

export function pluralizeEntityCount(count: number, entityType: ActionEntityType | string): string {
  switch (normalizeEntityType(entityType)) {
    case "sku":
      return `${count} SKU`;
    case "campaign":
      return pluralizeRu(count, "кампания", "кампании", "кампаний");
    case "category":
      return pluralizeRu(count, "категория", "категории", "категорий");
    case "product_card":
      return pluralizeRu(count, "карточка", "карточки", "карточек");
    case "funnel_stage":
      return pluralizeRu(count, "этап воронки", "этапа воронки", "этапов воронки");
    case "account":
      return "весь кабинет";
    default:
      return pluralizeRu(count, "позиция", "позиции", "позиций");
  }
}

export function normalizeEntityType(raw: string | undefined | null): ActionEntityType {
  switch ((raw ?? "").toLowerCase()) {
    case "sku":
    case "product":
      return "sku";
    case "campaign":
    case "ad_campaign":
      return "campaign";
    case "category":
      return "category";
    case "product_card":
    case "card":
      return "product_card";
    case "funnel":
    case "funnel_stage":
    case "funnel_transition":
      return "funnel_stage";
    case "account":
      return "account";
    default:
      return raw ? "unknown" : "unknown";
  }
}

export function entitySectionTitle(entityType: ActionEntityType, count: number): string {
  switch (entityType) {
    case "sku":
      return `Затронутые товары (${count})`;
    case "campaign":
      return `Затронутые кампании (${count})`;
    case "category":
      return `Затронутые категории (${count})`;
    case "product_card":
      return `Затронутые карточки (${count})`;
    case "funnel_stage":
      return `Затронутые этапы воронки (${count})`;
    case "account":
      return "Уровень действия";
    default:
      return `Затронутые позиции (${count})`;
  }
}

export function actionPriorityDisplayLabel(priority: ActionPriority): string {
  switch (priority) {
    case "critical":
      return "Критический";
    case "important":
      return "Высокий приоритет";
    case "watch":
      return "Средний приоритет";
    default:
      return priority;
  }
}

export function actionPriorityTone(priority: ActionPriority): ActionTone {
  switch (priority) {
    case "critical":
      return { bg: "bg-red-50", text: "text-red-700", ring: "ring-red-200" };
    case "important":
      return { bg: "bg-orange-50", text: "text-orange-700", ring: "ring-orange-200" };
    case "watch":
      return { bg: "bg-slate-50", text: "text-slate-700", ring: "ring-slate-200" };
    default:
      return { bg: "bg-slate-50", text: "text-slate-700", ring: "ring-slate-200" };
  }
}

function formatShortDate(iso?: string | null): string | null {
  if (!iso) return null;
  const datePart = iso.includes("T") ? iso.split("T")[0] : iso;
  if (!datePart) return null;
  const [y, m, d] = datePart.split("-").map(Number);
  if (!y || !m || !d) return formatMetricDate(datePart) || null;
  const date = new Date(Date.UTC(y, m - 1, d));
  if (Number.isNaN(date.getTime())) return formatMetricDate(datePart) || null;
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long" }).format(date);
}

function lookLikeTechnicalMetric(text: string): boolean {
  return /_[a-z0-9]+|[a-z]+_[a-z]+|available_stock|metric_code|target_value/i.test(text);
}

function humanizeCompletionLabel(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "";
  if (!lookLikeTechnicalMetric(trimmed)) return trimmed;
  // Prefer not to show raw field expressions as primary UI copy.
  if (/available_stock/i.test(trimmed)) {
    return "Доступный остаток по затронутым SKU стал больше нуля";
  }
  if (/coverage_days|idc_days/i.test(trimmed)) {
    return "Покрытие остатком вернулось в допустимый диапазон";
  }
  if (/drr/i.test(trimmed)) {
    return "ДРР затронутых кампаний снизился до целевого уровня";
  }
  return "Условия рекомендации выполнены по данным кабинета";
}

function parseMaybeNumber(value: string): number | null {
  const cleaned = value.replace(/\s/g, "").replace(",", ".").replace(/[^\d.-]/g, "");
  if (!cleaned) return null;
  const num = Number(cleaned);
  return Number.isFinite(num) ? num : null;
}

function evidenceLooksLikeRevenueRisk(label: string): boolean {
  const lower = label.toLowerCase();
  return (
    lower.includes("риск") ||
    lower.includes("потер") ||
    lower.includes("revenue_risk") ||
    lower.includes("potential") ||
    (lower.includes("выручк") && (lower.includes("риск") || lower.includes("недопол")) )
  );
}

function findRevenueRisk(action: ActionItem): { value: string; tooltip?: string } | null {
  for (const item of action.evidence ?? []) {
    const label = formatMetricLabel(item.label);
    if (!evidenceLooksLikeRevenueRisk(item.label) && !evidenceLooksLikeRevenueRisk(label)) continue;
    const formatted = formatActionMetricValue({
      metricKey: label,
      value: item.value,
      rawUnit: item.unit ?? "rub",
    });
    if (!formatted || formatted === "—") continue;
    return {
      value: formatted,
      tooltip: item.description || "Оценка потенциальной потери выручки по данным аналитики.",
    };
  }
  return null;
}

function metricIconForLabel(label: string): ActionMetricIcon {
  const lower = label.toLowerCase();
  if (lower.includes("покрыт") || lower.includes("дн") || lower.includes("coverage") || lower.includes("idc")) {
    return "hourglass";
  }
  if (lower.includes("срок") || lower.includes("реакц") || lower.includes("сегодня")) return "clock";
  if (lower.includes("период") || lower.includes("горизонт") || lower.includes("7") || lower.includes("30")) {
    return "calendar";
  }
  if (lower.includes("риск") || lower.includes("выруч") || lower.includes("₽")) return "trend";
  if (lower.includes("расход") || lower.includes("бюджет") || lower.includes("дрр")) return "wallet";
  if (lower.includes("кампан") || lower.includes("реклам")) return "megaphone";
  if (lower.includes("воронк") || lower.includes("конверс")) return "funnel";
  if (lower.includes("sku") || lower.includes("товар") || lower.includes("остат")) return "packages";
  return "info";
}

function metricToneForValue(label: string, value: string): ActionMetricTone {
  const lower = `${label} ${value}`.toLowerCase();
  if (lower.includes("0 дн") || lower.includes("нулев") || /\b0\b/.test(value) && lower.includes("остат")) {
    return "danger";
  }
  if (lower.includes("риск") || lower.includes("критич")) return "warning";
  return "neutral";
}

/** Distinguish unknown vs real zero: unknown values are omitted from metrics. */
export function isUnknownMetricValue(value: string | null | undefined): boolean {
  if (value == null) return true;
  const trimmed = value.trim();
  if (!trimmed || trimmed === "—" || trimmed === "-" || trimmed.toLowerCase() === "n/a") return true;
  return false;
}

function inferEntityType(action: ActionItem): ActionEntityType {
  const objectActions = action.operational_detail?.object_actions ?? [];
  if (objectActions.length > 0) {
    return normalizeEntityType(objectActions[0]?.entity_type);
  }
  const entities = action.affected_entities;
  if (entities?.skus?.length) return "sku";
  if (entities?.campaigns?.length) return "campaign";
  if (entities?.funnel) return "funnel_stage";
  switch (action.category) {
    case "advertising":
      return "campaign";
    case "funnel":
      return "funnel_stage";
    case "category":
      return "category";
    case "stock":
    case "slow_turnover":
    case "sku":
    case "sales":
      return "sku";
    default:
      break;
  }
  const counts = resolveAffectedObjectCounts(action);
  return normalizeEntityType(counts.entityType !== "other" ? counts.entityType : undefined);
}

export function buildActionMetrics(action: ActionItem): ActionMetricViewModel[] {
  const metrics: ActionMetricViewModel[] = [];
  const counts = resolveAffectedObjectCounts(action);
  const entityType = inferEntityType(action);

  if (counts.total > 0 && entityType !== "account") {
    metrics.push({
      id: "affected-count",
      label: entitySectionTitle(entityType, counts.total).replace(/\s*\(\d+\)\s*$/, ""),
      value: String(counts.total),
      helper: pluralizeEntityCount(counts.total, entityType),
      icon: entityType === "campaign" ? "megaphone" : entityType === "funnel_stage" ? "funnel" : "packages",
      tone: "info",
    });
  }

  for (const evidence of (action.evidence ?? []).slice(0, 6)) {
    const label = formatMetricLabel(evidence.label);
    if (!label) continue;
    if (evidenceLooksLikeRevenueRisk(evidence.label) || evidenceLooksLikeRevenueRisk(label)) {
      const risk = findRevenueRisk(action);
      if (risk && !isUnknownMetricValue(risk.value)) {
        metrics.push({
          id: `risk-${evidence.label}`,
          label: "Потенциальный риск",
          value: risk.value,
          icon: "trend",
          tone: "warning",
          tooltip: risk.tooltip,
        });
      }
      continue;
    }
    const value = formatActionMetricValue({
      metricKey: label,
      value: evidence.value,
      rawUnit: evidence.unit,
    });
    if (isUnknownMetricValue(value)) continue;
    // Avoid duplicating the affected-count metric.
    if (/затронут|sku|кампан/i.test(label) && metrics.some((m) => m.id === "affected-count")) continue;
    metrics.push({
      id: `evidence-${evidence.kind}-${evidence.label}`,
      label,
      value,
      icon: metricIconForLabel(label),
      tone: metricToneForValue(label, value),
      tooltip: evidence.description,
    });
  }

  // Deduplicate by label, keep first 5.
  const seen = new Set<string>();
  const unique: ActionMetricViewModel[] = [];
  for (const metric of metrics) {
    const key = metric.label.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    unique.push(metric);
    if (unique.length >= 5) break;
  }
  return unique;
}

function importanceFromFacts(facts?: OperationalFact[]): string[] {
  if (!facts?.length) return [];
  return facts
    .map((fact) => {
      const rawLabel = fact.label?.trim() || "";
      const label = rawLabel ? formatMetricLabel(rawLabel) : "";
      const value = (fact.formatted_value || fact.value || "").trim();
      if (!label && !value) return "";
      if (label && value) return `${label}: ${value}${fact.unit ? ` ${formatMetricLabel(fact.unit)}` : ""}`.trim();
      return label || value;
    })
    .filter(Boolean);
}

function buildImportance(action: ActionItem): { facts: string[]; text: string | null } {
  const detail = action.operational_detail;
  const fromFacts = importanceFromFacts(detail?.why?.facts);
  // Skip the synthetic single "importance" fact wrapper — treat its value as text.
  if (fromFacts.length === 1 && detail?.why?.facts?.[0]?.fact_id === "importance") {
    const value = detail.why.facts[0].formatted_value || detail.why.facts[0].value;
    if (value?.trim()) return { facts: [], text: value.trim() };
  }
  if (fromFacts.length > 0) return { facts: fromFacts, text: null };

  const reasonSource = action.reason || "";
  const situation = detail?.why?.text?.trim() || action.summary?.trim() || "";
  const text = normalizeActionReasonForDisplay({
    reason: reasonSource,
    title: action.title,
    summary: situation,
    affectedObjects: detail?.object_actions ?? [],
  });
  if (text) return { facts: [], text };
  if (action.reason?.trim() && !areActionTextsNearDuplicate(action.reason, situation)) {
    return { facts: [], text: action.reason.trim() };
  }
  return { facts: [], text: null };
}

function rowPriorityFromRecommendation(text?: string): { label?: string; tone?: ActionMetricTone } {
  if (!text) return {};
  const lower = text.toLowerCase();
  if (lower.includes("критич") || lower.includes("нулев") || lower.includes("срочн")) {
    return { label: "Критический", tone: "danger" };
  }
  if (lower.includes("высок") || lower.includes("приоритет")) {
    return { label: "Высокий", tone: "warning" };
  }
  if (lower.includes("средн")) {
    return { label: "Средний", tone: "neutral" };
  }
  return {};
}

function buildAffectedRows(action: ActionItem): AffectedEntityRowViewModel[] {
  const objects = action.operational_detail?.object_actions ?? [];
  if (objects.length > 0) {
    return normalizeAffectedObjects(objects).map((obj) => {
      const priority = rowPriorityFromRecommendation(obj.recommendedAction);
      const cells = obj.metrics.map((metric) => ({
        id: metric.key,
        label: metric.label,
        value: metric.formattedValue,
        tone: /остаток/i.test(metric.label) && /^0(\s|$|шт)/i.test(metric.formattedValue)
          ? ("danger" as const)
          : undefined,
      }));
      return {
        id: obj.key,
        primaryLabel: obj.primaryLabel,
        secondaryLabel: obj.secondaryLabel,
        entityType: normalizeEntityType(obj.entityType),
        cells,
        recommendation: obj.compactRecommendation ?? obj.recommendedAction,
        priorityLabel: priority.label,
        priorityTone: priority.tone,
      };
    });
  }

  const entities = action.affected_entities;
  if (!entities) return [];
  const rows: AffectedEntityRowViewModel[] = [];
  for (const sku of entities.skus ?? []) {
    rows.push({
      id: `sku-${sku.sku}`,
      primaryLabel: sku.name || normalizeEntityId(sku.sku),
      secondaryLabel: `SKU ${normalizeEntityId(sku.sku)}`,
      imageUrl: sku.image_url,
      entityType: "sku",
      cells: [],
    });
  }
  for (const campaign of entities.campaigns ?? []) {
    rows.push({
      id: `campaign-${campaign.campaign_id}`,
      primaryLabel: campaign.name || campaign.campaign_id,
      secondaryLabel: `ID ${campaign.campaign_id}`,
      entityType: "campaign",
      cells: [],
    });
  }
  if (entities.funnel) {
    rows.push({
      id: `funnel-${entities.funnel.transition_key || "stage"}`,
      primaryLabel: `${entities.funnel.from_stage} → ${entities.funnel.to_stage}`,
      secondaryLabel: entities.funnel.transition_key || undefined,
      entityType: "funnel_stage",
      cells: [],
    });
  }
  return rows;
}

function derivePrimaryCTA(action: ActionItem): ActionCTAViewModel | null {
  if (action.status === "checked" || action.status === "dismissed") return null;

  const navigable = (action.ctas ?? []).find(
    (cta) =>
      cta.href &&
      cta.type !== "ask_ai" &&
      cta.type !== "mark_checked" &&
      cta.type !== "mark_irrelevant",
  );
  if (navigable?.href) {
    return {
      id: `cta-${navigable.type}`,
      label: navigable.label,
      href: navigable.href,
      kind: "navigate",
    };
  }

  const counts = resolveAffectedObjectCounts(action);
  const entityType = normalizeEntityType(counts.entityType);
  const hasEntities = counts.total > 0 || buildAffectedRows(action).length > 0;

  if (action.category === "stock" || action.type === "stock_replenishment" || action.type === "stock_redistribution") {
    return {
      id: "cta-stocks-replenishment",
      label: "Рассчитать поставку",
      hint: "Перейти к расчёту объёма и сроков",
      href: "/app/stocks-replenishment",
      kind: "navigate",
    };
  }

  if (action.category === "advertising") {
    return {
      id: "cta-advertising",
      label: "Открыть проблемные кампании",
      href: "/app/advertising",
      kind: "navigate",
    };
  }

  if (hasEntities) {
    const label =
      entityType === "campaign"
        ? "Открыть затронутые кампании"
        : entityType === "category"
          ? "Открыть затронутые категории"
          : "Открыть затронутые SKU";
    return {
      id: "cta-scroll-entities",
      label,
      hint: "Показать список в карточке действия",
      kind: "scroll",
      href: "#action-affected-entities",
    };
  }

  return null;
}

function mapCTA(cta: ActionCTA): ActionCTAViewModel | null {
  if (cta.type === "ask_ai" || cta.type === "mark_checked" || cta.type === "mark_irrelevant") return null;
  if (cta.href) {
    return { id: `cta-${cta.type}-${cta.label}`, label: cta.label, href: cta.href, kind: "navigate" };
  }
  if (cta.export_type || cta.type === "export") {
    return {
      id: `cta-export-${cta.export_type || cta.label}`,
      label: cta.label,
      kind: "export",
      exportType: cta.export_type,
    };
  }
  return null;
}

export function mapActionToListCard(action: ActionItem, opts?: { statusFilter?: ActionStatusFilter }): ActionListCardViewModel {
  const statusFilter = opts?.statusFilter ?? "active";
  const counts = resolveAffectedObjectCounts(action);
  const entityType = inferEntityType(action);
  const entitiesLabel =
    counts.total > 0 && entityType !== "account"
      ? pluralizeEntityCount(counts.total, entityType)
      : counts.total > 0
        ? pluralizeEntityCount(counts.total, "sku")
        : null;

  const dateSource = action.created_at || action.metric_date;
  const showStatusBadge =
    statusFilter !== "active" ||
    action.status === "checked" ||
    action.status === "dismissed" ||
    Boolean(action.urgency_marker === "urgent");

  return {
    id: String(action.id),
    actionKey: action.action_key,
    title: action.title,
    summary: action.summary?.trim() || null,
    priority: {
      value: action.priority,
      label: actionPriorityDisplayLabel(action.priority),
      tone: actionPriorityTone(action.priority),
    },
    category: {
      id: action.category,
      label: actionCategoryLabel(action.category),
      tone: getActionCategoryTone(action.category),
    },
    status: {
      value: action.status,
      label:
        action.urgency_marker === "urgent" && action.status === "open"
          ? "Просрочено"
          : actionStatusLabel(action.status),
      tone: getActionStatusTone(action.status),
    },
    showStatusBadge,
    mandatoryLabel: action.execution_tier === "required" ? "Обязательное" : null,
    entitiesLabel,
    dateLabel: formatShortDate(dateSource),
    dueLabel: null,
  };
}

export function mapActionToDetail(action: ActionItem): ActionDetailViewModel {
  const detail = action.operational_detail;
  // Situation = WhyNow (operational why.text) first; avoid title/summary echo.
  const situationCandidate =
    detail?.why?.text?.trim() ||
    action.summary?.trim() ||
    null;
  const situationBody =
    situationCandidate &&
    !areActionTextsNearDuplicate(situationCandidate, action.title)
      ? situationCandidate
      : action.summary?.trim() && !areActionTextsNearDuplicate(action.summary, action.title)
        ? action.summary.trim()
        : situationCandidate;
  const importance = buildImportance(action);
  const rows = buildAffectedRows(action);
  const counts = resolveAffectedObjectCounts(action);
  const entityType = inferEntityType(action);
  const count = Math.max(counts.total, rows.length);
  const hasObjectActions = (action.operational_detail?.object_actions?.length ?? 0) > 0;
  const hasRowMetrics = rows.some((row) => row.cells.length > 0);

  let coverageNote: string | null = null;
  if (count > 0 && action.category === "stock") {
    const done = action.status === "checked" ? count : 0;
    coverageNote = `Прогресс: ${done} из ${count} SKU обработаны · ${done} из ${count} достигли целевого покрытия`;
  } else if (count > 0 && (!hasObjectActions || (!hasRowMetrics && rows.every((row) => !row.recommendation)))) {
    coverageNote = "Для всех позиций применяется общий план действий.";
  } else if (action.object_actions_coverage?.mode === "none") {
    coverageNote = "Для всех позиций применяется общий план действий.";
  } else if (action.object_actions_coverage?.mode === "partial") {
    coverageNote = "Индивидуальный расчёт доступен не для всех позиций; общий план относится ко всей группе.";
  }

  const steps = buildDisplaySteps(action, detail).map((step, index) => ({
    id: detail?.action_plan?.steps?.[index]?.step_id || `step-${index + 1}`,
    order: index + 1,
    title: step.title,
    description: step.instruction || undefined,
    resultLabel: step.expectedResult,
  }));

  const completionCriteria: CompletionCriterionViewModel[] = [];
  if (action.completion_conditions?.length) {
    for (const [index, condition] of action.completion_conditions.entries()) {
      const display = condition.display_text?.trim();
      const technical = formatCompletionConditionLine(condition);
      const label =
        display && !lookLikeTechnicalMetric(display) ? display : humanizeCompletionLabel(technical);
      const technicalExplanation =
        lookLikeTechnicalMetric(condition.metric_code) || lookLikeTechnicalMetric(technical)
          ? technical
          : undefined;
      completionCriteria.push({
        id: `cond-${condition.metric_code}-${index}`,
        label,
        status: "unknown",
        technicalExplanation,
      });
    }
  } else if (action.completion_criteria?.trim()) {
    completionCriteria.push({
      id: "criteria-text",
      label: humanizeCompletionLabel(action.completion_criteria),
      status: "unknown",
      technicalExplanation: lookLikeTechnicalMetric(action.completion_criteria)
        ? action.completion_criteria
        : undefined,
    });
  }

  // No primary navigate CTA in the sticky footer (product decision).
  const primaryAction = null;
  const secondaryActions: ActionCTAViewModel[] = [];

  return {
    id: String(action.id),
    actionKey: action.action_key,
    title: action.title,
    summary: action.summary?.trim() || null,
    priority: {
      value: action.priority,
      label: actionPriorityDisplayLabel(action.priority),
      tone: actionPriorityTone(action.priority),
    },
    category: {
      id: action.category,
      label: actionCategoryLabel(action.category),
      tone: getActionCategoryTone(action.category),
    },
    status: {
      value: action.status,
      label: actionStatusLabel(action.status),
      tone: getActionStatusTone(action.status),
    },
    createdAtLabel: formatShortDate(action.created_at || action.metric_date),
    dueAtLabel: null,
    mandatoryLabel: action.execution_tier === "required" ? "Обязательное" : null,
    metrics: [],
    situation: situationBody
      ? {
          title: "Ситуация",
          body: situationBody,
          tone:
            action.priority === "critical" ? "danger" : action.priority === "important" ? "warning" : "info",
        }
      : null,
    importanceFacts: importance.facts,
    importanceText: importance.text,
    affectedEntities:
      count > 0 || rows.length > 0
        ? {
            type: entityType,
            label: entitySectionTitle(entityType, count || rows.length),
            count: count || rows.length,
            rows,
            coverageNote,
          }
        : null,
    steps,
    expectedResult: action.expected_effect?.trim() || null,
    completionCriteria,
    primaryAction,
    secondaryActions,
    aiContextAvailable: true,
    openEntitiesHref: null,
    openEntitiesLabel: null,
  };
}

export function mapDailyFocus(args: {
  focus?: ActionsAIFocus;
  priorityAction?: ActionItem | null;
  hasOpenActions: boolean;
}): DailyFocusViewModel {
  if (!args.hasOpenActions) {
    return {
      headline: "Критических действий на сегодня нет.",
      recommendation: null,
      affectedLabel: null,
      riskLabel: null,
      riskTooltip: null,
      priorityLabel: null,
      priorityTone: null,
      ctaLabel: "Перейти к действию",
      empty: true,
    };
  }

  const action = args.priorityAction ?? null;
  const focus = args.focus;
  let headline =
    focus?.headline?.trim() ||
    action?.summary?.trim() ||
    action?.title?.trim() ||
    "Приоритетное действие на сегодня";
  headline = headline.replace(/^Главное действие:\s*/i, "").replace(/^Фокус:\s*/i, "");

  const recommendation =
    focus?.start_with?.trim() ||
    focus?.summary?.trim() ||
    action?.next_step?.trim() ||
    null;

  const counts = action ? resolveAffectedObjectCounts(action) : null;
  const entityType = normalizeEntityType(counts?.entityType);
  const affectedLabel =
    counts && counts.total > 0 ? pluralizeEntityCount(counts.total, entityType) : null;

  const risk = action ? findRevenueRisk(action) : null;

  return {
    headline,
    recommendation:
      recommendation && recommendation !== headline ? recommendation : action?.next_step?.trim() || null,
    affectedLabel,
    riskLabel: risk && !isUnknownMetricValue(risk.value) ? risk.value : null,
    riskTooltip: risk?.tooltip ?? null,
    priorityLabel: action ? actionPriorityDisplayLabel(action.priority) : null,
    priorityTone: action ? actionPriorityTone(action.priority) : null,
    ctaLabel: "Перейти к действию",
    empty: false,
  };
}

const PRIORITY_SCORE: Record<string, number> = { critical: 0, important: 1, watch: 2 };
const CATEGORY_SCORE: Record<string, number> = {
  stock: 0,
  slow_turnover: 1,
  sku: 2,
  sales: 3,
  advertising: 4,
  funnel: 5,
  category: 6,
  data_quality: 7,
};

function riskScore(action: ActionItem): number {
  const risk = findRevenueRisk(action);
  if (!risk) return 0;
  return parseMaybeNumber(risk.value) ?? 0;
}

export function sortActionsList(actions: ActionItem[], sort: ActionSortKey): ActionItem[] {
  const tierScore = (tier?: string) => (tier === "required" ? 0 : 1);
  return [...actions].sort((a, b) => {
    if (sort === "newest") {
      const byDate = (b.created_at ?? b.metric_date ?? "").localeCompare(a.created_at ?? a.metric_date ?? "");
      if (byDate !== 0) return byDate;
      return a.action_key.localeCompare(b.action_key);
    }
    if (sort === "oldest") {
      const byDate = (a.created_at ?? a.metric_date ?? "").localeCompare(b.created_at ?? b.metric_date ?? "");
      if (byDate !== 0) return byDate;
      return a.action_key.localeCompare(b.action_key);
    }
    if (sort === "category") {
      const byCat =
        (CATEGORY_SCORE[a.category] ?? 99) - (CATEGORY_SCORE[b.category] ?? 99) ||
        a.category.localeCompare(b.category);
      if (byCat !== 0) return byCat;
      return a.action_key.localeCompare(b.action_key);
    }
    if (sort === "entities") {
      const byCount =
        resolveAffectedObjectCounts(b).total - resolveAffectedObjectCounts(a).total;
      if (byCount !== 0) return byCount;
      return a.action_key.localeCompare(b.action_key);
    }

    // priority (default): tier → priority → risk → date → id
    const byTier = tierScore(a.execution_tier) - tierScore(b.execution_tier);
    if (byTier !== 0) return byTier;
    const byPriority = (PRIORITY_SCORE[a.priority] ?? 9) - (PRIORITY_SCORE[b.priority] ?? 9);
    if (byPriority !== 0) return byPriority;
    const byRisk = riskScore(b) - riskScore(a);
    if (byRisk !== 0) return byRisk;
    const byDate = (b.created_at ?? b.metric_date ?? "").localeCompare(a.created_at ?? a.metric_date ?? "");
    if (byDate !== 0) return byDate;
    return a.action_key.localeCompare(b.action_key);
  });
}

export function matchesActionSearch(action: ActionItem, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const haystacks: string[] = [
    action.title,
    action.summary ?? "",
    action.reason ?? "",
    actionCategoryLabel(action.category),
    action.affected_objects_summary ?? "",
  ];
  for (const sku of action.affected_entities?.skus ?? []) {
    haystacks.push(sku.sku, sku.name ?? "", sku.offer_id ?? "");
  }
  for (const campaign of action.affected_entities?.campaigns ?? []) {
    haystacks.push(campaign.campaign_id, campaign.name ?? "");
  }
  for (const obj of action.operational_detail?.object_actions ?? []) {
    haystacks.push(obj.entity_id, obj.name ?? "", obj.recommended_action ?? "");
  }
  return haystacks.some((part) => part.toLowerCase().includes(q));
}

export function filterActionsByCategory(actions: ActionItem[], category: ActionCategoryFilter): ActionItem[] {
  if (category === "all") return actions;
  if (category === "sku_sales") {
    return actions.filter((item) => item.category === "sku" || item.category === "sales");
  }
  return actions.filter((item) => item.category === category);
}

export const ACTION_SORT_OPTIONS: Array<{ value: ActionSortKey; label: string }> = [
  { value: "priority", label: "По приоритету" },
  { value: "newest", label: "Сначала новые" },
  { value: "oldest", label: "Сначала старые" },
  { value: "category", label: "По категории" },
  { value: "entities", label: "По количеству затронутых" },
];
