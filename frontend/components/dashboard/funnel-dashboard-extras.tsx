"use client";

import Link from "next/link";
import { useState, type ComponentType } from "react";
import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  ChevronRight,
  CircleAlert,
  Eye,
  FileText,
  Lightbulb,
  Package,
  ShoppingCart,
  Target,
  Truck,
  Zap,
} from "lucide-react";
import { cn } from "@/components/ui/cn";
import { asArray } from "@/lib/as-array";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { SalesFunnelViewModel } from "@/lib/sales-funnel-view-model";
import {
  resolveFunnelDailyStageComparison,
  type FunnelDailyPoint,
} from "@/lib/funnel-daily-view-model";
import {
  buildSalesFunnelDisplay,
  getFunnelStageIconName,
  type FunnelStageDisplay,
  type FunnelStageIconName,
} from "@/lib/sales-funnel-display";
import { sanitizeSellerFacingText } from "@/lib/seller-facing-text";

type Props = {
  data: DashboardHomeResponse;
  viewModel: SalesFunnelViewModel;
};

type ActionRow = {
  id: string;
  title: string;
  href: string;
  label?: string;
};

const ICONS: Record<FunnelStageIconName, ComponentType<{ className?: string; "aria-hidden"?: boolean }>> = {
  eye: Eye,
  "file-text": FileText,
  "shopping-cart": ShoppingCart,
  package: Package,
  truck: Truck,
  circle: Target,
};

function cleanText(value?: string | null): string {
  return sanitizeSellerFacingText(value ?? "").trim();
}

function uniqueText(values: Array<string | null | undefined>, limit: number): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const value of values) {
    const text = cleanText(value);
    const key = text.toLocaleLowerCase("ru-RU");
    if (!text || seen.has(key)) continue;
    seen.add(key);
    result.push(text);
    if (result.length >= limit) break;
  }
  return result;
}

function formatDailyDate(date: string): string {
  const [year, month, day] = date.split("-");
  return year && month && day ? `${day}.${month}.${year}` : date;
}

function FunnelKpiSparkline({ points, display }: { points: FunnelDailyPoint[]; display: FunnelStageDisplay }) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  if (points.length < 2) {
    return <div className="h-9 w-[76px]" aria-label={`Динамика ${display.label}: нет истории`} />;
  }

  const values = points.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const coords = points.map((point, index) => ({
    ...point,
    x: 5 + (index / (points.length - 1)) * 66,
    y: 5 + (1 - (point.value - min) / range) * 24,
  }));
  const active = activeIndex == null ? null : coords[activeIndex] ?? null;

  return (
    <div className="relative h-9 w-[76px] shrink-0" data-testid="funnel-kpi-sparkline">
      <svg
        viewBox="0 0 76 34"
        className="h-full w-full"
        role="img"
        aria-label={`Динамика: ${display.label}`}
        onMouseLeave={() => setActiveIndex(null)}
      >
        <polyline
          points={coords.map((point) => `${point.x},${point.y}`).join(" ")}
          fill="none"
          stroke="#0873f6"
          strokeWidth="1.7"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {coords.map((point, index) => (
          <g key={point.date}>
            <circle cx={point.x} cy={point.y} r={activeIndex === index ? 2.8 : 2} fill="#0873f6" />
            <rect
              x={Math.max(0, point.x - 6)}
              y={0}
              width={12}
              height={34}
              fill="transparent"
              tabIndex={0}
              role="button"
              aria-label={`${formatDailyDate(point.date)}: ${new Intl.NumberFormat("ru-RU").format(point.value)}`}
              onMouseEnter={() => setActiveIndex(index)}
              onFocus={() => setActiveIndex(index)}
              onBlur={() => setActiveIndex(null)}
            />
          </g>
        ))}
      </svg>
      {active ? (
        <div
          role="tooltip"
          className="pointer-events-none absolute bottom-full left-1/2 z-30 mb-1 min-w-[7.5rem] -translate-x-1/2 rounded-md bg-slate-800 px-2 py-1 text-center text-[10px] leading-tight text-white shadow-md"
          data-testid="funnel-kpi-tooltip"
        >
          <span className="block text-white/70">{formatDailyDate(active.date)}</span>
          <span className="font-semibold tabular-nums">
            {new Intl.NumberFormat("ru-RU").format(active.value)}
          </span>
        </div>
      ) : null}
    </div>
  );
}

