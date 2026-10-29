"use client";

import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Box,
  ChevronRight,
  CircleAlert,
  CircleCheck,
  CircleHelp,
  ClipboardList,
  Eye,
  Filter,
  Info,
  Lightbulb,
  Megaphone,
  Package,
  Sparkles,
  TrendingDown,
  Zap,
} from "lucide-react";
import { PulseKpiChart } from "@/components/dashboard/pulse-kpi-chart";
import { cn } from "@/components/ui/cn";
import { StatusPill } from "@/components/ui/status-pill";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { DashboardHomeResponse, GlobalStatus } from "@/lib/dashboard-home-types";
import { formatPeriodContextDisplay, type PeriodContextFields } from "@/lib/period-context-display";
import {
  buildPulseViewModel,
  formatNarrativeSentence,
  formatPulseReportDate,
  getPulseKnowledgeIconToneClass,
  getPulseMetricDeltaClass,
  getPulseSummaryIconClass,
  getPulseSummaryStripClass,
  type PulseMetricView,
  type PulseQuickAction,
  type PulseSummaryView,
  type SellerPulseViewModel,
} from "@/lib/seller-pulse-view";
import { renderMetricHighlightText } from "@/lib/metric-highlight";

type Props = {
  data: DashboardHomeResponse["seller_pulse"];
  metricDate?: string;
  dashboardStatus?: GlobalStatus;
  periodContext?: PeriodContextFields;
  /** @deprecated prefer periodContext structured fields */
  periodCaption?: string;
  /** 7-day totals matching the backend's ad_spend_share_total_revenue (weekly share) window. */
  adSpend7d?: number | null;
  storeRevenue7d?: number | null;
};

const KNOWLEDGE_EMPTY = "Ключевые наблюдения появятся после обновления данных.";
const ACTIONS_EMPTY = "Быстрые действия появятся после обновления рекомендаций.";

export function SellerPulseSection({
  data,
  metricDate,
  dashboardStatus,
  periodContext,
  periodCaption,
  adSpend7d,
  storeRevenue7d,
}: Props) {
  const view = buildPulseViewModel({
    data,
    dashboardStatus,
    adSpend7d,
    storeRevenue7d,
  });

  const knowledgeItems = view.knowledge.length > 0 ? view.knowledge : [{ text: KNOWLEDGE_EMPTY, tone: "neutral" }];
  const actionItems = view.quickActions.length > 0 ? view.quickActions : [{ label: ACTIONS_EMPTY }];
  const reportDate = formatPulseReportDate(metricDate);
  const periodDisplay = formatPeriodContextDisplay(
    periodContext ?? (periodCaption ? { caption_primary: periodCaption } : null),
  );
  const showPeriodContext = periodDisplay.groups.length > 0;

  return (
    <section aria-labelledby="seller-pulse-heading">
      <article className="overflow-hidden rounded-[22px] border border-aivion-border bg-white p-5 shadow-[0_8px_30px_rgb(6_27_69/0.06)] sm:p-6">
        <header className="space-y-3">
          <h2 id="seller-pulse-heading" className={dashboardTypography.moduleTitle}>
            Пульс продавца
          </h2>
          {showPeriodContext ? (
            <PeriodContextPanel groups={periodDisplay.groups} />
          ) : (
            <p className={dashboardTypography.moduleDescription}>
              {reportDate ? (
                <>
                  Анализ по данным на <span className="font-medium text-slate-700">{reportDate}</span>
                </>
              ) : (
                "Ключевые KPI за последние дни и что требует внимания"
              )}
            </p>
          )}
        </header>

        <div className="mt-5 space-y-5">
          {view.dataQualityNote ? (
            <p className={cn("rounded-2xl border border-[#b2ddff] bg-[#eff8ff] px-4 py-3", dashboardTypography.warningBanner, "text-[#175cd3]")}>
              {view.dataQualityNote}
            </p>
          ) : null}

          <PulseSummaryStrip summary={view.summary} keyPoints={view.keyPoints} />

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {view.metrics.map((metric) => (
              <PulseMetricCard key={metric.key} metric={metric} />
            ))}
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <PulseKnowledgeCard items={knowledgeItems} isEmpty={view.knowledge.length === 0} />
            <PulseQuickActionsCard actions={actionItems} isEmpty={view.quickActions.length === 0} />
          </div>
        </div>
      </article>
    </section>
  );
}

