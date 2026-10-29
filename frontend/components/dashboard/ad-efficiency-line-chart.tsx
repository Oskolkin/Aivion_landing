"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { cn } from "@/components/ui/cn";
import {
  buildAdEfficiencyChartSummary,
  formatAverageChartLabel,
  formatDrrPercent,
  type TrendPointDisplay,
} from "@/lib/ad-efficiency-display";
import {
  buildYTicks,
  CHART_LAYOUT,
  computeDRRChartYDomain,
  getAverageLabelPlacement,
  getPointLabelPlacement,
  shouldShowPointLabel,
  SVG_TEXT_HALO_STYLE,
} from "@/lib/ad-efficiency-chart-layout";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  points: TrendPointDisplay[];
  hasTrendData: boolean;
  averageDrrPct?: number | null;
  sampleDays?: number | null;
};

type HoveredPoint = {
  index: number;
  x: number;
  y: number;
  point: TrendPointDisplay;
};

const { width: CHART_WIDTH, height: CHART_HEIGHT, padding: PADDING } = CHART_LAYOUT;

function SvgHaloText({
  x,
  y,
  textAnchor,
  className,
  testId,
  children,
}: {
  x: number;
  y: number;
  textAnchor: "start" | "middle" | "end";
  className?: string;
  testId?: string;
  children: ReactNode;
}) {
  return (
    <text
      x={x}
      y={y}
      textAnchor={textAnchor}
      className={className}
      data-testid={testId}
      style={SVG_TEXT_HALO_STYLE}
    >
      {children}
    </text>
  );
}

