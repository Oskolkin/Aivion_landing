import {
  formatDrrPercent,
} from "@/lib/ad-efficiency-display";

export const CHART_LAYOUT = {
  width: 760,
  height: 252,
  // Right padding is an annotation gutter outside the plot area for "За 7 дней".
  padding: { top: 36, right: 128, bottom: 44, left: 58 },
} as const;

/** Horizontal gap from plot right edge to the weekly annotation. */
export const AVERAGE_LABEL_GUTTER_PX = 12;

export const NEAR_AVERAGE_THRESHOLD_PX = 20;
export const LABEL_ABOVE_OFFSET_PX = 16;
export const LABEL_BELOW_OFFSET_PX = 22;

/** Minimum vertical gap between Y-axis tick labels (pixels in chart viewBox space). */
export const Y_TICK_MIN_GAP_PX = 28;

const NICE_MULTIPLIERS = [1, 2, 2.5, 5, 10] as const;
const MIN_DOMAIN_SPAN_PP = 8;
const MIN_PADDING_PP = 2;
const FLAT_SERIES_PADDING_PP = 5;

export type DRRChartYDomain = {
  yMin: number;
  yMax: number;
};

export type TextAnchor = "start" | "middle" | "end";

export type PointLabelPlacement = {
  labelX: number;
  labelY: number;
  textAnchor: TextAnchor;
  placeBelow: boolean;
  fontSize: number;
  isCurrent: boolean;
};

export type AverageLabelPlacement = {
  x: number;
  y: number;
  textAnchor: "start" | "end";
  /** True when label is parked in the top-right legend zone (narrow layouts). */
  legendZone: boolean;
};

export type ShouldShowPointLabelInput = {
  index: number;
  pointsCount: number;
  width: number;
  isMaximum: boolean;
  isCurrent: boolean;
};

export function plotHeightPx(
  chartHeight = CHART_LAYOUT.height,
  paddingTop = CHART_LAYOUT.padding.top,
  paddingBottom = CHART_LAYOUT.padding.bottom,
): number {
  return chartHeight - paddingTop - paddingBottom;
}

function roundDown(value: number, step: number): number {
  return Math.floor(value / step) * step;
}

function roundUp(value: number, step: number): number {
  return Math.ceil(value / step) * step;
}

/** Pick a human-readable tick step for a given span and target tick count. */
export function chooseNiceStep(span: number, maxTicks: number): number {
  if (!(span > 0)) return 1;
  const safeMaxTicks = Math.max(2, maxTicks);
  const rough = span / (safeMaxTicks - 1);
  const magnitude = Math.pow(10, Math.floor(Math.log10(Math.max(rough, 1e-6))));
  for (const mult of NICE_MULTIPLIERS) {
    const step = mult * magnitude;
    if (span / step <= safeMaxTicks - 1) return step;
  }
  return 10 * magnitude;
}

/**
 * Adaptive Y-domain for account DRR series. Never hardcodes 0–100%.
 */
export function computeDRRChartYDomain(
  values: number[],
  average?: number | null,
  plotHeight: number = plotHeightPx(),
): DRRChartYDomain {
  const candidates = values.filter((v) => !Number.isNaN(v));
  if (average != null && !Number.isNaN(average)) candidates.push(average);

  if (candidates.length === 0) {
    return { yMin: 0, yMax: 25 };
  }

  const minValue = Math.min(...candidates);
  const maxValue = Math.max(...candidates);
  const spread = maxValue - minValue;

  if (spread === 0) {
    const yMin = Math.max(0, minValue - FLAT_SERIES_PADDING_PP);
    const yMax = maxValue + FLAT_SERIES_PADDING_PP;
    return normalizeDomain(yMin, yMax, plotHeight);
  }

  const padding = Math.max(MIN_PADDING_PP, spread * 0.2);
  let rawMin = Math.max(0, minValue - padding);
  let rawMax = maxValue + padding;

  if (rawMax - rawMin < MIN_DOMAIN_SPAN_PP) {
    const center = (rawMin + rawMax) / 2;
    rawMin = Math.max(0, center - MIN_DOMAIN_SPAN_PP / 2);
    rawMax = rawMin + MIN_DOMAIN_SPAN_PP;
  }

  return normalizeDomain(rawMin, rawMax, plotHeight);
}

