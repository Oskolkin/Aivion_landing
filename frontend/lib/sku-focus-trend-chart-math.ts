import type { SkuFocusTrendPoint, SkuFocusTrendVariant } from "@/lib/sku-focus-display";

export type SkuFocusTrendChartCoord = {
  index: number;
  x: number;
  y: number;
  point: SkuFocusTrendPoint;
  hasValue: boolean;
};

export type SkuFocusTrendChartSegment = {
  points: string;
};

export type SkuFocusTrendYAxisTick = {
  value: number;
  label: string;
  y: number;
};

export type SkuFocusTrendChartModel = {
  coords: SkuFocusTrendChartCoord[];
  segments: SkuFocusTrendChartSegment[];
  yAxisTicks: SkuFocusTrendYAxisTick[];
  plotLeft: number;
  plotRight: number;
  baselineY: number;
  canRender: boolean;
};

type BuildSkuFocusTrendChartModelArgs = {
  points: SkuFocusTrendPoint[];
  variant?: SkuFocusTrendVariant;
  width?: number;
  height?: number;
  paddingX?: number;
  paddingY?: number;
  yAxisWidth?: number;
};

function trendPointValue(point: SkuFocusTrendPoint, variant: SkuFocusTrendVariant): number | null {
  if (variant === "stock") return point.stock;
  if (variant === "revenue") return point.revenue;
  return point.orders;
}

function trendPointHasValue(point: SkuFocusTrendPoint, variant: SkuFocusTrendVariant): boolean {
  return trendPointValue(point, variant) != null;
}

export function formatSkuFocusYAxisValue(value: number, variant: SkuFocusTrendVariant): string {
  if (variant === "revenue") {
    if (Math.abs(value) >= 1000) {
      const thousands = value / 1000;
      const formatted = new Intl.NumberFormat("ru-RU", {
        maximumFractionDigits: thousands >= 10 || Number.isInteger(thousands) ? 0 : 1,
      }).format(thousands);
      return `${formatted}k`;
    }
    return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(value);
  }
  return new Intl.NumberFormat("ru-RU", {
    maximumFractionDigits: Number.isInteger(value) ? 0 : 1,
  }).format(value);
}

/** Build 2–3 unique readable ticks for the chart's data range. */
export function buildSkuFocusYAxisTickValues(min: number, max: number): number[] {
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return [0];
  }
  let lo = Math.min(min, max);
  const hi = Math.max(min, max);
  if (lo === hi) {
    if (lo === 0) {
      return [0, 1];
    }
    if (lo > 0) {
      return [0, lo];
    }
    return [lo, 0];
  }
  if (lo > 0 && lo / hi > 0.4) {
    lo = 0;
  }
  const range = hi - lo;
  const mid = lo + range / 2;
  const raw = [hi, mid, lo];
  const rounded = raw.map((v) => niceTick(v, range));
  const unique: number[] = [];
  for (const v of rounded) {
    if (unique.some((u) => Math.abs(u - v) < range * 0.08 + 1e-9)) continue;
    unique.push(v);
  }
  if (unique.length < 2) {
    return lo === 0 ? [hi, 0] : [hi, lo];
  }
  return unique.slice(0, 3).sort((a, b) => b - a);
}

function niceTick(value: number, range: number): number {
  if (!Number.isFinite(value)) return 0;
  if (range <= 0) return value;
  const step = niceStep(range / 2);
  if (step <= 0) return value;
  return Math.round(value / step) * step;
}

function niceStep(raw: number): number {
  if (!Number.isFinite(raw) || raw <= 0) return 1;
  const exp = Math.floor(Math.log10(raw));
  const base = Math.pow(10, exp);
  const norm = raw / base;
  if (norm <= 1) return base;
  if (norm <= 2) return 2 * base;
  if (norm <= 5) return 5 * base;
  return 10 * base;
}

export function buildSkuFocusTrendChartModel({
  points,
  variant = "orders",
  width = 320,
  height = 64,
  paddingX = 4,
  paddingY = 8,
  yAxisWidth = 28,
}: BuildSkuFocusTrendChartModelArgs): SkuFocusTrendChartModel {
  const plotLeft = yAxisWidth;
  const plotRight = width - paddingX;
  const innerW = Math.max(1, plotRight - plotLeft);
  const innerH = height - paddingY * 2;
  const baselineY = paddingY + innerH;

  if (points.length === 0) {
    return {
      coords: [],
      segments: [],
      yAxisTicks: [],
      plotLeft,
      plotRight,
      baselineY,
      canRender: false,
    };
  }

  const definedValues = points
    .map((point) => trendPointValue(point, variant))
    .filter((value): value is number => value != null);

  if (definedValues.length < 2) {
    return {
      coords: [],
      segments: [],
      yAxisTicks: [],
      plotLeft,
      plotRight,
      baselineY,
      canRender: false,
    };
  }

  const dataMin = Math.min(...definedValues);
  const dataMax = Math.max(...definedValues);
  const tickValues = buildSkuFocusYAxisTickValues(dataMin, dataMax);
  const scaleMin = Math.min(dataMin, ...tickValues);
  const scaleMax = Math.max(dataMax, ...tickValues);
  const range = scaleMax - scaleMin;
  const flatLine = range === 0;

  const yForValue = (value: number) => {
    if (flatLine) {
      return paddingY + innerH / 2;
    }
    return paddingY + innerH - ((value - scaleMin) / range) * innerH;
  };

  const xForIndex = (index: number) => {
    if (points.length === 1) {
      return plotLeft + innerW / 2;
    }
    return plotLeft + (index / (points.length - 1)) * innerW;
  };

  const coords: SkuFocusTrendChartCoord[] = points.map((point, index) => {
    const hasValue = trendPointHasValue(point, variant);
    const value = trendPointValue(point, variant);
    const y = hasValue && value != null ? yForValue(value) : baselineY;
    return {
      index,
      x: xForIndex(index),
      y,
      point,
      hasValue,
    };
  });

  const segments: SkuFocusTrendChartSegment[] = [];
  let current: string[] = [];

  const flush = () => {
    if (current.length >= 2) {
      segments.push({ points: current.join(" ") });
    }
    current = [];
  };

  coords.forEach((coord) => {
    if (!coord.hasValue) {
      flush();
      return;
    }
    current.push(`${coord.x},${coord.y}`);
  });
  flush();

  const yAxisTicks: SkuFocusTrendYAxisTick[] = tickValues.map((value) => ({
    value,
    label: formatSkuFocusYAxisValue(value, variant),
    y: yForValue(value),
  }));

  return {
    coords,
    segments,
    yAxisTicks,
    plotLeft,
    plotRight,
    baselineY,
    canRender: coords.some((coord) => coord.hasValue),
  };
}

export function formatSkuFocusTrendTooltipRevenue(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) {
    return "нет данных";
  }
  return `${new Intl.NumberFormat("ru-RU", {
    maximumFractionDigits: 0,
  }).format(value)} ₽`;
}

export function formatSkuFocusTrendTooltipCoverage(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) {
    return "нет данных";
  }
  return `${new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: value < 10 ? 1 : 0,
    maximumFractionDigits: 1,
  }).format(value)} дн.`;
}

export function formatSkuFocusTrendTooltipStock(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) {
    return "нет данных";
  }
  return `${value} шт.`;
}
