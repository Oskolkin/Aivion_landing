"use client";

import { useMemo } from "react";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { InfoNote } from "@/components/ui/info-note";
import { FunnelComparisonBars, type ComparisonBarsGroup } from "@/components/dashboard/funnel-comparison-bars";
import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import { cn } from "@/components/ui/cn";
import { buildFunnelAnalysisDisplayModel } from "@/lib/funnel-ai-analysis-view";
import { dashboardTypography } from "@/lib/dashboard-typography";
import { isWidgetAvailable } from "@/components/dashboard/widget-state";

type Props = {
  analysis: FunnelAIAnalysis;
  funnelAnalysisStatus?: string;
  primaryDropStage?: string;
  primaryDropMessage?: string;
  salesStages?: FunnelStage[];
};

export function FunnelAIAnalysisBlock({
  analysis,
  funnelAnalysisStatus,
  primaryDropMessage,
  primaryDropStage,
  salesStages,
}: Props) {
  return (
    <FunnelAIAnalysisContent
      analysis={analysis}
      funnelAnalysisStatus={funnelAnalysisStatus}
      primaryDropStage={primaryDropStage}
      primaryDropMessage={primaryDropMessage}
      salesStages={salesStages}
    />
  );
}

type ContentProps = {
  analysis: FunnelAIAnalysis;
  funnelAnalysisStatus?: string;
  primaryDropStage?: string;
  primaryDropMessage?: string;
  salesStages?: FunnelStage[];
  /** @deprecated expanded details UI removed */
  detailsOpen?: boolean;
  onDetailsOpenChange?: (open: boolean) => void;
  /** @deprecated use detailsOpen */
  isExpanded?: boolean;
  onToggle?: () => void;
};

function deltaToneClass(delta?: string): string {
  const src = (delta ?? "").trim();
  if (/^[-−]/.test(src)) return "text-rose-700";
  if (/^[+]/.test(src)) return "text-emerald-700";
  return "text-slate-700";
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h3 className={dashboardTypography.sectionTitle}>{children}</h3>;
}

function SectionShell({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("rounded-lg border border-slate-200 bg-slate-50/80 p-3 sm:p-3.5", className)}>{children}</section>
  );
}

function DeltaChip({ label, delta }: { label: string; delta: string }) {
  return (
    <span className={cn("inline-flex max-w-full flex-wrap items-center gap-1 rounded-full border border-slate-200 bg-white px-2 py-0.5", dashboardTypography.secondary)}>
      <span className={cn("font-semibold tabular-nums", deltaToneClass(delta))}>{delta}</span>
      <span className="text-slate-500">{label}</span>
    </span>
  );
}

function WeakSpotSection({
  transitionLabel,
  transitionSubline,
  conversionRate,
  conversionSubline,
  deltaPrevious,
  delta7d,
  explanation,
  volumeVsConversionNote,
}: {
  transitionLabel: string;
  transitionSubline?: string;
  conversionRate?: string;
  conversionSubline?: string;
  deltaPrevious?: string;
  delta7d?: string;
  explanation?: string;
  volumeVsConversionNote?: string;
}) {
  const hasNegative = /^[-−]/.test(deltaPrevious ?? "") || /^[-−]/.test(delta7d ?? "");
  return (
    <SectionShell
      className={cn(
        "border-l-[3px]",
        hasNegative ? "border-l-rose-400 bg-rose-50/40" : "border-l-slate-300 bg-slate-50",
      )}
    >
      <p className={dashboardTypography.aiSummaryLabel}>Главное ухудшение перехода</p>
      <p className={cn("mt-1.5", dashboardTypography.bodyEmphasis, "text-lg sm:text-xl")}>{transitionLabel}</p>
      {transitionSubline ? <p className={cn("mt-0.5", dashboardTypography.secondary)}>{transitionSubline}</p> : null}
      {conversionRate ? (
        <div className="mt-2 rounded-md border border-slate-200/90 bg-white/80 px-2.5 py-2">
          <p className={dashboardTypography.body}>
            <span className="font-medium text-slate-700">Конверсия перехода: </span>
            <span className="font-semibold tabular-nums text-slate-800">{conversionRate}</span>
          </p>
          {conversionSubline ? <p className={cn("mt-0.5", dashboardTypography.secondary)}>{conversionSubline}</p> : null}
        </div>
      ) : null}
      {(deltaPrevious || delta7d) && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {deltaPrevious ? <DeltaChip label="к прошлой сводке" delta={ensurePpSuffix(deltaPrevious)} /> : null}
          {delta7d ? <DeltaChip label="к среднему за 7 дней" delta={ensurePpSuffix(delta7d)} /> : null}
        </div>
      )}
      {volumeVsConversionNote ? <InfoNote className="mt-2">{volumeVsConversionNote}</InfoNote> : null}
      {explanation ? (
        <p className={cn("mt-2", dashboardTypography.body, "text-slate-700")}>
          <span className="font-medium text-slate-700">Что это значит: </span>
          <span>{explanation}</span>
        </p>
      ) : null}
    </SectionShell>
  );
}