export function AdEfficiencyLineChart({ points, hasTrendData, averageDrrPct, sampleDays }: Props) {
  const [hovered, setHovered] = useState<HoveredPoint | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(1280);

  useEffect(() => {
    const node = containerRef.current;
    if (!node || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (width && width > 0) setContainerWidth(width);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const chartSummary = useMemo(
    () => buildAdEfficiencyChartSummary(points, averageDrrPct ?? null),
    [points, averageDrrPct],
  );

  if (!hasTrendData || points.length === 0) {
    return (
      <div
        className={cn(
          "rounded-[20px] border border-slate-200 bg-white px-5 py-12 text-center",
          dashboardTypography.emptyState,
        )}
        data-testid="ad-efficiency-line-chart-empty"
      >
        Недостаточно данных для динамики ДРР в выручке с рекламных кампаний.
      </div>
    );
  }

  const plotWidth = CHART_WIDTH - PADDING.left - PADDING.right;
  const plotHeight = CHART_HEIGHT - PADDING.top - PADDING.bottom;
  const plotLeft = PADDING.left;
  const plotRight = CHART_WIDTH - PADDING.right;
  const plotTop = PADDING.top;
  const plotBottom = PADDING.top + plotHeight;

  const numericDrr = points
    .map((p) => p.drrValue)
    .filter((v): v is number => v != null && !Number.isNaN(v));

  const maxValue = numericDrr.length > 0 ? Math.max(...numericDrr) : 0;
  const maxValueIndex = points.findIndex((p) => p.drrValue === maxValue);

  const { yMin, yMax } = computeDRRChartYDomain(numericDrr, averageDrrPct, plotHeight);
  const ySpan = yMax - yMin;
  const valueToY = (value: number) =>
    plotTop + plotHeight - ((value - yMin) / ySpan) * plotHeight;

  const yTicks = buildYTicks(yMin, yMax, plotHeight);
  const xStep = points.length > 1 ? plotWidth / (points.length - 1) : 0;

  const coords = points.map((point, index) => {
    if (!point.hasDrr || point.drrValue == null) return null;
    const x = plotLeft + index * xStep;
    const y = valueToY(point.drrValue);
    return { x, y, point, index };
  });

  const lineSegments: string[] = [];
  let segment: string[] = [];
  for (const coord of coords) {
    if (!coord) {
      if (segment.length > 0) {
        lineSegments.push(segment.join(" "));
        segment = [];
      }
      continue;
    }
    segment.push(`${coord.x},${coord.y}`);
  }
  if (segment.length > 0) lineSegments.push(segment.join(" "));

  const baselineY = plotBottom;

  const areaSegments = lineSegments.map((segmentPoints, index) => {
    const segmentCoords = segmentPoints
      .split(" ")
      .map((pair) => pair.split(",").map(Number))
      .filter((pair): pair is [number, number] => pair.length === 2 && !Number.isNaN(pair[0]) && !Number.isNaN(pair[1]));
    if (segmentCoords.length < 2) return null;
    const first = segmentCoords[0];
    const last = segmentCoords[segmentCoords.length - 1];
    const path = `M ${first[0]} ${baselineY} L ${segmentCoords.map(([x, y]) => `${x} ${y}`).join(" L ")} L ${last[0]} ${baselineY} Z`;
    return { key: `area-${index}`, path };
  });

  const averageY =
    averageDrrPct != null && !Number.isNaN(averageDrrPct)
      ? valueToY(averageDrrPct)
      : null;

  const labelPlacements = coords.map((coord) => {
    if (!coord) return null;
    const show = shouldShowPointLabel({
      index: coord.index,
      pointsCount: points.length,
      width: containerWidth,
      isMaximum: coord.index === maxValueIndex,
      isCurrent: coord.point.isToday,
    });
    if (!show) return { ...coord, placement: null, show: false };
    const placement = getPointLabelPlacement({
      x: coord.x,
      y: coord.y,
      index: coord.index,
      pointsCount: points.length,
      averageY,
      chartTop: plotTop,
      chartRight: plotRight,
      chartLeft: plotLeft,
      plotBottom,
      isCurrent: coord.point.isToday,
    });
    return { ...coord, placement, show: true };
  });

  const lastVisible = labelPlacements
    .filter((item): item is NonNullable<typeof item> => item != null && item.show && item.placement != null)
    .at(-1);
  const lastCoord = coords[coords.length - 1];

  const averageLabelPlacement =
    averageY != null
      ? getAverageLabelPlacement({
          averageY,
          chartRight: plotRight,
          chartTop: plotTop,
          chartWidth: CHART_WIDTH,
          containerWidth,
          lastPointX: lastCoord?.x ?? null,
          lastPointLabelY: lastVisible?.placement?.labelY ?? null,
        })
      : null;

  const averageLabel = formatAverageChartLabel(sampleDays, averageDrrPct);

  return (
    <div
      className="rounded-[20px] border border-slate-200 bg-white p-5 shadow-[0_4px_18px_rgba(15,23,42,0.035)]"
      data-testid="ad-efficiency-line-chart"
    >
      <h3 className="mb-4 text-lg font-bold text-slate-950">
        Динамика ДРР в выручке с рекламных кампаний за 7 дней
      </h3>

      <div ref={containerRef} className="relative max-w-full overflow-x-auto overscroll-x-contain">
        <svg
          viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
          className="w-full min-w-[640px] max-w-none"
          role="img"
          aria-label={chartSummary}
        >
          <title>{chartSummary}</title>
          <defs>
            <linearGradient id="ad-efficiency-area-gradient" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor="#2563eb" stopOpacity={0.12} />
              <stop offset="100%" stopColor="#2563eb" stopOpacity={0} />
            </linearGradient>
          </defs>

          {yTicks.map((tick) => {
            const y = valueToY(tick);
            return (
              <g key={tick}>
                <line x1={plotLeft} x2={plotRight} y1={y} y2={y} stroke="#e2e8f0" strokeWidth={1} />
                <text
                  x={plotLeft - 12}
                  y={y + 4}
                  textAnchor="end"
                  className="fill-slate-500 text-[13px] font-medium"
                >
                  {formatDrrPercent(tick)}
                </text>
              </g>
            );
          })}

          {areaSegments.map((segment) =>
            segment ? (
              <path
                key={segment.key}
                d={segment.path}
                fill="url(#ad-efficiency-area-gradient)"
                stroke="none"
                data-testid="ad-efficiency-area-segment"
              />
            ) : null,
          )}

          {averageY != null ? (
            <g data-testid="ad-efficiency-average-line">
              <line
                x1={plotLeft}
                x2={plotRight}
                y1={averageY}
                y2={averageY}
                stroke="#2563eb"
                strokeWidth={1.5}
                strokeDasharray="6 5"
                opacity={0.72}
              />
              {averageLabel && averageLabelPlacement ? (
                <SvgHaloText
                  x={averageLabelPlacement.x}
                  y={averageLabelPlacement.y}
                  textAnchor={averageLabelPlacement.textAnchor}
                  className="fill-blue-600 text-[13px] font-semibold"
                  testId="ad-efficiency-average-label"
                >
                  {averageLabel}
                </SvgHaloText>
              ) : null}
            </g>
          ) : null}

          {lineSegments.map((segmentPoints, index) => (
            <polyline
              key={`segment-${index}`}
              fill="none"
              stroke="#2563eb"
              strokeWidth={2.5}
              strokeLinejoin="round"
              strokeLinecap="round"
              points={segmentPoints}
            />
          ))}

          {labelPlacements.map((item) => {
            if (!item) return null;
            const { x, y, point, placement, show } = item;
            const radius = point.isToday ? 5.5 : 4;
            return (
              <g
                key={`point-${point.date}`}
                data-testid="ad-efficiency-point-label-group"
                onMouseEnter={() => setHovered({ index: item.index, x, y, point })}
                onMouseLeave={() => setHovered(null)}
                onFocus={() => setHovered({ index: item.index, x, y, point })}
                onBlur={() => setHovered(null)}
                tabIndex={0}
                role="graphics-symbol"
                aria-label={`${point.label}: ${point.drrLabel ?? "нет данных"}`}
              >
                {point.drrLabel && show && placement ? (
                  <SvgHaloText
                    x={placement.labelX}
                    y={placement.labelY}
                    textAnchor={placement.textAnchor}
                    className={cn(
                      "font-semibold tabular-nums",
                      point.isToday ? "fill-blue-600" : "fill-slate-800",
                    )}
                    testId="ad-efficiency-value-label"
                  >
                    <tspan fontSize={placement.fontSize}>{point.drrLabel}</tspan>
                  </SvgHaloText>
                ) : null}
                <circle
                  cx={x}
                  cy={y}
                  r={radius}
                  fill={point.isToday ? "#2563eb" : "#ffffff"}
                  stroke="#2563eb"
                  strokeWidth={point.isToday ? 2 : 1.5}
                  data-testid={point.isToday ? "ad-efficiency-today-marker" : undefined}
                />
              </g>
            );
          })}

          {points.map((point, index) => {
            const x = plotLeft + index * xStep;
            const dateLabel = point.label
              .replace(/\s*\(сегодня\)\s*$/i, "")
              .replace(/\s*\(вчера\)\s*$/i, "")
              .trim();
            return (
              <text
                key={`label-${point.date}`}
                x={x}
                y={CHART_HEIGHT - 14}
                textAnchor="middle"
                className={cn(
                  "text-[13px]",
                  point.isToday ? "fill-blue-600 font-semibold" : "fill-slate-600",
                )}
              >
                {dateLabel}
              </text>
            );
          })}
        </svg>

        {hovered ? (
          <div
            className="pointer-events-none absolute z-10 min-w-[168px] rounded-xl border border-slate-200 bg-white px-3 py-2.5 shadow-lg"
            style={{
              left: `${(hovered.x / CHART_WIDTH) * 100}%`,
              top: `${(hovered.y / CHART_HEIGHT) * 100}%`,
              transform: "translate(-50%, calc(-100% - 12px))",
            }}
            data-testid="ad-efficiency-chart-tooltip"
          >
            <p className="text-[13px] font-semibold text-slate-800">
              {hovered.point.label
                .replace(/\s*\(сегодня\)\s*$/i, "")
                .replace(/\s*\(вчера\)\s*$/i, "")
                .trim()}
            </p>
            <p className="mt-1 text-[13px] text-slate-700">
              Доля: <span className="font-semibold tabular-nums">{hovered.point.drrLabel ?? "—"}</span>
            </p>
            {hovered.point.deltaLabel ? (
              <p className="text-[13px] text-slate-700" data-testid="ad-efficiency-tooltip-delta">
                Изменение: <span className="font-semibold tabular-nums">{hovered.point.deltaLabel}</span>
              </p>
            ) : null}
            {averageLabel ? (
              <p className="text-[13px] text-slate-700">
                <span className="font-semibold tabular-nums">{averageLabel}</span>
              </p>
            ) : null}
          </div>
        ) : null}
      </div>

      <p className="sr-only" data-testid="ad-efficiency-chart-summary">
        {chartSummary}
      </p>
    </div>
  );
}
