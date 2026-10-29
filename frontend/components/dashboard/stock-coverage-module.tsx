"use client";

import type { LucideIcon } from "lucide-react";
import {
  ArrowRight,
  CalendarCheck,
  CalendarClock,
  CircleDollarSign,
  CircleHelp,
  PackageX,
  ShieldCheck,
  Timer,
} from "lucide-react";
import { buttonClassNames } from "@/components/ui/button";
import { cn } from "@/components/ui/cn";
import type { DashboardHomeResponse, StockCoverageNarrative } from "@/lib/dashboard-home-types";
import { asArray } from "@/lib/as-array";
import {
  buildCoverageDistributionBuckets,
  buildDiagnosisDescription,
  buildDiagnosisTitle,
  buildDistributionBarSummary,
  buildNoSalesCTALabel,
  buildNoSalesTitle,
  buildRiskCTALabel,
  type CoverageDistributionBucket,
  formatSKUCount,
  getCoverageDiagnosisTone,
  getNoSalesInsightText,
  hasNoSalesTarget,
  noSalesFocusHref,
  segmentCountByKey,
  shouldShowRiskPrimaryCTA,
  stockRiskFocusHref,
  getRiskSKUCount,
  type SegmentCountSource,
} from "@/lib/stock-coverage-display";
import { navigateToSKUFocusTarget } from "@/lib/sku-focus-hash";
import { dashboardTypography } from "@/lib/dashboard-typography";
import { isWidgetAvailable } from "@/components/dashboard/widget-state";

const BAR_SEGMENT_CLASS: Record<string, { fill: string; dot: string }> = {
  out_of_stock: { fill: "bg-red-800", dot: "bg-red-800" },
  critical_0_3: { fill: "bg-red-500", dot: "bg-red-500" },
  shortage_4_7: { fill: "bg-orange-500", dot: "bg-orange-500" },
  planning_8_14: { fill: "bg-blue-500", dot: "bg-blue-500" },
  healthy_14plus: { fill: "bg-green-500", dot: "bg-green-500" },
  surplus: { fill: "bg-purple-500", dot: "bg-purple-500" },
  no_sales: { fill: "bg-stone-500", dot: "bg-stone-500" },
};

const METRIC_TONE_CLASS = {
  revenue: {
    border: "border-l-blue-600",
    icon: "bg-blue-50 text-blue-600 ring-blue-100",
    value: "text-slate-950",
  },
  critical: {
    border: "border-l-red-600",
    icon: "bg-red-50 text-red-600 ring-red-100",
    value: "text-red-600",
  },
  short: {
    border: "border-l-orange-600",
    icon: "bg-orange-50 text-orange-600 ring-orange-100",
    value: "text-orange-600",
  },
  control: {
    border: "border-l-slate-500",
    icon: "bg-slate-100 text-slate-500 ring-slate-200",
    value: "text-slate-950",
  },
  stable: {
    border: "border-l-green-600",
    icon: "bg-green-50 text-green-600 ring-green-100",
    value: "text-green-600",
  },
} as const;

const ZERO_BUCKET_WIDTH_PX = 6;
const MIN_NONZERO_SEGMENT_WIDTH_PX = 8;

type Props = {
  stockCoverage: DashboardHomeResponse["ai_diagnostics"]["stock_coverage"];
};

export function StockCoverageHeaderHint() {
  return (
    <span
      className="inline-flex shrink-0 text-slate-500"
      title="Покрытие показывает, на сколько дней хватит текущего остатка при сохранении текущего темпа продаж."
      aria-label="Покрытие показывает, на сколько дней хватит текущего остатка при сохранении текущего темпа продаж."
    >
      <CircleHelp className="h-5 w-5" aria-hidden="true" />
    </span>
  );
}

function InfoHint({ title, className }: { title: string; className?: string }) {
  return (
    <span
      className={cn("inline-flex shrink-0 text-slate-400", className)}
      title={title}
      aria-label={title}
    >
      <CircleHelp className="h-3.5 w-3.5" aria-hidden="true" />
    </span>
  );
}

