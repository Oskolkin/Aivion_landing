"use client";

import { useMemo, useState } from "react";
import { OverviewWhatChangedRow } from "@/components/dashboard/overview/overview-what-changed-row";
import { OverviewWhatChangedTabs } from "@/components/dashboard/overview/overview-what-changed-tabs";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type {
  OverviewWhatChangedTabId,
  OverviewWhatChangedView,
} from "@/lib/overview-view-model";

type Props = {
  whatChanged: OverviewWhatChangedView;
};

export function OverviewWhatChanged({ whatChanged }: Props) {
  const [activeTab, setActiveTab] = useState<OverviewWhatChangedTabId>("all");
  const rows = useMemo(
    () => whatChanged.rowsByTab[activeTab] ?? [],
    [whatChanged.rowsByTab, activeTab],
  );

  return (
    <section
      className="flex h-full flex-col rounded-[14px] border border-aivion-border bg-white p-3.5 sm:p-4"
      data-testid="overview-what-changed"
      aria-labelledby="overview-what-changed-title"
    >
      <h2 id="overview-what-changed-title" className={dashboardTypography.cardTitleCompact}>
        Динамика и риски
      </h2>

      {whatChanged.unavailable ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>
          {whatChanged.unavailableMessage}
        </p>
      ) : (
        <>
          <div className="mt-3">
            <OverviewWhatChangedTabs
              tabs={whatChanged.tabs}
              activeId={activeTab}
              onChange={setActiveTab}
            />
          </div>

          {rows.length === 0 ? (
            <p className={cn("mt-3", dashboardTypography.emptyState)}>
              В этой группе пока нет изменений.
            </p>
          ) : (
            <ul className="mt-2 divide-y divide-aivion-border/80">
              {rows.map((row) => (
                <li key={row.id}>
                  <OverviewWhatChangedRow row={row} />
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}