export function FunnelKpiRow({ data }: { data: DashboardHomeResponse }) {
  const rawStages = asArray(data.ai_diagnostics.sales_funnel.stages).slice(0, 5);
  const displayStages = buildSalesFunnelDisplay(data.ai_diagnostics.sales_funnel.status, rawStages);

  return (
    <section
      className="grid grid-cols-1 gap-2.5 min-[520px]:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5"
      aria-label="Показатели воронки"
      data-testid="funnel-kpi-row"
    >
      {displayStages.map((display, index) => {
        const raw = rawStages[index]!;
        const comparison = resolveFunnelDailyStageComparison({
          salesFunnel: data.ai_diagnostics.sales_funnel,
          stage: raw,
          metricDate: data.metric_date,
        });
        const comparisonText =
          comparison.deltaPct == null
            ? "Нет данных для сравнения с предыдущим днём"
            : `${comparison.deltaPct > 0 ? "↑" : comparison.deltaPct < 0 ? "↓" : ""} ${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(Math.abs(comparison.deltaPct))}% к предыдущему дню`;
        const Icon = ICONS[getFunnelStageIconName(raw.key)] ?? Target;
        return (
          <article
            key={display.key}
            className="min-w-0 rounded-[12px] border border-aivion-border bg-white px-3 py-3"
            data-testid={`funnel-kpi-${display.key}`}
          >
            <div className="flex items-center gap-1.5 text-aivion-muted">
              <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden />
              <h2 className="min-w-0 text-[11px] font-medium leading-tight text-aivion-text sm:text-xs">
                {display.label}
              </h2>
            </div>
            <div className="mt-2 flex items-center justify-between gap-2">
              <p className="min-w-0 text-xl font-bold tabular-nums tracking-tight text-aivion-text">
                {display.valueLabel}
              </p>
              <FunnelKpiSparkline points={comparison.points} display={display} />
            </div>
            <p
              className={cn(
                "mt-1 text-[10px] font-semibold tabular-nums sm:text-[11px]",
                comparison.deltaPct != null && comparison.deltaPct > 0
                  ? "text-emerald-600"
                  : comparison.deltaPct != null && comparison.deltaPct < 0
                    ? "text-rose-500"
                    : "text-aivion-muted",
              )}
            >
              {comparison.deltaPct == null ? "—" : comparisonText}
            </p>
          </article>
        );
      })}
    </section>
  );
}

function buildActionRows(data: DashboardHomeResponse, limit = 3): ActionRow[] {
  const rootActions = asArray(data.top_actions);
  const diagnosticActions = asArray(data.ai_diagnostics.top_actions);
  const source = rootActions.length > 0 ? rootActions : diagnosticActions;

  return source
    .filter((action) => {
      const category = (action.category ?? "").trim().toLowerCase();
      const type = (action.type ?? "").trim().toLowerCase();
      return category === "funnel" || type === "funnel";
    })
    .slice(0, limit)
    .map((action, index) => ({
      id: action.action_key || action.id || `funnel-action-${index}`,
      title: cleanText(action.title) || "Действие по воронке",
      href: "/app/actions",
      label: cleanText(action.category_label) || "Воронка",
    }));
}

function buildSupportingBullets(data: DashboardHomeResponse, viewModel: SalesFunnelViewModel): string[] {
  const analysis = data.ai_diagnostics.funnel_analysis.ai_analysis;
  const weak = viewModel.weakTransition;
  return uniqueText(
    [
      ...(viewModel.comparisonMode === "day_over_day" ? [] : asArray(analysis?.related_signals)),
      weak?.volumeDeltaLabel,
      weak?.conversionDeltaLabel
        ? `${weak.compactLabel}: изменение конверсии ${weak.conversionDeltaLabel}`
        : null,
      viewModel.comparisonMode === "day_over_day" ? null : viewModel.mainConclusion?.text,
      ...transitionSignal(viewModel),
    ],
    6,
  );
}

