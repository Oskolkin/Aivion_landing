"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { OverviewChartTooltip } from "@/components/dashboard/overview/overview-chart-tooltip";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewStockPreviewView } from "@/lib/overview-view-model";

type Props = {
  stock: OverviewStockPreviewView;
};

const TONE_BAR: Record<OverviewStockPreviewView["buckets"][number]["tone"], string> = {
  critical: "#f04438",
  warning: "#f79009",
  success: "#12b76a",
  neutral: "#0873f6",
  muted: "#98a2b3",
};

/** Semantic preview palette matching target mock (key-based nuance for 0–3). */
const KEY_BAR: Record<string, string> = {
  out_of_stock: "#f04438",
  critical_0_3: "#f97066",
  shortage_4_7: "#f79009",
  planning_8_14: "#0873f6",
  surplus: "#98a2b3",
};

const PLOT_W = 280;
const PLOT_H = 132;
const Y_AXIS_W = 28;
const PAD_TOP = 16;
const PAD_RIGHT = 8;

function niceYMax(max: number): number {
  if (max <= 0) return 5;
  if (max <= 5) return 5;
  if (max <= 10) return 10;
  if (max <= 15) return 15;
  if (max <= 20) return 20;
  const step = max <= 50 ? 10 : 25;
  return Math.ceil(max / step) * step;
}

function yTickValues(yMax: number): number[] {
  if (yMax <= 5) return [5, 0];
  if (yMax <= 10) return [10, 5, 0];
  if (yMax <= 15) return [15, 10, 5, 0];
  if (yMax <= 20) return [20, 10, 0];
  return [yMax, yMax / 2, 0];
}

export type StockBarGeometry = {
  x: number;
  y: number;
  width: number;
  height: number;
  centerX: number;
};

export function getStockBarGeometry(
  index: number,
  count: number,
  barCount: number,
  yMax: number,
): StockBarGeometry {
  const slot = PLOT_W / Math.max(barCount, 1);
  const width = Math.min(34, Math.max(22, slot * 0.42));
  const height = (count / yMax) * PLOT_H;
  const x = Y_AXIS_W + slot * index + (slot - width) / 2;
  const y = PAD_TOP + PLOT_H - height;
  return { x, y, width, height, centerX: x + width / 2 };
}

function wrapBucketLabel(label: string): string[] {
  if (label === "Нет остатка") return ["Нет", "остатка"];
  if (label === "0–3 дня" || label === "0-3 дня") return ["0–3", "дня"];
  if (label === "4–7 дней" || label === "4-7 дней") return ["4–7", "дней"];
  if (label === "8–14 дней" || label === "8-14 дней") return ["8–14", "дней"];
  if (label.length <= 9) return [label];
  const parts = label.split(/\s+/);
  if (parts.length >= 2) return [parts[0], parts.slice(1).join(" ")];
  return [label];
}

