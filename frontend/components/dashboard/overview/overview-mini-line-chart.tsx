"use client";

import { useMemo, useState } from "react";
import { OverviewChartTooltip } from "@/components/dashboard/overview/overview-chart-tooltip";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { SparklinePoint } from "@/lib/dashboard-home-types";
import { sparklineNumericValue } from "@/lib/sparkline-value";

const COMPACT = {
  viewWidth: 240,
  viewHeight: 88,
  yAxisWidth: 36,
  paddingTop: 6,
  paddingBottom: 4,
  paddingRight: 6,
  yTickCount: 3,
} as const;

const PANEL = {
  viewWidth: 320,
  viewHeight: 168,
  yAxisWidth: 40,
  paddingTop: 10,
  paddingBottom: 4,
  paddingRight: 16,
  yTickCount: 5,
} as const;

export type OverviewChartPoint = {
  date: string;
  value: number | null;
};

type LayoutPoint = {
  index: number;
  x: number;
  y: number;
  value: number;
  date: string;
};

type Props = {
  points: OverviewChartPoint[] | SparklinePoint[];
  stroke?: string;
  formatYTick: (value: number) => string;
  formatTooltipValue: (value: number) => string;
  formatTooltipDate: (date: string) => string;
  ariaLabel: string;
  className?: string;
  heightClassName?: string;
  /** Compact = KPI sparklines; panel = domain preview charts. */
  size?: "compact" | "panel";
};

function buildYTicks(min: number, max: number, count: number): number[] {
  if (count <= 2) return [max, min];
  const ticks: number[] = [];
  for (let i = 0; i < count; i += 1) {
    const t = max - ((max - min) * i) / (count - 1);
    ticks.push(t);
  }
  return ticks;
}

