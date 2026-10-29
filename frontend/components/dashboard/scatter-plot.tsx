import type { AdEfficiencyPoint } from "@/lib/dashboard-home-types";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

type ScatterPlotProps = {
  points: AdEfficiencyPoint[];
};

const SIGNAL_COLOR: Record<string, string> = {
  strong: "fill-emerald-500",
  ok: "fill-blue-500",
  weak: "fill-amber-500",
  waste: "fill-red-500",
};

export function AdScatterPlot({ points }: ScatterPlotProps) {
  if (points.length === 0) return null;

  if (points.length <= 3) {
    return (
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {points.map((p) => (
          <div key={p.campaign_external_id} className={cn("rounded-lg border border-aivion-border p-3", dashboardTypography.body)}>
            <p className={cn("font-medium whitespace-normal break-words text-slate-800")}>{p.campaign_name}</p>
            <p className={cn("mt-1", dashboardTypography.secondary)}>
              Расход {p.spend.toLocaleString("ru-RU")} ₽ · ДРР{" "}
              {p.drr != null ? `${(p.drr * 100).toFixed(1)}%` : "—"}
            </p>
            <p className={dashboardTypography.metadata}>{p.efficiency_signal}</p>
          </div>
        ))}
      </div>
    );
  }

  const width = 320;
  const height = 180;
  const pad = 24;
  const maxSpend = Math.max(...points.map((p) => p.spend), 1);
  const maxDRR = Math.max(...points.map((p) => p.drr ?? 0), 0.01);

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full max-w-md" role="img" aria-label="Расход vs ДРР">
      <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} stroke="#e5e7eb" />
      <line x1={pad} y1={pad} x2={pad} y2={height - pad} stroke="#e5e7eb" />
      {points.map((p) => {
        const x = pad + (p.spend / maxSpend) * (width - pad * 2);
        const drr = p.drr ?? 0;
        const y = height - pad - (drr / maxDRR) * (height - pad * 2);
        const r = Math.min(14, 4 + Math.sqrt(p.revenue) / 200);
        const color = SIGNAL_COLOR[p.efficiency_signal] ?? "fill-gray-400";
        return <circle key={p.campaign_external_id} cx={x} cy={y} r={r} className={color} opacity={0.85} />;
      })}
      <text x={width / 2} y={height - 4} textAnchor="middle" className={dashboardTypography.chartAxis}>
        Расход
      </text>
      <text
        x={8}
        y={height / 2}
        textAnchor="middle"
        transform={`rotate(-90 8 ${height / 2})`}
        className={dashboardTypography.chartAxis}
      >
        ДРР
      </text>
    </svg>
  );
}
