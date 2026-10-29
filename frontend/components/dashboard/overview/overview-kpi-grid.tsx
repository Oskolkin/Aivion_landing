"use client";

import { OverviewKpiCard } from "@/components/dashboard/overview/overview-kpi-card";
import type { OverviewKpiCardView } from "@/lib/overview-view-model";

type Props = {
  kpis: OverviewKpiCardView[];
};

export function OverviewKpiGrid({ kpis }: Props) {
  return (
    <section
      aria-label="Ключевые показатели"
      className="grid grid-cols-1 gap-3 md:grid-cols-3 lg:grid-cols-3 lg:gap-3.5"
      data-testid="overview-kpi-grid"
    >
      {kpis.map((kpi) => (
        <OverviewKpiCard key={kpi.key} kpi={kpi} />
      ))}
    </section>
  );
}