export function OverviewMiniLineChart({
  points,
  stroke = "#0873f6",
  formatYTick,
  formatTooltipValue,
  formatTooltipDate,
  ariaLabel,
  className,
  heightClassName,
  size = "compact",
}: Props) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const dims = size === "panel" ? PANEL : COMPACT;
  const resolvedHeight =
    heightClassName ?? (size === "panel" ? "h-full min-h-[11.5rem]" : "h-[5.5rem]");

  const layout = useMemo(() => {
    const numericValues = points
      .map((point) => sparklineNumericValue(point.value))
      .filter((value): value is number => value != null);
    if (numericValues.length < 2) return null;

    let min = Math.min(...numericValues);
    let max = Math.max(...numericValues);
    if (min === max) {
      min -= 1;
      max += 1;
    }

    const chartWidth = dims.viewWidth - dims.yAxisWidth - dims.paddingRight;
    const chartHeight = dims.viewHeight - dims.paddingTop - dims.paddingBottom;
    const range = max - min;
    const step = points.length > 1 ? chartWidth / (points.length - 1) : 0;
    const toY = (value: number) =>
      dims.paddingTop + chartHeight - ((value - min) / range) * chartHeight;

    const chartPoints: LayoutPoint[] = [];
    points.forEach((point, index) => {
      const value = sparklineNumericValue(point.value);
      if (value == null) return;
      chartPoints.push({
        index,
        x: dims.yAxisWidth + index * step,
        y: toY(value),
        value,
        date: point.date,
      });
    });
    if (chartPoints.length < 2) return null;

    const segments: LayoutPoint[][] = [];
    let current: LayoutPoint[] = [];
    points.forEach((point, index) => {
      const value = sparklineNumericValue(point.value);
      if (value == null) {
        if (current.length > 0) {
          segments.push(current);
          current = [];
        }
        return;
      }
      const existing = chartPoints.find((p) => p.index === index);
      if (existing) current.push(existing);
    });
    if (current.length > 0) segments.push(current);

    const yTicks = buildYTicks(min, max, dims.yTickCount);
    return {
      min,
      max,
      chartHeight,
      chartWidth,
      yTicks,
      segments,
      chartPoints,
      toY,
      step,
    };
  }, [points, dims]);

  if (!layout) return null;

  const active =
    activeIndex == null
      ? null
      : (layout.chartPoints.find((point) => point.index === activeIndex) ?? null);
  const chartRight = dims.yAxisWidth + layout.chartWidth;
  const chartBottom = dims.paddingTop + layout.chartHeight;
  const visibleYTicks = layout.yTicks.filter((tick, index, ticks) => {
    const label = formatYTick(tick);
    return ticks.findIndex((candidate) => formatYTick(candidate) === label) === index;
  });

  return (
    <div className={cn("relative w-full", className)}>
      <svg
        viewBox={`0 0 ${dims.viewWidth} ${dims.viewHeight}`}
        className={cn("w-full", resolvedHeight)}
        role="img"
        aria-label={ariaLabel}
        tabIndex={0}
        onMouseLeave={() => setActiveIndex(null)}
        onBlur={() => setActiveIndex(null)}
        data-testid="overview-mini-line-chart"
        data-size={size}
      >
        {visibleYTicks.map((tick, tickIndex) => {
          const y = layout.toY(tick);
          return (
            <g key={`y-${tickIndex}`}>
              {tickIndex > 0 && tickIndex < visibleYTicks.length - 1 ? (
                <line
                  x1={dims.yAxisWidth}
                  y1={y}
                  x2={chartRight}
                  y2={y}
                  stroke="#eef2f6"
                  strokeWidth="1"
                />
              ) : null}
              <text
                x={dims.yAxisWidth - 4}
                y={y + 3}
                textAnchor="end"
                className={dashboardTypography.chartAxis}
              >
                {formatYTick(tick)}
              </text>
            </g>
          );
        })}

        <line
          x1={dims.yAxisWidth}
          y1={chartBottom}
          x2={chartRight}
          y2={chartBottom}
          stroke="#e4e7ec"
          strokeWidth="1"
        />

        {layout.segments.map((segment, segmentIndex) => (
          <polyline
            key={segmentIndex}
            fill="none"
            stroke={stroke}
            strokeWidth={size === "panel" ? 2 : 1.75}
            strokeLinejoin="round"
            strokeLinecap="round"
            points={segment.map((point) => `${point.x},${point.y}`).join(" ")}
          />
        ))}

        {layout.chartPoints.map((point) => (
          <circle
            key={`pt-${point.index}`}
            cx={point.x}
            cy={point.y}
            r={
              active?.index === point.index
                ? size === "panel"
                  ? 3.75
                  : 3.25
                : size === "panel"
                  ? 2.5
                  : 1.75
            }
            fill={stroke}
            stroke="white"
            strokeWidth={active?.index === point.index ? 1.5 : 0.75}
          />
        ))}

        {active ? (
          <line
            x1={active.x}
            y1={dims.paddingTop}
            x2={active.x}
            y2={chartBottom}
            stroke="#d0d5dd"
            strokeWidth="1"
          />
        ) : null}

        {layout.chartPoints.map((point) => (
          <rect
            key={`hit-${point.index}`}
            x={point.x - Math.max(layout.step, 12) / 2}
            y={dims.paddingTop}
            width={Math.max(layout.step, 12)}
            height={layout.chartHeight}
            fill="transparent"
            tabIndex={0}
            role="button"
            aria-label={`${formatTooltipDate(point.date)}: ${formatTooltipValue(point.value)}`}
            onMouseEnter={() => setActiveIndex(point.index)}
            onFocus={() => setActiveIndex(point.index)}
            onClick={() => setActiveIndex(point.index)}
          />
        ))}
      </svg>

      {active ? (
        <OverviewChartTooltip
          dateLabel={formatTooltipDate(active.date)}
          valueLabel={formatTooltipValue(active.value)}
          className="absolute"
          style={{
            left: `${(active.x / dims.viewWidth) * 100}%`,
            top: 4,
            transform: "translate(-50%, -100%)",
          }}
        />
      ) : null}
    </div>
  );
}