function normalizeDomain(rawMin: number, rawMax: number, plotHeight: number): DRRChartYDomain {
  const span = rawMax - rawMin;
  const maxTicks = Math.max(4, Math.min(6, Math.floor(plotHeight / Y_TICK_MIN_GAP_PX) + 1));
  const step = chooseNiceStep(span, maxTicks);
  const yMin = roundDown(rawMin, step);
  let yMax = roundUp(rawMax, step);
  if (yMax - yMin < MIN_DOMAIN_SPAN_PP) {
    yMax = yMin + Math.max(step, MIN_DOMAIN_SPAN_PP);
  }
  return { yMin, yMax };
}

/** @deprecated use computeDRRChartYDomain().yMax — kept for transitional callers. */
export function getDRRChartDomain(
  values: number[],
  average?: number | null,
  plotHeight: number = plotHeightPx(),
): number {
  return computeDRRChartYDomain(values, average, plotHeight).yMax;
}

/**
 * Evenly spaced Y ticks between yMin and yMax using a nice step.
 */
export function buildYTicks(
  yMin: number,
  yMax: number,
  plotHeight: number = plotHeightPx(),
  minGapPx = Y_TICK_MIN_GAP_PX,
): number[] {
  const span = yMax - yMin;
  if (!(span > 0)) return [yMin];
  const maxTicks = Math.max(4, Math.min(6, Math.floor(plotHeight / minGapPx) + 1));
  const step = chooseNiceStep(span, maxTicks);
  const ticks: number[] = [];
  for (let value = yMin; value <= yMax + 1e-9; value += step) {
    ticks.push(Math.round(value * 100) / 100);
  }
  if (ticks.length === 0 || Math.abs(ticks[ticks.length - 1] - yMax) > 1e-6) {
    const last = ticks[ticks.length - 1] ?? yMin;
    const gapPx = ((yMax - last) / span) * plotHeight;
    if (gapPx >= minGapPx) ticks.push(yMax);
  }
  return dedupeYTicksByRoundedLabel(ticks, yMin, yMax, plotHeight, minGapPx);
}

/** Drop ticks whose rounded Russian percent labels would collide or be identical. */
export function dedupeYTicksByRoundedLabel(
  ticks: number[],
  yMin: number,
  yMax: number,
  plotHeight: number,
  minGapPx = Y_TICK_MIN_GAP_PX,
): number[] {
  if (ticks.length <= 1) return ticks;
  const span = yMax - yMin;
  const out: number[] = [ticks[0]];
  for (let i = 1; i < ticks.length; i++) {
    const prev = out[out.length - 1];
    const cur = ticks[i];
    const gapPx = span > 0 ? (Math.abs(cur - prev) / span) * plotHeight : minGapPx;
    const sameLabel = formatDrrPercent(prev) === formatDrrPercent(cur);
    if (sameLabel || gapPx < minGapPx) {
      if (Math.abs(cur - yMax) < Math.abs(prev - yMax) && out.length > 1) {
        out[out.length - 1] = cur;
      }
      continue;
    }
    out.push(cur);
  }
  return out;
}

export function shouldShowPointLabel(input: ShouldShowPointLabelInput): boolean {
  const { index, pointsCount, width, isMaximum, isCurrent } = input;
  if (pointsCount <= 0) return false;
  if (isCurrent || isMaximum || index === 0) return true;
  if (width >= 1024) return true;
  if (width >= 640) return index % 2 === 0 || index === pointsCount - 2;
  return false;
}

function clampY(y: number, chartTop: number, plotBottom: number): number {
  return Math.max(chartTop + 10, Math.min(plotBottom - 6, y));
}