function StockCoverageDiagnosisIcon({ tone }: { tone: "risk" | "calm" }) {
  const fill = tone === "risk" ? "#3B82F6" : "#16A34A";
  const bgClass =
    tone === "risk"
      ? "bg-blue-100 ring-blue-200/70"
      : "bg-green-50 ring-green-200/70";

  return (
    <div
      data-testid="stock-coverage-summary-icon"
      className={cn(
        "flex h-[4.5rem] w-[4.5rem] shrink-0 items-center justify-center self-start rounded-full ring-1 ring-inset sm:h-[5.25rem] sm:w-[5.25rem]",
        bgClass,
      )}
      aria-hidden="true"
    >
      <svg
        viewBox="0 0 80 80"
        className="h-[3.25rem] w-[3.25rem] sm:h-[3.75rem] sm:w-[3.75rem]"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <rect x="11" y="44" width="10" height="13" rx="2.5" fill={fill} />
        <rect x="23" y="35" width="10" height="22" rx="2.5" fill={fill} />
        <rect x="35" y="39" width="10" height="18" rx="2.5" fill={fill} />
        <rect x="47" y="31" width="10" height="26" rx="2.5" fill={fill} />

        <path
          d="M16 33.5L28 24.5L40 28.5L52 22"
          stroke={fill}
          strokeWidth="2.75"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <circle cx="16" cy="33.5" r="3.25" fill={fill} />
        <circle cx="28" cy="24.5" r="3.25" fill={fill} />
        <circle cx="40" cy="28.5" r="3.25" fill={fill} />
        <circle cx="52" cy="22" r="3.25" fill={fill} />

        {tone === "risk" ? (
          <g transform="translate(46, 44)">
            <path
              d="M14 2.5C14.9 1 17.1 1 18 2.5L27.2 18.5C28.1 20 27 22 25.1 22H6.9C5 22 3.9 20 4.8 18.5L14 2.5Z"
              fill={fill}
            />
            <rect x="14.25" y="9" width="2.5" height="7.5" rx="1.25" fill="white" />
            <circle cx="15.5" cy="19.5" r="1.5" fill="white" />
          </g>
        ) : (
          <g transform="translate(48, 46)">
            <circle cx="12" cy="12" r="11" fill={fill} />
            <path
              d="M8 12.5L11 15.5L17 9.5"
              stroke="white"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </g>
        )}
      </svg>
    </div>
  );
}

function StockCoverageMainDiagnosis({
  narrative,
  segments,
  riskCount,
}: {
  narrative: StockCoverageNarrative;
  segments: SegmentCountSource[];
  riskCount: number;
}) {
  const tone = getCoverageDiagnosisTone(segments);
  const title = buildDiagnosisTitle(narrative, segments);
  const description = buildDiagnosisDescription(narrative, segments);
  const showPrimaryCTA = shouldShowRiskPrimaryCTA(segments);

  return (
    <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:gap-5">
      <StockCoverageDiagnosisIcon tone={tone} />
      <div className="min-w-0 flex-1 space-y-4">
        <div className="space-y-2">
          <h3
            data-testid="stock-coverage-diagnosis-title"
            className={cn(
              "text-xl font-bold leading-tight sm:text-2xl",
              tone === "risk" ? "text-blue-700" : "text-slate-950",
            )}
          >
            {title}
          </h3>
          <p
            data-testid="stock-coverage-diagnosis-description"
            className="text-base leading-[1.55] text-slate-700 sm:text-[17px]"
          >
            {description}
          </p>
        </div>
        {showPrimaryCTA ? (
          <a
            data-testid="stock-coverage-primary-cta"
            data-sku-focus-target={narrative.sku_focus_link?.target ?? "sku-focus-stock"}
            href={stockRiskFocusHref(narrative)}
            onClick={(event) => {
              event.preventDefault();
              navigateToSKUFocusTarget(narrative.sku_focus_link?.target ?? "sku-focus-stock", {
                showAll: true,
                filter: "all-risk",
              });
            }}
            className={cn(
              buttonClassNames("primary", "h-11 w-full gap-2 px-5 sm:w-auto sm:min-w-[12rem]"),
              dashboardTypography.button,
            )}
            aria-label={buildRiskCTALabel(riskCount, narrative)}
          >
            {buildRiskCTALabel(riskCount, narrative)}
            <ArrowRight className="h-4 w-4" aria-hidden="true" data-testid="stock-coverage-primary-cta-icon" />
          </a>
        ) : null}
      </div>
    </div>
  );
}

