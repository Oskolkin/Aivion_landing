"use client";

import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  dateLabel: string;
  valueLabel: string;
  metricLabel?: string;
  className?: string;
  style?: React.CSSProperties;
};

/** Shared dark tooltip for Overview mini-charts. */
export function OverviewChartTooltip({
  dateLabel,
  valueLabel,
  metricLabel,
  className,
  style,
}: Props) {
  return (
    <div
      role="tooltip"
      className={cn(
        "pointer-events-none z-20 min-w-[7rem] max-w-[11rem] rounded-md bg-[#1f2937] px-2.5 py-1.5 text-white shadow-md",
        dashboardTypography.chartTooltip,
        className,
      )}
      style={style}
      data-testid="overview-chart-tooltip"
    >
      {metricLabel ? <p className="text-[11px] text-white/70">{metricLabel}</p> : null}
      <p className="font-medium leading-snug text-white">{dateLabel}</p>
      <p className="mt-0.5 text-sm font-semibold tabular-nums text-white">{valueLabel}</p>
    </div>
  );
}
