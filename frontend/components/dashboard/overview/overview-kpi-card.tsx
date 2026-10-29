"use client";

import { OverviewMiniLineChart } from "@/components/dashboard/overview/overview-mini-line-chart";
import { cn } from "@/components/ui/cn";
import { dashboardTypography, dashboardTextColor } from "@/lib/dashboard-typography";
import type { OverviewKpiCardView } from "@/lib/overview-view-model";

type Props = {
  kpi: OverviewKpiCardView;
};

const DELTA_CLASS: Record<OverviewKpiCardView["deltaMode"], string> = {
  positive: dashboardTextColor.positive,
  negative: dashboardTextColor.negative,
  warning: dashboardTextColor.warning,
  neutral: dashboardTextColor.body,
  missing: dashboardTextColor.muted,
};

export function OverviewKpiCard({ kpi }: Props) {
  return (
    <article
      className="flex flex-col rounded-[14px] border border-aivion-border bg-white p-3.5 sm:p-4"
      data-testid={`overview-kpi-${kpi.key}`}
      aria-label={`${kpi.label}: ${kpi.valueDisplay}${kpi.deltaText ? `, ${kpi.deltaText}` : ""}`}
    >
      <div className="flex items-start gap-2">
        <h3 className={dashboardTypography.kpiLabelPlain}>{kpi.label}</h3>
      </div>
      <div className="mt-2.5 min-w-0">
        <p className={cn(dashboardTypography.kpiValueCompact, "leading-none")}>{kpi.valueDisplay}</p>
        {kpi.deltaText ? (
          <p className={cn("mt-1.5", dashboardTypography.kpiDelta, DELTA_CLASS[kpi.deltaMode])}>
            {kpi.deltaText}
          </p>
        ) : null}
      </div>
      {kpi.hasSparkline ? (
        <div className="mt-3">
          <OverviewMiniLineChart
            points={kpi.sparkline}
            stroke={kpi.sparklineStroke}
            formatYTick={(value) => String(value)}
            formatTooltipValue={(value) => String(value)}
            formatTooltipDate={(d) => d}
            ariaLabel={`Динамика: ${kpi.label}`}
            heightClassName="h-[5.25rem]"
          />
        </div>
      ) : null}
    </article>
  );
}