function StockCoverageNoSalesSignal({ narrative }: { narrative: StockCoverageNarrative }) {
  const count = narrative.no_sales_with_stock?.count ?? 0;
  if (!hasNoSalesTarget(narrative)) return null;

  const noSalesInsight = getNoSalesInsightText(narrative);

  return (
    <div
      data-testid="stock-coverage-no-sales-card"
      className="flex h-full flex-col justify-between rounded-[18px] border border-slate-200 bg-white/80 p-5 sm:p-6"
    >
      <div className="flex items-start gap-4">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-slate-100 text-slate-500 ring-1 ring-inset ring-slate-200">
          <PackageX className="h-6 w-6" aria-hidden="true" />
        </div>
        <div className="min-w-0 space-y-1.5">
          <p
            data-testid="stock-coverage-no-sales-title"
            className="text-xl font-bold leading-tight text-slate-900 sm:text-2xl"
          >
            {buildNoSalesTitle(count)}
          </p>
          <p className="text-sm leading-relaxed text-slate-600 sm:text-[15px]">
            Отдельная проблема оборачиваемости
          </p>
          {noSalesInsight.kind === "ai" ? (
            <p
              className="pt-1 text-sm leading-relaxed text-slate-600"
              data-testid="stock-coverage-no-sales-ai"
            >
              {noSalesInsight.text}
            </p>
          ) : null}
          {noSalesInsight.kind === "factual" ? (
            <p
              className="sr-only"
              data-testid="stock-coverage-no-sales-factual"
            >
              {noSalesInsight.text}
            </p>
          ) : null}
        </div>
      </div>
      <a
        data-testid="stock-coverage-secondary-cta"
        data-sku-focus-target={narrative.no_sales_focus_link?.target ?? "sku-focus-no-sales"}
        href={noSalesFocusHref(narrative)}
        onClick={(event) => {
          event.preventDefault();
          navigateToSKUFocusTarget(narrative.no_sales_focus_link?.target ?? "sku-focus-no-sales");
        }}
        className={cn(
          "mt-5 inline-flex items-center gap-1.5 text-sm font-semibold text-blue-600 transition-colors hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40",
          dashboardTypography.ctaLink,
        )}
        aria-label={buildNoSalesCTALabel(count, narrative)}
      >
        {buildNoSalesCTALabel(count, narrative)}
        <ArrowRight className="h-4 w-4 shrink-0" aria-hidden="true" data-testid="stock-coverage-secondary-cta-icon" />
      </a>
    </div>
  );
}

function StockCoverageDiagnosisPanel({
  narrative,
  segments,
  riskCount,
}: {
  narrative: StockCoverageNarrative;
  segments: SegmentCountSource[];
  riskCount: number;
}) {
  const showNoSales = hasNoSalesTarget(narrative);

  return (
    <div
      data-testid="stock-coverage-diagnosis-panel"
      className={cn(
        "grid gap-6 rounded-[20px] border border-blue-200 bg-gradient-to-r from-blue-50/70 via-white to-blue-50/40 p-6",
        showNoSales ? "lg:grid-cols-[minmax(0,1.7fr)_minmax(280px,0.9fr)] lg:items-stretch" : "",
      )}
    >
      <StockCoverageMainDiagnosis narrative={narrative} segments={segments} riskCount={riskCount} />
      {showNoSales ? <StockCoverageNoSalesSignal narrative={narrative} /> : null}
    </div>
  );
}

function MetricIconBadge({ icon: Icon, tone }: { icon: LucideIcon; tone: keyof typeof METRIC_TONE_CLASS }) {
  const toneClass = METRIC_TONE_CLASS[tone].icon;
  return (
    <div
      className={cn(
        "flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ring-1 ring-inset [&_svg]:h-5 [&_svg]:w-5",
        toneClass,
      )}
    >
      <Icon aria-hidden="true" />
    </div>
  );
}