function ensurePpSuffix(delta: string): string {
  const trimmed = delta.trim();
  if (!trimmed || /п\.п\./i.test(trimmed) || /%/.test(trimmed)) return trimmed;
  return `${trimmed} п.п.`;
}

export function FunnelAIAnalysisContent({
  analysis,
  primaryDropMessage,
  primaryDropStage,
  salesStages,
}: ContentProps) {
  const model = buildFunnelAnalysisDisplayModel(analysis, { primaryDropMessage, primaryDropStage, stages: salesStages });

  const previousGroups: ComparisonBarsGroup[] = useMemo(
    () => [
      { heading: "Этапы", subtitle: "изменение количества", items: model.comparisonGroups.stages },
      { heading: "Конверсии", subtitle: "изменение переходов, п.п.", items: model.comparisonGroups.conversions },
      { heading: "Экономика", subtitle: "деньги и средний чек", items: model.comparisonGroups.business },
    ],
    [model.comparisonGroups],
  );

  const sevenDayGroups: ComparisonBarsGroup[] = useMemo(
    () => [
      { heading: "Этапы", subtitle: "изменение количества", items: model.sevenDayComparisonGroups.stages },
      { heading: "Конверсии", subtitle: "изменение переходов, п.п.", items: model.sevenDayComparisonGroups.conversions },
      { heading: "Экономика", subtitle: "деньги и средний чек", items: model.sevenDayComparisonGroups.business },
    ],
    [model.sevenDayComparisonGroups],
  );

  const historyNote = model.contextSignals.find((s) => s.key === "history_status")?.value?.trim();
  const weakestStage = model.weakestStage;
  const transitionLabel = weakestStage
    ? weakestStage.label || weakestStage.metric || "Слабое место воронки"
    : "";

  if (!isWidgetAvailable(analysis.status)) {
    return null;
  }

  return (
    <div className={cn("w-full space-y-3 rounded-lg border border-slate-200 bg-white p-3.5 shadow-sm sm:p-4", dashboardTypography.body)}>
      <header className="flex flex-wrap items-center gap-2">
        <h2 className={dashboardTypography.sectionTitle}>Что это значит</h2>
        <Badge tone="info" className={dashboardTypography.badge}>
          AI
        </Badge>
        {process.env.NODE_ENV === "development" ? (
          <span data-testid="funnel-ai-debug-version" className="text-[10px] text-slate-400">
            funnel-ai-skeleton-v3
          </span>
        ) : null}
      </header>

      {model.comparisonsCommentary.previousPeriod ? (
        <p className={dashboardTypography.body}>{model.comparisonsCommentary.previousPeriod}</p>
      ) : null}

      {weakestStage ? (
        <div className="space-y-2">
          <SectionTitle>Что могло усилить просадку</SectionTitle>
          <WeakSpotSection
            transitionLabel={transitionLabel}
            transitionSubline={weakestStage.labelSubline}
            conversionRate={weakestStage.conversionRate}
            conversionSubline={weakestStage.conversionSubline}
            deltaPrevious={weakestStage.deltaPrevious}
            delta7d={weakestStage.delta7d}
            explanation={weakestStage.explanation}
            volumeVsConversionNote={weakestStage.volumeVsConversionNote}
          />
        </div>
      ) : null}

      <div className="grid gap-3 lg:grid-cols-2">
        <FunnelComparisonBars
          title="К прошлой сводке"
          commentary={model.comparisonsCommentary.previousPeriod}
          items={[]}
          groups={previousGroups}
          emptyText="Нет достаточно данных для сравнения с прошлой сводкой."
        />
        <FunnelComparisonBars
          title="К среднему за 7 дней"
          commentary={model.comparisonsCommentary.sevenDayAverage}
          items={[]}
          groups={sevenDayGroups}
          note={historyNote}
          emptyText="Пока недостаточно истории для сравнения со средним уровнем."
        />
      </div>
    </div>
  );
}
