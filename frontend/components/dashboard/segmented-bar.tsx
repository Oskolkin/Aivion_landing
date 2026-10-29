import type { StockCoverageSegment } from "@/lib/dashboard-home-types";

const SEGMENT_COLORS: Record<string, string> = {
  critical_0_3: "bg-red-500",
  shortage_4_7: "bg-orange-500",
  planning_8_14: "bg-amber-400",
  healthy_14plus: "bg-emerald-500",
  surplus: "bg-violet-500",
};

type SegmentedBarProps = {
  segments: StockCoverageSegment[];
  total?: number;
};

export function SegmentedBar({ segments, total }: SegmentedBarProps) {
  const sum = total ?? segments.reduce((acc, s) => acc + s.sku_count, 0);
  if (sum <= 0) {
    return <p className="text-sm text-aivion-muted-soft">Нет данных для сегментации.</p>;
  }
  return (
    <div className="space-y-3">
      <div className="flex h-3 overflow-hidden rounded-full bg-aivion-surface-soft">
        {segments.map((seg) => {
          if (seg.sku_count <= 0) return null;
          const pct = (seg.sku_count / sum) * 100;
          return (
            <div
              key={seg.key}
              className={`${SEGMENT_COLORS[seg.key] ?? "bg-gray-400"} transition-all`}
              style={{ width: `${pct}%` }}
              title={`${seg.label}: ${seg.sku_count}`}
            />
          );
        })}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
        {segments.map((seg) => (
          <div key={seg.key} className="rounded-lg border border-aivion-border bg-aivion-surface-soft/80 px-2 py-1.5 text-xs">
            <div className="flex items-center gap-1.5">
              <span className={`inline-block h-2 w-2 rounded-full ${SEGMENT_COLORS[seg.key] ?? "bg-gray-400"}`} />
              <span className="font-medium text-aivion-text">{seg.label}</span>
            </div>
            <p className="mt-0.5 tabular-nums text-aivion-muted">{seg.sku_count} SKU</p>
          </div>
        ))}
      </div>
    </div>
  );
}