export function getPointLabelPlacement(input: {
  x: number;
  y: number;
  index: number;
  pointsCount: number;
  averageY: number | null;
  chartTop: number;
  chartRight: number;
  chartLeft: number;
  plotBottom: number;
  isCurrent: boolean;
}): PointLabelPlacement {
  const { x, y, index, pointsCount, averageY, chartTop, chartRight, chartLeft, plotBottom, isCurrent } = input;

  const isFirst = index === 0;
  const isLast = index === pointsCount - 1;
  const isPenultimate = index === pointsCount - 2;
  const plotWidth = chartRight - chartLeft;
  const inRightThird = plotWidth > 0 && x > chartLeft + plotWidth * 0.66;

  const nearAverage = averageY != null && Math.abs(y - averageY) < NEAR_AVERAGE_THRESHOLD_PX;

  let placeBelow = nearAverage;
  let labelX = x;
  let textAnchor: TextAnchor = "middle";

  if (isFirst) {
    labelX = x + 14;
    textAnchor = "start";
    placeBelow = nearAverage;
  } else if (isLast) {
    // Last value: centered above the last point (10–16px).
    labelX = x;
    textAnchor = "middle";
    placeBelow = false;
  } else if (isCurrent) {
    const spaceRight = chartRight - x;
    if (nearAverage) {
      placeBelow = true;
      labelX = x + 10;
      textAnchor = "start";
    } else if (spaceRight < 58) {
      labelX = x - 12;
      textAnchor = "end";
      placeBelow = false;
    } else {
      labelX = x + 12;
      textAnchor = "start";
      placeBelow = false;
    }
  } else if (isPenultimate && inRightThird && nearAverage) {
    placeBelow = true;
    labelX = x - 8;
    textAnchor = "end";
  } else if (nearAverage) {
    placeBelow = true;
    labelX = x;
    textAnchor = "middle";
  }

  const rawLabelY = placeBelow ? y + LABEL_BELOW_OFFSET_PX : y - LABEL_ABOVE_OFFSET_PX;
  const labelY = clampY(rawLabelY, chartTop, plotBottom);

  return {
    labelX,
    labelY,
    textAnchor,
    placeBelow,
    fontSize: isCurrent ? 18 : 14,
    isCurrent,
  };
}

export function getAverageLabelPlacement(input: {
  averageY: number;
  chartRight: number;
  chartTop: number;
  chartWidth?: number;
  containerWidth?: number;
  pointLabelYs?: number[];
  lastPointX?: number | null;
  lastPointLabelY?: number | null;
}): AverageLabelPlacement {
  const {
    averageY,
    chartRight,
    chartTop,
    chartWidth = CHART_LAYOUT.width,
    containerWidth,
    lastPointX = null,
    lastPointLabelY = null,
  } = input;

  const narrow =
    (containerWidth != null && containerWidth < 720) ||
    chartWidth - chartRight < AVERAGE_LABEL_GUTTER_PX + 72;

  if (narrow) {
    return {
      x: chartWidth - 8,
      y: chartTop + 4,
      textAnchor: "end",
      legendZone: true,
    };
  }

  // Strictly outside the plot area, to the right of the dashed-line endpoint.
  let x = chartRight + AVERAGE_LABEL_GUTTER_PX;
  if (lastPointX != null && x - lastPointX < 28) {
    x = lastPointX + 28;
  }
  // Vertically align with the dashed line; nudge only if a point label shares the band.
  let y = averageY + 4;
  if (lastPointLabelY != null && Math.abs(lastPointLabelY - y) < 16) {
    y = averageY + 18;
  }
  return {
    x,
    y: Math.max(chartTop + 8, y),
    textAnchor: "start",
    legendZone: false,
  };
}

export const SVG_TEXT_HALO_STYLE = {
  paintOrder: "stroke" as const,
  stroke: "white",
  strokeWidth: 4,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};
