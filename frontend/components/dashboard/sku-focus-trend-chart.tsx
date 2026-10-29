"use client";

import { useMemo, useState } from "react";
import { cn } from "@/components/ui/cn";
import {
  buildSkuFocusTrendTooltipText,
  formatSkuFocusTrendDate,
  type SkuFocusTrendPoint,
  type SkuFocusTrendVariant,
} from "@/lib/sku-focus-display";
import {
  buildSkuFocusTrendChartModel,
  formatSkuFocusTrendTooltipCoverage,
  formatSkuFocusTrendTooltipRevenue,
  formatSkuFocusTrendTooltipStock,
} from "@/lib/sku-focus-trend-chart-math";

type SkuFocusTrendChartProps = {
  points: SkuFocusTrendPoint[];
  variant?: SkuFocusTrendVariant;
  width?: number;
  height?: number;
  stroke?: string;
  fill?: string;
  className?: string;
};

export function SkuFocusTrendChart({
  points,
  variant = "orders",
  width = 320,
  height = 72,
  stroke = "#EA580C",
  fill = "rgba(251, 146, 60, 0.12)",
  className,
}: SkuFocusTrendChartProps) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);

  const model = useMemo(
    () => buildSkuFocusTrendChartModel({ points, variant, width, height }),
    [height, points, variant, width],
  );

  if (!model.canRender || model.coords.length < 2) {
    return <svg width={width} height={height} className={className} aria-hidden />;
  }

  const activeCoord =
    activeIndex == null ? null : model.coords.find((coord) => coord.index === activeIndex);

  return (
    <div className={cn("relative h-full w-full", className)} data-testid="sku-focus-trend-chart">
      <svg
        width="100%"
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        className="h-full w-full"
        data-testid="sku-focus-trend-chart-svg"
      >
        {model.yAxisTicks.map((tick) => (
          <g key={`y-${tick.value}`} data-testid="sku-focus-trend-y-tick">
            <line
              x1={model.plotLeft}
              y1={tick.y}
              x2={model.plotRight}
              y2={tick.y}
              stroke="#E2E8F0"
              strokeWidth={1}
              strokeDasharray="2 3"
            />
            <text
              x={model.plotLeft - 4}
              y={tick.y}
              textAnchor="end"
              dominantBaseline="middle"
              className="fill-slate-400"
              style={{ fontSize: 9 }}
              data-testid="sku-focus-trend-y-label"
            >
              {tick.label}
            </text>
          </g>
        ))}
        {model.segments.map((segment, index) => {
          const segmentCoords = segment.points
            .split(" ")
            .map((pair) => pair.split(",").map(Number))
            .filter((pair) => pair.length === 2);
          if (segmentCoords.length < 2) {
            return null;
          }
          const areaPoints = [
            `${segmentCoords[0][0]},${model.baselineY}`,
            ...segmentCoords.map((coord) => `${coord[0]},${coord[1]}`),
            `${segmentCoords[segmentCoords.length - 1][0]},${model.baselineY}`,
          ].join(" ");
          return (
            <g key={index}>
              <polygon points={areaPoints} fill={fill} />
              <polyline
                fill="none"
                stroke={stroke}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
                points={segment.points}
              />
            </g>
          );
        })}
        {model.coords.map((coord) => (
          <g key={coord.index}>
            <circle
              cx={coord.x}
              cy={coord.y}
              r={12}
              fill="transparent"
              className="cursor-pointer outline-none"
              tabIndex={0}
              onMouseEnter={() => setActiveIndex(coord.index)}
              onMouseLeave={() => setActiveIndex(null)}
              onFocus={() => setActiveIndex(coord.index)}
              onBlur={() => setActiveIndex(null)}
              data-testid="sku-focus-trend-point"
            />
            <circle
              cx={coord.x}
              cy={coord.y}
              r={activeIndex === coord.index ? 4.5 : 3}
              fill={activeIndex === coord.index ? stroke : "#FFFFFF"}
              stroke={stroke}
              strokeWidth={activeIndex === coord.index ? 2.5 : 2}
              pointerEvents="none"
              className={cn(
                "transition-[r,stroke-width] duration-150",
                activeIndex === coord.index ? "drop-shadow-sm" : "",
              )}
            />
          </g>
        ))}
      </svg>
      {activeCoord ? (
        <div
          className="pointer-events-none absolute left-1/2 top-0 z-10 min-w-[148px] -translate-x-1/2 -translate-y-[calc(100%+8px)] rounded-lg border border-[#E2E8F0] bg-white px-2.5 py-2 text-[11px] leading-snug text-[#334155] shadow-[0_8px_20px_rgba(15,23,42,0.12)]"
          data-testid="sku-focus-trend-tooltip"
        >
          <p className="font-semibold text-[#1E293B]">{formatSkuFocusTrendDate(activeCoord.point.date)}</p>
          {variant === "stock" ? (
            <>
              <p className="mt-0.5">
                Остаток: {formatSkuFocusTrendTooltipStock(activeCoord.point.stock)}
              </p>
              <p>Покрытие: {formatSkuFocusTrendTooltipCoverage(activeCoord.point.coverageDays)}</p>
            </>
          ) : variant === "revenue" ? (
            <>
              <p className="mt-0.5">
                Выручка: {formatSkuFocusTrendTooltipRevenue(activeCoord.point.revenue)}
              </p>
              <p>
                Заказы:{" "}
                {activeCoord.point.orders == null ? "нет данных" : activeCoord.point.orders}
              </p>
            </>
          ) : (
            <>
              <p className="mt-0.5">
                Заказы: {activeCoord.point.orders == null ? "нет данных" : activeCoord.point.orders}
              </p>
              <p>Выручка: {formatSkuFocusTrendTooltipRevenue(activeCoord.point.revenue)}</p>
            </>
          )}
          <span className="sr-only">
            {buildSkuFocusTrendTooltipText(activeCoord.point, variant)}
          </span>
        </div>
      ) : null}
    </div>
  );
}