function PulseSummaryStrip({
  summary,
  keyPoints,
}: {
  summary: PulseSummaryView;
  keyPoints: SellerPulseViewModel["keyPoints"];
}) {
  const headline = stripTrailingPeriod(summary.headline);
  const summaryText = summary.summary ? formatNarrativeSentence(summary.summary) : null;
  const hideHeadline = shouldHidePulseNarrativeHeadline(headline);
  const showHeadline = Boolean(headline) && !hideHeadline;
  const points = keyPoints.filter((item) => Boolean(item.text?.trim()));

  return (
    <div
      className={cn(
        "rounded-[18px] border px-5 py-4",
        getPulseSummaryStripClass(summary.tone),
      )}
      data-testid="seller-pulse-ai-resume"
    >
      <div className="flex min-w-0 items-start gap-3.5">
        <div
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-white/90 shadow-sm ring-1 ring-inset ring-black/5",
            getPulseSummaryIconClass(summary.tone),
          )}
        >
          <Activity className="h-5 w-5" aria-hidden />
        </div>
        <div className="min-w-0 w-full max-w-[1100px] flex-1 space-y-3 sm:w-[min(100%,85%)]">
          <p
            className={cn(dashboardTypography.metadata, "font-medium uppercase tracking-[0.04em] text-slate-500")}
            data-testid="seller-pulse-ai-label"
          >
            AI-резюме по кабинету
          </p>
          {showHeadline ? (
            <h3
              className={cn(
                dashboardTypography.sectionTitle,
                "text-[1.05rem] leading-snug text-slate-900 sm:text-[1.125rem]",
              )}
              data-testid="seller-pulse-ai-headline"
            >
              {headline}
            </h3>
          ) : null}
          {summaryText ? (
            <p
              className={cn(dashboardTypography.aiSummary, "whitespace-normal break-words leading-relaxed")}
              data-testid="seller-pulse-ai-summary"
            >
              {renderMetricHighlightText(summaryText)}
            </p>
          ) : null}
          {points.length > 0 ? (
            <ul
              className={cn(
                "gap-x-8 gap-y-3 pt-1",
                points.length >= 2 ? "grid grid-cols-1 md:grid-cols-2" : "space-y-3",
              )}
              data-testid="seller-pulse-ai-key-points"
            >
              {points.map((item) => (
                <li key={`${item.title ?? ""}-${item.text}`} className="flex gap-2.5">
                  <span
                    className={cn(
                      "mt-2 h-1.5 w-1.5 shrink-0 rounded-full",
                      getPulseKnowledgeIconToneClass(item.tone).includes("red")
                        ? "bg-[#f04438]"
                        : getPulseKnowledgeIconToneClass(item.tone).includes("green")
                          ? "bg-[#12b76a]"
                          : "bg-[#0873f6]",
                    )}
                    aria-hidden
                  />
                  <div className="min-w-0 space-y-1">
                    {item.title ? (
                      <p className={cn(dashboardTypography.listItemTitle)} data-testid="seller-pulse-ai-key-point-title">
                        {item.title}
                      </p>
                    ) : null}
                    <p className={cn(dashboardTypography.listItemBody, "leading-relaxed")}>
                      {renderMetricHighlightText(item.text)}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function stripTrailingPeriod(text: string): string {
  return text.trim().replace(/[.…]+$/u, "").trim();
}

function shouldHidePulseNarrativeHeadline(headline: string): boolean {
  const normalized = headline.trim().toLowerCase().replace(/\.$/, "");
  return (
    normalized === "пульс продавца" ||
    normalized === "пульс селлера" ||
    normalized === "seller pulse"
  );
}

function PeriodContextPanel({ groups }: { groups: { label: string; value: string }[] }) {
  return (
    <div
      className="rounded-xl border border-slate-200/90 bg-slate-50/80 px-3.5 py-3 sm:px-4 sm:py-3.5"
      data-testid="seller-pulse-period-context"
    >
      <dl
        className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4"
        data-testid="seller-pulse-period-groups"
      >
        {groups.map((item) => (
          <div key={item.label} className="min-w-0">
            <dt className="text-[11px] font-medium uppercase tracking-[0.04em] text-slate-500">
              {item.label}
            </dt>
            <dd className="mt-0.5 text-sm font-semibold leading-snug text-slate-900 sm:text-[15px]">
              {item.value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function PulseInfoHint({ title }: { title: string }) {
  return (
    <span className="inline-flex shrink-0 text-aivion-muted" title={title} aria-label={title}>
      <CircleHelp className="h-3.5 w-3.5" aria-hidden="true" />
    </span>
  );
}

function PulseMetricCard({ metric }: { metric: PulseMetricView }) {
  return (
    <article
      className={cn(
        "flex h-full min-h-[12.5rem] flex-col overflow-hidden rounded-[18px] border border-aivion-border bg-white p-4 sm:p-5",
        metric.cardAccentClass,
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-start gap-1.5">
          <p className={dashboardTypography.kpiLabel}>{metric.label}</p>
          {metric.hint ? <PulseInfoHint title={metric.hint} /> : null}
        </div>
        <StatusPill variant={metric.pillVariant} size="xs" className={dashboardTypography.badge}>
          {metric.statusLabel}
        </StatusPill>
      </div>

      <p className={cn("mt-3", dashboardTypography.kpiValue)}>{metric.valueDisplay}</p>

      {metric.deltaText ? (
        <p className={cn("mt-2", dashboardTypography.kpiDelta, getPulseMetricDeltaClass(metric.state))}>
          {metric.deltaText}
        </p>
      ) : metric.hasSparkline ? (
        <p className={cn("mt-2", dashboardTypography.secondary)}>Динамика за 7 дней</p>
      ) : metric.state === "no_data" || metric.state === "unavailable" ? (
        <p className={cn("mt-2", dashboardTypography.secondary)}>
          {metric.state === "unavailable" ? "Динамика недоступна" : "Появится после синхронизации"}
        </p>
      ) : null}

      <div className="mt-auto pt-3">
        {metric.hasSparkline ? (
          <PulseKpiChart
            points={metric.sparkline}
            metricKey={metric.key}
            stroke={metric.sparklineStroke}
          />
        ) : (
          <SparklinePlaceholder label={metric.sparklineEmptyLabel} />
        )}
      </div>
    </article>
  );
}

function SparklinePlaceholder({ label }: { label: string }) {
  return (
    <div className="space-y-1.5">
      <svg viewBox="0 0 120 32" className="h-8 w-full text-slate-200" aria-hidden>
        <line
          x1="4"
          y1="24"
          x2="116"
          y2="12"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeDasharray="4 4"
          strokeLinecap="round"
        />
      </svg>
      <p className={dashboardTypography.metadata}>{label}</p>
    </div>
  );
}

function PulseKnowledgeCard({
  items,
  isEmpty,
}: {
  items: SellerPulseViewModel["knowledge"];
  isEmpty: boolean;
}) {
  return (
    <article className="flex h-full flex-col rounded-[18px] border border-aivion-border bg-aivion-surface-soft/70 p-5">
      <div className="flex items-center gap-2.5">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-[#0873f6] ring-1 ring-inset ring-aivion-border">
          <Lightbulb className="h-[18px] w-[18px]" aria-hidden />
        </div>
        <h3 className={dashboardTypography.sectionTitle}>Что важно знать</h3>
      </div>
      <ul className="mt-4 space-y-4">
        {items.map((item) => (
          <li key={`${item.title ?? ""}-${item.text}`} className="flex gap-3">
            <PulseKnowledgeIcon tone={item.tone} isEmpty={isEmpty} />
            <div className="min-w-0">
              {item.title ? (
                <p className={cn(dashboardTypography.listItemTitle, "mb-1")}>{item.title}</p>
              ) : null}
              <p
                className={cn(
                  dashboardTypography.listItemBody,
                  !item.title && "text-slate-800",
                )}
              >
                {item.text}
              </p>
            </div>
          </li>
        ))}
      </ul>
    </article>
  );
}

function PulseKnowledgeIcon({ tone, isEmpty }: { tone?: string; isEmpty: boolean }) {
  const className = cn(
    "mt-0.5 h-[18px] w-[18px] shrink-0",
    isEmpty ? "text-slate-300" : getPulseKnowledgeIconToneClass(tone),
  );

  switch ((tone ?? "").trim().toLowerCase()) {
    case "critical":
      return <AlertTriangle className={className} aria-hidden />;
    case "warning":
      return <CircleAlert className={className} aria-hidden />;
    case "positive":
      return <CircleCheck className={className} aria-hidden />;
    case "negative":
      return <TrendingDown className={className} aria-hidden />;
    case "neutral":
      return <Info className={className} aria-hidden />;
    default:
      return <Sparkles className={className} aria-hidden />;
  }
}

function QuickActionIcon({ kind }: { kind?: string }) {
  const className = "h-[18px] w-[18px] shrink-0";
  switch ((kind ?? "").trim().toLowerCase()) {
    case "advertising":
      return <Megaphone className={className} aria-hidden />;
    case "stock":
      return <Box className={className} aria-hidden />;
    case "sku":
      return <Package className={className} aria-hidden />;
    case "funnel":
      return <Filter className={className} aria-hidden />;
    case "recommendations":
      return <ClipboardList className={className} aria-hidden />;
    case "monitoring":
      return <Eye className={className} aria-hidden />;
    case "products":
      return <BarChart3 className={className} aria-hidden />;
    default:
      return <Zap className={className} aria-hidden />;
  }
}

function PulseQuickActionsCard({
  actions,
  isEmpty,
}: {
  actions: PulseQuickAction[];
  isEmpty: boolean;
}) {
  return (
    <article className="flex h-full flex-col rounded-[18px] border border-aivion-border bg-aivion-surface-soft/70 p-5">
      <div className="flex items-center gap-2.5">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-white text-[#0873f6] ring-1 ring-inset ring-aivion-border">
          <Zap className="h-[18px] w-[18px]" aria-hidden />
        </div>
        <h3 className={dashboardTypography.sectionTitle}>Быстрые действия</h3>
      </div>
      <ul className="mt-4 space-y-2">
        {actions.map((action) => {
          const isHighPriority = action.priority === "high";
          const rowClass = cn(
            "flex items-center gap-3 rounded-xl px-3 py-3.5 transition-colors",
            isHighPriority && !isEmpty ? "bg-[#fffaeb]/80 ring-1 ring-inset ring-[#fedf89]" : "hover:bg-white/80",
            isEmpty ? "text-slate-500" : "text-slate-800",
          );
          const content = (
            <>
              <div
                className={cn(
                  "flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-white text-slate-500 ring-1 ring-inset ring-aivion-border",
                  isHighPriority && !isEmpty ? "text-[#b54708]" : undefined,
                )}
              >
                <QuickActionIcon kind={action.kind} />
              </div>
              <div className="min-w-0 flex-1">
                <p className={dashboardTypography.listItemTitle}>{action.label}</p>
                {action.description ? (
                  <p className={cn("mt-1", dashboardTypography.listItemBody)}>{action.description}</p>
                ) : null}
              </div>
              {action.href ? (
                <ChevronRight className="h-5 w-5 shrink-0 text-slate-500" aria-hidden />
              ) : null}
            </>
          );

          return (
            <li key={action.label}>
              {action.href && !isEmpty ? (
                <Link href={action.href} className={rowClass}>
                  {content}
                </Link>
              ) : (
                <div className={rowClass}>{content}</div>
              )}
            </li>
          );
        })}
      </ul>
    </article>
  );
}