export function FunnelInsightActions({ data, viewModel }: Props) {
  const headline =
    viewModel.insightText ??
    (viewModel.comparisonMode === "day_over_day" ? null : viewModel.mainConclusion?.text) ??
    (viewModel.comparisonMode === "day_over_day" ? "" : cleanText(data.ai_diagnostics.funnel_analysis.primary_drop_message));
  const bullets = buildSupportingBullets(data, viewModel)
    .filter((bullet) => bullet.toLocaleLowerCase("ru-RU") !== headline.toLocaleLowerCase("ru-RU"))
    .slice(0, 3);

  return (
    <section className="min-w-0" data-testid="funnel-insight-actions">
      <article className="rounded-[12px] border border-aivion-border bg-white p-4">
        <div className="flex items-center gap-2">
          <Lightbulb className="h-4 w-4 text-aivion-blue" aria-hidden />
          <h2 className={dashboardTypography.cardTitleCompact}>Главный инсайт</h2>
        </div>
        {headline ? (
          <p className="mt-2.5 text-sm font-semibold leading-relaxed text-aivion-text sm:text-[15px]">
            {headline}
          </p>
        ) : (
          <p className={cn("mt-2.5", dashboardTypography.emptyState)}>Недостаточно данных для вывода.</p>
        )}
        {bullets.length > 0 ? (
          <ul className="mt-2 space-y-1.5">
            {bullets.map((bullet) => (
              <li key={bullet} className="flex gap-2 text-xs leading-relaxed text-aivion-muted sm:text-[13px]">
                <span className="mt-[0.45rem] h-1 w-1 shrink-0 rounded-full bg-aivion-blue" aria-hidden />
                <span>{bullet}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </article>
    </section>
  );
}

function transitionSignal(viewModel: SalesFunnelViewModel): string[] {
  return viewModel.transitions
    .slice()
    .sort((left, right) => {
      if (left.highlightKind === "weak_spot") return -1;
      if (right.highlightKind === "weak_spot") return 1;
      return Math.abs(right.conversionDeltaPp ?? right.volumeDeltaPct ?? 0) - Math.abs(left.conversionDeltaPp ?? left.volumeDeltaPct ?? 0);
    })
    .map((transition) => {
      const conversion = transition.conversionDeltaLabel
        ? `конверсия ${transition.conversionDeltaLabel}`
        : `конверсия ${transition.conversionLabel}`;
      const volume = transition.volumeDeltaPrimary ? `, объём ${transition.volumeDeltaPrimary}` : "";
      return `${transition.compactLabel}: ${conversion}${volume}.`;
    });
}

export function FunnelSignalsCard({ data, viewModel }: Props) {
  const analysis = data.ai_diagnostics.funnel_analysis.ai_analysis;
  const signals = uniqueText(
    [...(viewModel.comparisonMode === "day_over_day" ? [] : asArray(analysis?.related_signals)), ...transitionSignal(viewModel)],
    3,
  );
  return (
    <article className="rounded-[12px] border border-aivion-border bg-white p-4" data-testid="funnel-key-signals">
      <div className="flex items-center gap-2">
        <Zap className="h-4 w-4 text-aivion-blue" aria-hidden />
        <h2 className={dashboardTypography.cardTitleCompact}>Ключевые сигналы</h2>
      </div>
      <ul className="mt-2.5 space-y-2">
        {signals.map((signal, index) => {
          const transition = viewModel.transitions[index];
          const positive = (transition?.conversionDeltaPp ?? transition?.volumeDeltaPct ?? 0) > 0;
          const Icon = positive ? ArrowUp : index === 0 ? ArrowDown : CircleAlert;
          return (
            <li key={signal} className="flex gap-2 text-xs leading-relaxed text-aivion-text">
              <span className={cn("mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full", positive ? "bg-emerald-50 text-emerald-600" : index === 0 ? "bg-rose-50 text-rose-600" : "bg-blue-50 text-aivion-blue")}>
                <Icon className="h-3 w-3" aria-hidden />
              </span>
              <span>{signal}</span>
            </li>
          );
        })}
      </ul>
    </article>
  );
}

function FunnelRelatedActionsCard({ actions }: { actions: ActionRow[] }) {
  return (
    <article className="rounded-[12px] border border-aivion-border bg-white p-4" data-testid="funnel-related-actions">
      <div className="flex items-center gap-2">
        <Zap className="h-4 w-4 text-aivion-blue" aria-hidden />
        <h2 className={dashboardTypography.cardTitleCompact}>Связанные действия</h2>
      </div>
      <ul className="mt-2 divide-y divide-aivion-border">
        {actions.map((action) => (
          <li key={action.id}>
            <Link href={action.href} className="group flex min-h-10 items-center gap-2 py-1.5 text-xs leading-snug text-aivion-text">
              <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-sky-50 text-aivion-blue">
                <Target className="h-3 w-3" aria-hidden />
              </span>
              <span className="min-w-0 flex-1">{action.title}</span>
              {action.label ? (
                <span className="hidden shrink-0 rounded bg-blue-50 px-1.5 py-0.5 text-[9px] font-medium text-aivion-blue xl:inline-flex">
                  {action.label}
                </span>
              ) : null}
              <ChevronRight className="h-3.5 w-3.5 shrink-0 text-aivion-muted group-hover:text-aivion-blue" aria-hidden />
            </Link>
          </li>
        ))}
      </ul>
      <Link href="/app/actions" className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-aivion-blue">
        Все действия <ArrowRight className="h-3.5 w-3.5" aria-hidden />
      </Link>
    </article>
  );
}

export function FunnelBottomCards({ data }: Props) {
  const actions = buildActionRows(data);
  return (
    <section
      className={cn("grid grid-cols-1 gap-3", actions.length > 0 ? "xl:grid-cols-2" : "xl:grid-cols-1")}
      data-testid="funnel-bottom-cards"
      data-has-funnel-actions={actions.length > 0 ? "true" : "false"}
    >
      {actions.length > 0 ? <FunnelRelatedActionsCard actions={actions} /> : null}
    </section>
  );
}
