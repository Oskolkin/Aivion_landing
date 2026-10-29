"use client";

import { OverviewAiSummary } from "@/components/dashboard/overview/overview-ai-summary";
import { OverviewDomainPreviews } from "@/components/dashboard/overview/overview-domain-previews";
import { OverviewFocusActions } from "@/components/dashboard/overview/overview-focus-actions";
import { OverviewKpiGrid } from "@/components/dashboard/overview/overview-kpi-grid";
import { OverviewSkuFocus } from "@/components/dashboard/overview/overview-sku-focus";
import { OverviewWhatChanged } from "@/components/dashboard/overview/overview-what-changed";
import { useDashboardData } from "@/components/dashboard/shell/dashboard-data-provider";
import { buildOverviewViewModel } from "@/lib/overview-view-model";

export function DashboardOverviewPage() {
  const { data } = useDashboardData();
  const overview = data ? buildOverviewViewModel(data) : null;
  if (!overview) return null;

  return (
    <div className="space-y-3.5 sm:space-y-4" data-testid="dashboard-overview">
      <OverviewKpiGrid kpis={overview.kpis} />

      <div className="grid grid-cols-1 items-stretch gap-3.5 xl:grid-cols-12 xl:gap-4">
        <div className="xl:col-span-5">
          <OverviewAiSummary summary={overview.aiSummary} />
        </div>
        <div className="xl:col-span-7">
          <OverviewFocusActions focus={overview.focusActions} />
        </div>
      </div>

      <OverviewWhatChanged whatChanged={overview.whatChanged} />

      <OverviewDomainPreviews overview={overview} />
      <OverviewSkuFocus rows={data!.overview?.sku_focus_preview ?? []} />
    </div>
  );
}
