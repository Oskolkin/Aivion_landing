import { classifyComparisonBucket, type FunnelTone } from "@/lib/funnel-ai-analysis-view";

export type ComparisonBarItem = {
  key: string;
  label: string;
  titleLabel?: string;
  delta?: string;
  value?: string;
  tone: FunnelTone;
};

/** Parses signed percent or percentage-point delta for bar width. */
export function parseComparisonDeltaMagnitude(delta?: string): number {
  const raw = (delta ?? "").trim().replace(",", ".");
  if (!raw) return 0;
  const m = raw.match(/([+-]?\d+(?:\.\d+)?)/);
  if (!m) return 0;
  const n = Math.abs(parseFloat(m[1]));
  return Number.isFinite(n) ? n : 0;
}

/** Parses signed percent or percentage-point delta for diverging bars. */
export function parseComparisonDeltaSigned(delta?: string): number {
  const raw = (delta ?? "").trim().replace(",", ".");
  if (!raw) return 0;
  const m = raw.match(/([+-]?\d+(?:\.\d+)?)/);
  if (!m) return 0;
  const n = parseFloat(m[1]);
  return Number.isFinite(n) ? n : 0;
}

export function scaleComparisonBars(items: ComparisonBarItem[]): Array<ComparisonBarItem & { barPct: number }> {
  const mags = items.map((it) => parseComparisonDeltaMagnitude(it.delta));
  const max = Math.max(1, ...mags);
  return items.map((it, i) => ({
    ...it,
    barPct: Math.max(8, Math.round((mags[i] / max) * 100)),
  }));
}

export function scaleDivergingComparisonBars(
  items: ComparisonBarItem[],
): Array<ComparisonBarItem & { barPct: number; signedBarPct: number }> {
  const signed = items.map((it) => parseComparisonDeltaSigned(it.delta));
  const maxAbs = Math.max(1, ...signed.map((v) => Math.abs(v)));
  return items.map((it, i) => {
    const absPct = Math.max(6, Math.round((Math.abs(signed[i]) / maxAbs) * 100));
    const sign = signed[i] === 0 ? 0 : signed[i] > 0 ? 1 : -1;
    return {
      ...it,
      barPct: absPct,
      signedBarPct: sign * absPct,
    };
  });
}

/** Half-track width (0–50) from scaled signed magnitude; 100 → full half. */
export function comparisonBarHalfWidthPct(signedBarPct: number): number {
  return Math.min(50, Math.abs(signedBarPct) / 2);
}

/** Shared grid: label | fixed bar track | fixed value column (desktop). */
export const COMPARISON_BAR_GRID_CLASS =
  "sm:grid sm:grid-cols-[minmax(0,1fr)_9rem_4.75rem] sm:gap-x-2 sm:gap-y-1";

export function toneBarClass(tone: FunnelTone): string {
  switch (tone) {
    case "negative":
      return "bg-rose-300";
    case "positive":
      return "bg-emerald-400";
    case "warning":
      return "bg-amber-300";
    default:
      return "bg-slate-300";
  }
}

export function divergingBarClass(signedBarPct: number, tone: FunnelTone): string {
  if (signedBarPct > 0) return "bg-emerald-500";
  if (signedBarPct < 0) return "bg-rose-400";
  if (tone === "warning") return "bg-amber-300";
  return "bg-slate-300";
}

export function isComparisonConversionItem(item: ComparisonBarItem): boolean {
  return classifyComparisonBucket(item.key) === "conversion";
}

export function partitionComparisonItems(items: ComparisonBarItem[]): {
  stages: ComparisonBarItem[];
  conversions: ComparisonBarItem[];
} {
  const stages: ComparisonBarItem[] = [];
  const conversions: ComparisonBarItem[] = [];
  for (const item of items) {
    if (isComparisonConversionItem(item)) {
      conversions.push(item);
    } else {
      stages.push(item);
    }
  }
  return { stages, conversions };
}

export function toneTextClass(tone: FunnelTone): string {
  switch (tone) {
    case "negative":
      return "text-rose-600";
    case "positive":
      return "text-emerald-700";
    case "warning":
      return "text-amber-800";
    default:
      return "text-slate-700";
  }
}