function StockCoverageMetricCard({
  testId,
  label,
  value,
  range,
  icon,
  tone,
  infoTooltip,
  valueSize = "default",
}: {
  testId: string;
  label: string;
  value: string;
  range?: string;
  icon: LucideIcon;
  tone: keyof typeof METRIC_TONE_CLASS;
  infoTooltip?: string;
  valueSize?: "default" | "balanced";
}) {
  const toneClass = METRIC_TONE_CLASS[tone];
  const valueClass =
    valueSize === "balanced"
      ? "mt-1.5 text-[1.2rem] font-bold tabular-nums leading-snug tracking-tight sm:text-[1.35rem] xl:text-[1.4rem]"
      : "mt-1 text-[1.75rem] font-bold tabular-nums leading-tight sm:text-[1.9rem]";
  const valueParts =
    valueSize === "balanced"
      ? value.split(/\s*·\s*/u).map((part) => part.trim()).filter(Boolean)
      : [value];
  return (
    <div
      data-testid={testId}
      className={cn(
        "flex gap-4 rounded-2xl border border-slate-200 border-l-[3px] bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.035)]",
        toneClass.border,
      )}
    >
      <MetricIconBadge icon={icon} tone={tone} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <p className="text-sm font-medium text-slate-600">{label}</p>
          {infoTooltip ? <InfoHint title={infoTooltip} /> : null}
        </div>
        <p className={cn(valueClass, toneClass.value)}>
          {valueParts.length > 1 ? (
            valueParts.map((part, index) => (
              <span key={`${part}-${index}`} className="block">
                {index > 0 ? <span className="sr-only"> · </span> : null}
                {part}
              </span>
            ))
          ) : (
            value
          )}
        </p>
        {range ? <p className="mt-1 text-sm text-slate-600">{range}</p> : null}
      </div>
    </div>
  );
}

function StockCoverageMetricsRow({
  narrative,
  segments,
}: {
  narrative: StockCoverageNarrative;
  segments: SegmentCountSource[];
}) {
  const crit = segmentCountByKey(segments, "critical_0_3");
  const soon = segmentCountByKey(segments, "shortage_4_7");
  const control = segmentCountByKey(segments, "planning_8_14");
  const stable = segmentCountByKey(segments, "healthy_14plus");
  const hasRisk = shouldShowRiskPrimaryCTA(segments);

  const revenueAvailable = narrative.revenue_at_risk?.status === "available";
  const revenueValue = narrative.revenue_at_risk?.value ?? "—";
  const showRevenue = hasRisk;

  return (
    <div
      data-testid="stock-coverage-metrics-row"
      className={cn(
        "grid grid-cols-1 gap-3 sm:grid-cols-2",
        showRevenue
          ? "xl:grid-cols-[1.35fr_repeat(4,minmax(0,1fr))]"
          : "xl:grid-cols-4",
      )}
    >
      {showRevenue ? (
        <StockCoverageMetricCard
          testId="stock-coverage-kpi-revenue"
          label="Выручка под риском"
          value={revenueAvailable ? revenueValue : "Нет данных"}
          icon={CircleDollarSign}
          tone="revenue"
          valueSize="balanced"
          infoTooltip="Оценка среднедневной выручки SKU с покрытием до 7 дней. Значение может отличаться от общей дневной выручки кабинета."
        />
      ) : null}
      <StockCoverageMetricCard
        testId="stock-coverage-kpi-critical"
        label="Критично"
        value={formatSKUCount(crit)}
        range="0–3 дня"
        icon={CalendarClock}
        tone="critical"
      />
      <StockCoverageMetricCard
        testId="stock-coverage-kpi-shortage"
        label="Короткий запас"
        value={formatSKUCount(soon)}
        range="4–7 дней"
        icon={Timer}
        tone="short"
      />
      <StockCoverageMetricCard
        testId="stock-coverage-kpi-control"
        label="Под контролем"
        value={formatSKUCount(control)}
        range="8–14 дней"
        icon={CalendarCheck}
        tone="control"
      />
      <StockCoverageMetricCard
        testId="stock-coverage-kpi-stable"
        label="Стабильно"
        value={formatSKUCount(stable)}
        range=">14 дней"
        icon={ShieldCheck}
        tone="stable"
      />
    </div>
  );
}

