import type { StockCoverageNarrative } from "@/lib/dashboard-home-types";
import { asArray } from "@/lib/as-array";

export const SELLER_REVENUE_METHOD_NOTE =
  "По SKU с продажами за последние 7 дней и коротким покрытием.";

export const STOCK_COVERAGE_PENDING_SUMMARY =
  "AI-вывод по остаткам обновляется. Основные показатели уже доступны.";

export const STOCK_COVERAGE_PENDING_ACTIONS =
  "AI-рекомендации по остаткам обновляются.";

export const COVERAGE_DISTRIBUTION_BUCKET_ORDER = [
  "out_of_stock",
  "critical_0_3",
  "shortage_4_7",
  "planning_8_14",
  "healthy_14plus",
  "surplus",
  "no_sales",
] as const;

export type CoverageDistributionBucketKey = (typeof COVERAGE_DISTRIBUTION_BUCKET_ORDER)[number];

export type CoverageDistributionBucket = {
  key: CoverageDistributionBucketKey;
  label: string;
  count: number;
};

const BUCKET_LABELS: Record<CoverageDistributionBucketKey, string> = {
  out_of_stock: "Нет остатка",
  critical_0_3: "0–3 дня",
  shortage_4_7: "4–7 дней",
  planning_8_14: "8–14 дней",
  healthy_14plus: ">14 дней",
  surplus: "Избыток",
  no_sales: "Без продаж",
};

const TECHNICAL_TERMS = [
  "narrative",
  "fallback",
  "generation",
  "daily summary",
  "raw payload",
  "weighted_avg",
  "weighted_avg_daily_revenue_7d",
];

const LEGACY_SKU_FOCUS_LABEL_PATTERNS = [
  /^перейти к SKU\b/i,
  /^смотреть SKU\b/i,
  /SKU-фокус/i,
  /смотреть товары/i,
];

export function replaceArtikulWithSKU(value: string): string {
  return value.replace(/артикул(?:ами|ах|ов|ом|у|е|ы|а)?/gi, "SKU");
}

export function sanitizeSellerFacingStockText(text?: string | null): string {
  let s = replaceArtikulWithSKU((text ?? "").trim().replace(/\s+/g, " "));
  for (const term of TECHNICAL_TERMS) {
    const re = new RegExp(term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "gi");
    s = s.replace(re, "");
  }
  return s.trim();
}

function isLegacySKUFocusLabel(label: string): boolean {
  return LEGACY_SKU_FOCUS_LABEL_PATTERNS.some((re) => re.test(label));
}

export function stockCoverageLinkLabel(label?: string): string {
  const cleaned = sanitizeSellerFacingStockText(label);
  if (!cleaned || isLegacySKUFocusLabel(cleaned)) {
    return "Открыть SKU с риском";
  }
  return cleaned;
}

const LEGACY_NO_SALES_FOCUS_LABEL_PATTERNS = [/^проверить SKU без продаж$/i, /^смотреть SKU без продаж$/i];

function isLegacyNoSalesFocusLabel(label: string): boolean {
  return LEGACY_NO_SALES_FOCUS_LABEL_PATTERNS.some((re) => re.test(label));
}

/** Secondary CTA: navigation wording only; target/hash unchanged. */
export function noSalesFocusLinkLabel(label?: string): string {
  const cleaned = sanitizeSellerFacingStockText(label);
  if (!cleaned || isLegacyNoSalesFocusLabel(cleaned)) {
    return "Открыть SKU без продаж";
  }
  return cleaned;
}

export function sellerFacingRevenueMethodNote(raw: string | undefined): string {
  const s = (raw ?? "").trim();
  if (!s || /weighted_avg_daily_revenue_7d/i.test(s) || /weighted_avg/i.test(s)) {
    return SELLER_REVENUE_METHOD_NOTE;
  }
  return sanitizeSellerFacingStockText(s);
}

export type SegmentCountSource = { key: string; count?: number; sku_count?: number };

export function segmentCountByKey(segments: SegmentCountSource[] | null | undefined, key: string): number {
  const list = segments ?? [];
  const seg = list.find((s) => s.key === key);
  return seg?.count ?? seg?.sku_count ?? 0;
}

export function buildCoverageDistributionBuckets(
  segments?: SegmentCountSource[] | null,
): CoverageDistributionBucket[] {
  const list = segments ?? [];
  return COVERAGE_DISTRIBUTION_BUCKET_ORDER.map((key) => ({
    key,
    label: BUCKET_LABELS[key],
    count: segmentCountByKey(list, key),
  }));
}

function isGenericStockCoverageText(text: string): boolean {
  const lower = text.toLowerCase();
  if (lower === "недостаточно данных для оценки покрытия остатков.") return true;
  if (text.startsWith("Покрытие показывает, на сколько дней")) return true;
  return false;
}

