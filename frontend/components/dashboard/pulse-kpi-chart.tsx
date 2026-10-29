"use client";

import { useMemo, useState } from "react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { SparklinePoint } from "@/lib/dashboard-home-types";
import {
  formatPulseChartAxisValue,
  formatPulseChartDate,
  formatPulseChartTooltipValue,
  type PulseMetricKey,
} from "@/lib/seller-pulse-view";
import { sparklineNumericValue } from "@/lib/sparkline-value";

const VIEW_WIDTH = 220;
const VIEW_HEIGHT = 72;
const Y_AXIS_WIDTH = 34;
const PADDING_TOP = 6;
const PADDING_BOTTOM = 6;
const PADDING_RIGHT = 8;

type ChartPoint = {
  index: number;
  x: number;
  y: number;
  value: number;
  date: string;
};

type Props = {
  points: SparklinePoint[];
  metricKey: PulseMetricKey;
  stroke: string;
  className?: string;
};

export function PulseKpiChart({ points, metricKey, stroke, className }: Props) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const layout = useMemo(() => {
    const numericValues = points
      .map((point) => sparklineNumericValue(point.value))
      .filter((value): value is number => value != null);
    if (numericValues.length < 2) {
      return null;
    }

    let min = Math.min(...numericValues);
    let max = Math.max(...numericValues);
    if (min === max) {
      min -= 1;
      max += 1;
    }

    const chartWidth = VIEW_WIDTH - Y_AXIS_WIDTH - PADDING_RIGHT;
    const chartHeight = VIEW_HEIGHT - PADDING_TOP - PADDING_BOTTOM;
    const range = max - min;
    const step = points.length > 1 ? chartWidth / (points.length - 1) : 0;

    const toY = (value: number) => PADDING_TOP + chartHeight - ((value - min) / range) * chartHeight;

    const chartPoints: ChartPoint[] = [];
    points.forEach((point, index) => {
      const value = sparklineNumericValue(point.value);
      if (value == null) return;
      chartPoints.push({
        index,
        x: Y_AXIS_WIDTH + index * step,
        y: toY(value),
        value,
        date: point.date,
      });
    });

    if (chartPoints.length < 2) {
      return null;
    }

    const segments: ChartPoint[][] = [];
    let current: ChartPoint[] = [];
    points.forEach((point, index) => {
      const value = sparklineNumericValue(point.value);
      if (value == null) {
        if (current.length > 0) {
          segments.push(current);
          current = [];
        }
        return;
      }
      const existing = chartPoints.find((chartPoint) => chartPoint.index === index);
      if (existing) {
        current.push(existing);
      }
    });
    if (current.length > 0) {
      segments.push(current);
    }

    const yTicks = [max, (max + min) / 2, min];

    return {
      min,
      max,
      chartHeight,
      chartWidth,
      yTicks,
      segments,
      chartPoints,
      toY,
    };
  }, [metricKey, points]);

  if (!layout) {
    return null;
  }

  const hovered = hoverIndex == null ? null : layout.chartPoints.find((point) => point.index === hoverIndex) ?? null;
  const chartRight = Y_AXIS_WIDTH + layout.chartWidth;

  return (
    <div className={cn("relative w-full", className)}>
      <svg
        viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
        className="h-[4.5rem] w-full"
        role="img"
        aria-label="Динамика показателя за 7 дней"
        onMouseLeave={() => setHoverIndex(null)}
      >
        {layout.yTicks.map((tick) => {
          const y = layout.toY(tick);
          return (
            <g key={tick}>
              <line
                x1={Y_AXIS_WIDTH}
                y1={y}
                x2={chartRight}
                y2={y}
                stroke="#eaecf0"
                strokeWidth="1"
                strokeDasharray="3 3"
              />
              <text
                x={Y_AXIS_WIDTH - 4}
                y={y + 3}
                textAnchor="end"
                className={dashboardTypography.chartAxis}
              >
                {formatPulseChartAxisValue(metricKey, tick)}
              </text>
            </g>
          );
        })}

        {layout.segments.map((segment, segmentIndex) => (
          <polyline
            key={segmentIndex}
            fill="none"
            stroke={stroke}
            strokeWidth="1.75"
            strokeLinejoin="round"
            strokeLinecap="round"
            points={segment.map((point) => `${point.x},${point.y}`).join(" ")}
          />
        ))}

        {hovered ? (
          <>
            <line
              x1={hovered.x}
              y1={PADDING_TOP}
              x2={hovered.x}
              y2={VIEW_HEIGHT - PADDING_BOTTOM}
              stroke="#d0d5dd"
              strokeWidth="1"
            />
            <circle cx={hovered.x} cy={hovered.y} r="3.25" fill={stroke} stroke="white" strokeWidth="1.5" />
          </>
        ) : null}

        {layout.chartPoints.map((point) => (
          <rect
            key={point.index}
            x={point.x - (layout.chartWidth / Math.max(points.length - 1, 1)) / 2}
            y={PADDING_TOP}
            width={layout.chartWidth / Math.max(points.length - 1, 1)}
            height={layout.chartHeight}
            fill="transparent"
            onMouseEnter={() => setHoverIndex(point.index)}
          />
        ))}
      </svg>

      {hovered ? (
        <div
          className={cn(
            "pointer-events-none absolute z-10 min-w-[7.5rem] rounded-lg border border-aivion-border bg-white px-2.5 py-1.5 shadow-sm",
            dashboardTypography.chartTooltip,
          )}
          style={{
            left: `${(hovered.x / VIEW_WIDTH) * 100}%`,
            top: 0,
            transform: "translate(-50%, -110%)",
          }}
        >
          <p className="font-medium text-slate-800">{formatPulseChartDate(hovered.date)}</p>
          <p className={cn(dashboardTypography.chartTooltipValue, "text-slate-600")}>
            {formatPulseChartTooltipValue(metricKey, hovered.value)}
          </p>
        </div>
      ) : null}
    </div>
  );
}