export function OverviewStockPreview({ stock }: Props) {
  const [activeKey, setActiveKey] = useState<string | null>(null);

  const layout = useMemo(() => {
    const counts = stock.buckets.map((b) => b.count);
    const rawMax = Math.max(0, ...counts);
    const yMax = niceYMax(rawMax);
    const yTicks = yTickValues(yMax);
    const barCount = Math.max(stock.buckets.length, 1);
    return { yMax, yTicks, barCount };
  }, [stock.buckets]);

  const active = stock.buckets.find((b) => b.key === activeKey) ?? null;
  const svgW = Y_AXIS_W + PLOT_W + PAD_RIGHT;
  const svgH = PAD_TOP + PLOT_H;
  const activeIndex = active ? stock.buckets.findIndex((bucket) => bucket.key === active.key) : -1;
  const activeGeometry =
    active && activeIndex >= 0
      ? getStockBarGeometry(activeIndex, active.count, layout.barCount, layout.yMax)
      : null;

  return (
    <section
      className="flex h-full flex-col rounded-[14px] border border-aivion-border bg-white p-4"
      data-testid="overview-stock-preview"
      aria-labelledby="overview-stock-preview-title"
    >
      <h2 id="overview-stock-preview-title" className={dashboardTypography.cardTitleCompact}>
        Остатки
      </h2>

      {stock.limited ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{stock.limitedMessage}</p>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <p className="mt-2.5 text-2xl font-bold tabular-nums text-aivion-text">{stock.summary}</p>

          <div
            className="mt-3 flex min-h-0 flex-1 flex-col"
            data-testid="overview-stock-bar-chart"
          >
            <div className="relative min-h-0 flex-1">
              <svg
                viewBox={`0 0 ${svgW} ${svgH}`}
                className="h-full w-full"
                role="img"
                aria-label="Распределение SKU по покрытию остатками"
                preserveAspectRatio="xMidYMid meet"
                onMouseLeave={() => setActiveKey(null)}
              >
                {layout.yTicks.map((tick, index) => {
                  const y = PAD_TOP + PLOT_H - (tick / layout.yMax) * PLOT_H;
                  return (
                    <g key={`yt-${index}`}>
                      <line
                        x1={Y_AXIS_W}
                        y1={y}
                        x2={Y_AXIS_W + PLOT_W}
                        y2={y}
                        stroke="#eef2f6"
                        strokeWidth="1"
                      />
                      <text
                        x={Y_AXIS_W - 4}
                        y={y + 3}
                        textAnchor="end"
                        className={dashboardTypography.chartAxis}
                      >
                        {Math.round(tick)}
                      </text>
                    </g>
                  );
                })}

                {stock.buckets.map((bucket, index) => {
                  const geometry = getStockBarGeometry(
                    index,
                    bucket.count,
                    layout.barCount,
                    layout.yMax,
                  );
                  const fill = KEY_BAR[bucket.key] ?? TONE_BAR[bucket.tone];
                  const hitHeight = Math.max(geometry.height, 12);
                  return (
                    <g key={bucket.key}>
                      <rect
                        x={geometry.x}
                        y={geometry.y}
                        width={geometry.width}
                        height={Math.max(geometry.height, bucket.count > 0 ? 3 : 0)}
                        rx={3}
                        fill={fill}
                        pointerEvents="none"
                      />
                      <rect
                        x={geometry.x}
                        y={PAD_TOP + PLOT_H - hitHeight}
                        width={geometry.width}
                        height={hitHeight}
                        fill="transparent"
                        tabIndex={0}
                        role="button"
                        aria-label={`${bucket.label}: ${bucket.count} SKU`}
                        data-testid={`overview-stock-bar-${bucket.key}`}
                        data-tooltip-x={geometry.centerX}
                        data-tooltip-y={geometry.y}
                        onMouseEnter={() => setActiveKey(bucket.key)}
                        onFocus={() => setActiveKey(bucket.key)}
                        onBlur={() => setActiveKey(null)}
                        onClick={() => setActiveKey(bucket.key)}
                      />
                      <text
                        x={geometry.centerX}
                        y={geometry.y - 5}
                        textAnchor="middle"
                        className="fill-aivion-text text-[11px] font-medium tabular-nums"
                      >
                        {bucket.count}
                      </text>
                    </g>
                  );
                })}
              </svg>

              {active && activeGeometry ? (
                <OverviewChartTooltip
                  dateLabel={active.label}
                  valueLabel={`${active.count} SKU`}
                  className="absolute"
                  style={{
                    left: `clamp(5.5rem, ${(activeGeometry.centerX / svgW) * 100}%, calc(100% - 5.5rem))`,
                    top: `calc(${(activeGeometry.y / svgH) * 100}% - 10px)`,
                    transform: "translate(-50%, -100%)",
                  }}
                />
              ) : null}
            </div>

            <div
              className="mt-1.5 grid gap-0"
              style={{
                paddingLeft: `${(Y_AXIS_W / svgW) * 100}%`,
                paddingRight: `${(PAD_RIGHT / svgW) * 100}%`,
                gridTemplateColumns: `repeat(${Math.max(stock.buckets.length, 1)}, minmax(0, 1fr))`,
              }}
              data-testid="overview-stock-x-labels"
            >
              {stock.buckets.map((bucket) => (
                <div
                  key={bucket.key}
                  className="px-0.5 text-center text-[11px] leading-[1.25] text-aivion-muted"
                >
                  {wrapBucketLabel(bucket.label).map((line) => (
                    <span key={`${bucket.key}-${line}`} className="block">
                      {line}
                    </span>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="mt-auto pt-3">
        <Link
          href={stock.href}
          className={cn(dashboardTypography.ctaLink, "inline-flex items-center gap-1")}
          data-testid="overview-open-stock"
        >
          Открыть раздел
          <span aria-hidden>→</span>
        </Link>
      </div>
    </section>
  );
}