export function isStockCoverageSubstantiveAIText(text?: string | null): boolean {
  const s = sanitizeSellerFacingStockText(text);
  if (!s || s.length < 24) return false;
  if (isGenericStockCoverageText(s)) return false;
  return true;
}

export function isStockCoverageAISummaryAvailable(narrative?: StockCoverageNarrative | null): boolean {
  if (isStockCoverageSubstantiveAIText(narrative?.headline)) return true;
  return isStockCoverageSubstantiveAIText(narrative?.summary);
}

export function getStockCoverageHeadline(narrative?: StockCoverageNarrative | null): string {
  const headline = sanitizeSellerFacingStockText(narrative?.headline);
  if (isStockCoverageSubstantiveAIText(headline)) {
    return headline;
  }
  return "";
}

export function getStockCoverageNextStep(narrative?: StockCoverageNarrative | null): string {
  const nextStep = sanitizeSellerFacingStockText(narrative?.next_step);
  return isStockCoverageSubstantiveAIText(nextStep) ? nextStep : "";
}

/** Summary strip: `summary` only; `risk_comment` is legacy fallback when summary is absent. */
export function getStockCoverageSummaryText(narrative?: StockCoverageNarrative | null): string {
  if (isStockCoverageAISummaryAvailable(narrative)) {
    return sanitizeSellerFacingStockText(narrative?.summary);
  }
  if (isStockCoverageSubstantiveAIText(narrative?.risk_comment)) {
    return sanitizeSellerFacingStockText(narrative?.risk_comment);
  }
  return STOCK_COVERAGE_PENDING_SUMMARY;
}

function isNoSalesInsightSubstantive(text?: string | null): boolean {
  const s = sanitizeSellerFacingStockText(text);
  if (!s || s.length < 12) return false;
  if (/^SKU без продаж:\s*\d+/i.test(s)) return false;
  if (isGenericStockCoverageText(s)) return false;
  return true;
}

export function buildFactualNoSalesChipText(count: number, stockUnits?: number): string {
  if (count <= 0) return "";
  const units = stockUnits ?? 0;
  if (units > 0) {
    return `SKU без продаж: ${count} · остаток ${units} шт.`;
  }
  return `SKU без продаж: ${count}`;
}

/** AI insight only from `no_sales_insight`; factual chip from counts — never from `comment`. */
export function getNoSalesInsightText(narrative?: StockCoverageNarrative | null): {
  kind: "ai" | "factual" | "none";
  text: string;
} {
  const aiInsight = sanitizeSellerFacingStockText(narrative?.no_sales_insight);
  if (isNoSalesInsightSubstantive(aiInsight)) {
    return { kind: "ai", text: aiInsight };
  }

  const count = narrative?.no_sales_with_stock?.count ?? 0;
  if (count > 0 && narrative?.no_sales_with_stock?.status === "available") {
    return {
      kind: "factual",
      text: buildFactualNoSalesChipText(count, narrative.no_sales_with_stock.stock_units),
    };
  }
  return { kind: "none", text: "" };
}

export type StockCoverageActionDisplay = {
  title: string;
  text: string;
};

export function getStockCoverageActions(narrative?: StockCoverageNarrative | null): StockCoverageActionDisplay[] {
  const raw = asArray(narrative?.actions).length > 0 ? asArray(narrative?.actions) : asArray(narrative?.checklist);
  const out: StockCoverageActionDisplay[] = [];
  for (const action of raw) {
    const title = sanitizeSellerFacingStockText(action.title);
    const text = sanitizeSellerFacingStockText(action.text);
    if (!title && !text) continue;
    if (/^(проверьте всё|что сделать|нет рекомендаций)$/i.test(`${title} ${text}`.trim())) continue;
    out.push({ title: title || text, text: title ? text : "" });
    if (out.length >= 4) break;
  }
  return out;
}

export function hasNoSalesTarget(narrative?: StockCoverageNarrative | null): boolean {
  const count = narrative?.no_sales_with_stock?.count ?? 0;
  return count > 0 && narrative?.no_sales_with_stock?.status === "available";
}

export function noSalesFocusHref(narrative?: StockCoverageNarrative | null): string {
  const target = narrative?.no_sales_focus_link?.target ?? "sku-focus-no-sales";
  return `#${target}`;
}

export function stockRiskFocusHref(narrative?: StockCoverageNarrative | null): string {
  const target = narrative?.sku_focus_link?.target ?? "sku-focus-stock";
  return `#${target}?show=all&filter=all-risk`;
}

export function formatSKUCount(count: number): string {
  return `${count} SKU`;
}

export function getRiskSKUCount(segments: SegmentCountSource[] | null | undefined): number {
  return (
    segmentCountByKey(segments, "out_of_stock") +
    segmentCountByKey(segments, "critical_0_3") +
    segmentCountByKey(segments, "shortage_4_7")
  );
}

