"use client";

import { OverviewAdvertisingPreview } from "@/components/dashboard/overview/overview-advertising-preview";
import { OverviewFunnelPreview } from "@/components/dashboard/overview/overview-funnel-preview";
import { OverviewStockPreview } from "@/components/dashboard/overview/overview-stock-preview";
import type { OverviewViewModel } from "@/lib/overview-view-model";

type Props = {
  overview: Pick<OverviewViewModel, "funnel" | "stock" | "advertising">;
};

export function OverviewDomainPreviews({ overview }: Props) {
  return (
    <section
      aria-label="Превью разделов"
      className="grid grid-cols-1 items-stretch gap-4 md:grid-cols-2 xl:grid-cols-3"
      data-testid="overview-domain-previews"
    >
      <OverviewFunnelPreview funnel={overview.funnel} />
      <OverviewStockPreview stock={overview.stock} />
      <div className="h-full md:col-span-2 xl:col-span-1">
        <OverviewAdvertisingPreview advertising={overview.advertising} />
      </div>
    </section>
  );
}