function DistributionSegment({ bucket }: { bucket: CoverageDistributionBucket }) {
  const colors = BAR_SEGMENT_CLASS[bucket.key] ?? { fill: "bg-slate-400", dot: "bg-slate-400" };
  const isZero = bucket.count <= 0;

  const style = isZero
    ? { flex: `0 0 ${ZERO_BUCKET_WIDTH_PX}px`, width: ZERO_BUCKET_WIDTH_PX }
    : {
        flex: `${bucket.count} 1 0%`,
        minWidth: MIN_NONZERO_SEGMENT_WIDTH_PX,
      };

  return (
    <div
      data-testid={
        bucket.key === "planning_8_14" && isZero
          ? "stock-coverage-zero-bucket"
          : `stock-coverage-bar-segment-${bucket.key}`
      }
      data-segment-count={bucket.count}
      className={cn(
        "h-full first:rounded-l-md last:rounded-r-md",
        isZero ? "bg-slate-200" : colors.fill,
      )}
      style={style}
      title={`${bucket.label}: ${bucket.count}`}
      aria-hidden="true"
    />
  );
}

function StockCoverageDistribution({
  narrative,
  totalSkus,
}: {
  narrative: StockCoverageNarrative;
  totalSkus?: number;
}) {
  const buckets = buildCoverageDistributionBuckets(narrative.segments ?? stockCoverageSegmentsFallback(narrative));
  const positiveSum = buckets.reduce((acc, b) => acc + (b.count > 0 ? b.count : 0), 0);
  const total = totalSkus && totalSkus > 0 ? totalSkus : positiveSum;
  const barSummary = buildDistributionBarSummary(buckets, total);

  return (
    <div data-testid="stock-coverage-distribution" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-base font-semibold text-slate-800 sm:text-[17px]">Распределение покрытия</h3>
        {total > 0 ? (
          <span className="text-sm tabular-nums text-slate-600 sm:text-[15px]">Всего: {total} SKU</span>
        ) : null}
      </div>
      {positiveSum <= 0 ? (
        <p className={dashboardTypography.secondary}>Недостаточно данных для распределения по покрытиям.</p>
      ) : (
        <>
          <div
            data-testid="stock-coverage-distribution-bar"
            className="flex h-[18px] overflow-hidden rounded-md bg-slate-100 ring-1 ring-inset ring-slate-200/80"
            role="img"
            aria-label={barSummary}
          >
            <span className="sr-only">{barSummary}</span>
            {buckets.map((bucket) => (
              <DistributionSegment key={bucket.key} bucket={bucket} />
            ))}
          </div>
          <div className="flex flex-wrap gap-x-5 gap-y-2">
            {buckets.map((bucket) => {
              const colors = BAR_SEGMENT_CLASS[bucket.key] ?? { fill: "bg-slate-400", dot: "bg-slate-400" };
              return (
                <div
                  key={bucket.key}
                  className="flex items-center gap-2 text-sm text-slate-700 sm:text-[15px]"
                >
                  <span
                    className={cn(
                      "inline-block h-2.5 w-2.5 rounded-full",
                      bucket.count > 0 ? colors.dot : "bg-slate-300",
                    )}
                    aria-hidden="true"
                  />
                  <span>
                    {bucket.label} · <span className="font-semibold tabular-nums">{bucket.count}</span>
                  </span>
                </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}

function stockCoverageSegmentsFallback(narrative: StockCoverageNarrative) {
  return asArray(narrative.segments);
}

export function StockCoverageSkeleton() {
  return (
    <div className="animate-pulse space-y-6" data-testid="stock-coverage-skeleton">
      <div className="h-40 rounded-[20px] bg-slate-100" />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-28 rounded-2xl bg-slate-100" />
        ))}
      </div>
      <div className="space-y-3">
        <div className="h-5 w-48 rounded bg-slate-100" />
        <div className="h-[18px] rounded-md bg-slate-100" />
        <div className="flex gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-4 w-24 rounded bg-slate-100" />
          ))}
        </div>
      </div>
    </div>
  );
}

export function StockCoverageModule({ stockCoverage }: Props) {
  const narrative = stockCoverage.stock_coverage_narrative;
  if (!narrative || !isWidgetAvailable(narrative.status)) return null;

  const segments = asArray(narrative.segments) as SegmentCountSource[];
  const riskCount = getRiskSKUCount(segments);

  return (
    <div className="space-y-6" data-testid="stock-coverage-module">
      <StockCoverageDiagnosisPanel narrative={narrative} segments={segments} riskCount={riskCount} />
      <StockCoverageMetricsRow narrative={narrative} segments={segments} />
      <StockCoverageDistribution narrative={narrative} totalSkus={stockCoverage.total_skus} />
    </div>
  );
}