export function hasStockCoverageRisk(segments: SegmentCountSource[] | null | undefined): boolean {
  return getRiskSKUCount(segments) > 0;
}

export type CoverageDiagnosisTone = "risk" | "calm";

export function getCoverageDiagnosisTone(segments: SegmentCountSource[] | null | undefined): CoverageDiagnosisTone {
  return hasStockCoverageRisk(segments) ? "risk" : "calm";
}

const NO_RISK_DIAGNOSIS_TITLE = "Критического риска дефицита не обнаружено";
const NO_RISK_DIAGNOSIS_DESCRIPTION =
  "Продающие SKU имеют достаточное покрытие при текущем темпе продаж";
const DEFAULT_RISK_DIAGNOSIS_TITLE = "Есть риск дефицита у продающих SKU";

export function buildRiskCTALabel(riskCount: number, narrative?: StockCoverageNarrative | null): string {
  const legacy = stockCoverageLinkLabel(narrative?.sku_focus_link?.label);
  if (riskCount <= 0) return legacy;
  if (legacy === "Открыть SKU с риском") {
    return `Открыть ${formatSKUCount(riskCount)} с риском`;
  }
  return legacy;
}

export function buildNoSalesCTALabel(count: number, narrative?: StockCoverageNarrative | null): string {
  if (count <= 0) return noSalesFocusLinkLabel(narrative?.no_sales_focus_link?.label);
  return `Проверить ${formatSKUCount(count)}`;
}

export function buildNoSalesTitle(count: number): string {
  return `${formatSKUCount(count)} без продаж`;
}

export function buildDiagnosisTitle(
  narrative: StockCoverageNarrative | null | undefined,
  segments: SegmentCountSource[] | null | undefined,
): string {
  if (!hasStockCoverageRisk(segments)) {
    return NO_RISK_DIAGNOSIS_TITLE;
  }
  const headline = getStockCoverageHeadline(narrative);
  if (headline) return headline;
  return DEFAULT_RISK_DIAGNOSIS_TITLE;
}

function formatRiskSegmentPhrase(oos: number, critical: number, short: number): string {
  const parts: string[] = [];
  if (oos > 0) {
    parts.push(`${oos} без остатка`);
  }
  if (critical > 0) {
    parts.push(`${critical} с покрытием 0–3 дня`);
  }
  if (short > 0) {
    parts.push(`${short} с покрытием 4–7 дней`);
  }
  if (parts.length === 0) return "";
  return parts.join(", ");
}

export function buildFactualDiagnosisDescription(
  segments: SegmentCountSource[] | null | undefined,
  revenueValue?: string | null,
): string {
  const oos = segmentCountByKey(segments, "out_of_stock");
  const critical = segmentCountByKey(segments, "critical_0_3");
  const short = segmentCountByKey(segments, "shortage_4_7");
  const riskTotal = oos + critical + short;
  if (riskTotal <= 0) return NO_RISK_DIAGNOSIS_DESCRIPTION;

  const segmentPhrase = formatRiskSegmentPhrase(oos, critical, short);
  let text = `${riskTotal} SKU требуют внимания: ${segmentPhrase}.`;
  const revenue = (revenueValue ?? "").trim();
  if (revenue) {
    text += ` Под риском до ${revenue} среднедневной выручки.`;
  }
  return text;
}

export function buildDiagnosisDescription(
  narrative: StockCoverageNarrative | null | undefined,
  segments: SegmentCountSource[] | null | undefined,
): string {
  if (!hasStockCoverageRisk(segments)) {
    return NO_RISK_DIAGNOSIS_DESCRIPTION;
  }
  const summaryText = getStockCoverageSummaryText(narrative);
  if (summaryText !== STOCK_COVERAGE_PENDING_SUMMARY) {
    return summaryText;
  }
  const revenue =
    narrative?.revenue_at_risk?.status === "available" ? narrative.revenue_at_risk.value : null;
  return buildFactualDiagnosisDescription(segments, revenue);
}

export function shouldShowRiskPrimaryCTA(segments: SegmentCountSource[] | null | undefined): boolean {
  return hasStockCoverageRisk(segments);
}

export function buildDistributionBarSummary(buckets: CoverageDistributionBucket[], total: number): string {
  if (total <= 0) return "";
  const parts = buckets
    .filter((bucket) => bucket.count > 0)
    .map((bucket) => `${bucket.count} — ${bucket.label}`);
  if (parts.length === 0) return "";
  return `Из ${total} SKU: ${parts.join(", ")}.`;
}

/** Parse "Остаток: 6" / "Покрытие: нет" into label + value for chips. */
export function parseStockMetricChip(raw: string): { kind: string; value: string } | null {
  const m = raw.trim().match(/^(Остаток|Покрытие)\s*:\s*(.+)$/i);
  if (!m) return null;
  return { kind: m[1], value: m[2].trim() };
}
