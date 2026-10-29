"use client";

import { useId, useMemo } from "react";
import { Info } from "lucide-react";
import { FunnelChart } from "@/components/dashboard/funnel-chart";
import { FunnelInsightBanner } from "@/components/dashboard/funnel-insight-banner";
import { FunnelTransitions } from "@/components/dashboard/funnel-transitions";
import { FunnelMainConclusion } from "@/components/dashboard/funnel-main-conclusion";
import { WidgetState } from "@/components/dashboard/widget-state";
import { asArray } from "@/lib/as-array";
import type { DashboardHomeResponse, FunnelStage } from "@/lib/dashboard-home-types";
import { buildFunnelStageChainSubtitle } from "@/lib/funnel-transitions-display";
import { mapSalesFunnelToViewModel } from "@/lib/sales-funnel-view-model";
import { dashboardTypography } from "@/lib/dashboard-typography";

type SalesFunnelModuleProps = {
  salesFunnel: DashboardHomeResponse["ai_diagnostics"]["sales_funnel"];
  funnelAnalysis: DashboardHomeResponse["ai_diagnostics"]["funnel_analysis"];
  compact?: boolean;
  stagesOverride?: FunnelStage[];
  comparisonMode?: "default" | "day_over_day";
};

const FUNNEL_INFO_TOOLTIP =
  "Процент конверсии показывает, какая доля предыдущего этапа перешла на следующий. Изменение в п.п. показывает динамику самой конверсии. Процент под количеством показывает, насколько изменилось число показов, открытий, добавлений, заказанных или доставленных единиц относительно предыдущего периода.";

function funnelFootnote(
  hasConversionDelta: boolean,
  hasCountDelta: boolean,
  comparisonMode: "default" | "day_over_day",
): string {
  const comparison = comparisonMode === "day_over_day" ? "предыдущего дня" : "предыдущего периода";
  if (hasConversionDelta && hasCountDelta) {
    return `Слева показано количество событий на каждом этапе. Справа — конверсия между соседними этапами, изменение конверсии в п.п. и изменение количества событий относительно ${comparison}.`;
  }
  return `Слева показано количество событий на каждом этапе. Справа — конверсия между соседними этапами и изменение количества событий относительно ${comparison}.`;
}

export function SalesFunnelModule({
  salesFunnel,
  funnelAnalysis,
  compact = false,
  stagesOverride,
  comparisonMode = "default",
}: SalesFunnelModuleProps) {
  const stages = stagesOverride ?? asArray(salesFunnel.stages);
  const viewModel = useMemo(
    () =>
      mapSalesFunnelToViewModel({
        status: salesFunnel.status,
        stages,
        funnelAnalysis: funnelAnalysis.ai_analysis,
        primaryDropStage: funnelAnalysis.primary_drop_stage,
        comparisonMode,
      }),
    [salesFunnel.status, stages, funnelAnalysis.ai_analysis, funnelAnalysis.primary_drop_stage, comparisonMode],
  );

  if (compact) {
    return (
      <WidgetState widget={salesFunnel}>
        <div className="min-w-0 max-w-full" data-testid="funnel-compact-module">
          <div className="grid min-w-0 grid-cols-1 items-start gap-5 xl:grid-cols-[minmax(0,45fr)_minmax(0,55fr)] xl:gap-6">
            <section aria-label="Воронка продаж" className="min-w-0">
              <FunnelChart
                compact
                status={salesFunnel.status}
                stages={stages}
                highlightedStageKey={viewModel.highlightedStageKey}
                highlightKind={viewModel.highlightKind}
              />
            </section>
            <FunnelTransitions compact transitions={viewModel.transitions} comparisonMode={comparisonMode} />
          </div>

          <div className="mt-3 flex items-start gap-2 border-t border-aivion-border px-0 pt-3 text-[11px] leading-relaxed text-aivion-muted sm:text-xs">
            <Info className="mt-0.5 h-4 w-4 shrink-0 text-aivion-blue" aria-hidden />
            <p>{funnelFootnote(viewModel.hasConversionDelta, viewModel.hasVolumeDelta, comparisonMode)}</p>
          </div>
        </div>
      </WidgetState>
    );
  }

  return (
    <WidgetState widget={salesFunnel}>
      <div className="min-w-0 max-w-full space-y-5">
        {viewModel.insightText ? <FunnelInsightBanner text={viewModel.insightText} /> : null}

        <div className="grid min-w-0 max-w-full grid-cols-1 items-start gap-7 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.95fr)] xl:gap-8">
          <FunnelChart
            status={salesFunnel.status}
            stages={stages}
            highlightedStageKey={viewModel.highlightedStageKey}
            highlightKind={viewModel.highlightKind}
          />
          <div className="min-w-0 max-w-full space-y-4">
            <FunnelTransitions transitions={viewModel.transitions} />
            {viewModel.mainConclusion ? (
              <FunnelMainConclusion
                text={viewModel.mainConclusion.text}
                source={viewModel.mainConclusion.source}
                highlightLabel={viewModel.weakTransition?.compactLabel}
              />
            ) : null}
          </div>
        </div>

        <p className={dashboardTypography.footerNote}>
          {funnelFootnote(viewModel.hasConversionDelta, viewModel.hasVolumeDelta, comparisonMode)}
        </p>
      </div>
    </WidgetState>
  );
}

export function buildSalesFunnelSubtitle(
  salesFunnel: DashboardHomeResponse["ai_diagnostics"]["sales_funnel"],
): string {
  return buildFunnelStageChainSubtitle(asArray(salesFunnel.stages));
}

export function SalesFunnelInfoHint() {
  const tooltipId = useId();
  return (
    <button
      type="button"
      className="relative inline-flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full text-aivion-muted transition-colors hover:bg-slate-100 hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 focus-visible:ring-offset-2"
      title={FUNNEL_INFO_TOOLTIP}
      aria-label="Справка по конверсии воронки"
      aria-describedby={tooltipId}
    >
      <Info className="h-4 w-4" aria-hidden="true" />
      <span id={tooltipId} className="sr-only">
        {FUNNEL_INFO_TOOLTIP}
      </span>
    </button>
  );
}
