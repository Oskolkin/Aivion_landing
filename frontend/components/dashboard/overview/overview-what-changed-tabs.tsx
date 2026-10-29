"use client";

import { ScrollFadeRail } from "@/components/dashboard/shell/scroll-fade-rail";
import { cn } from "@/components/ui/cn";
import type { OverviewWhatChangedTabId, OverviewWhatChangedTabView } from "@/lib/overview-view-model";

type Props = {
  tabs: OverviewWhatChangedTabView[];
  activeId: OverviewWhatChangedTabId;
  onChange: (id: OverviewWhatChangedTabId) => void;
};

const COUNT_CLASS: Record<OverviewWhatChangedTabId, string> = {
  all: "text-aivion-muted",
  worsened: "text-[#b42318]",
  new_risks: "text-[#b54708]",
  unchanged_risks: "text-aivion-blue",
  improved: "text-[#027a48]",
};

export function OverviewWhatChangedTabs({ tabs, activeId, onChange }: Props) {
  return (
    <ScrollFadeRail
      role="tablist"
      aria-label="Фильтр изменений"
      data-testid="overview-what-changed-tabs"
      activeSelector='[aria-selected="true"]'
      className="-mx-1"
    >
      <div className="flex min-w-max gap-1 px-1 pb-0.5">
        {tabs.map((tab) => {
          const active = tab.id === activeId;
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => onChange(tab.id)}
              className={cn(
                "inline-flex h-8 items-center gap-1.5 rounded-full px-2.5 text-[13px] font-medium transition-colors",
                "focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-aivion-blue",
                active
                  ? "bg-[#eff8ff] text-aivion-blue"
                  : "text-aivion-muted hover:bg-aivion-surface-soft hover:text-aivion-text",
              )}
              data-testid={`overview-what-changed-tab-${tab.id}`}
            >
              <span>{tab.label}</span>
              <span className={cn("tabular-nums", COUNT_CLASS[tab.id])}>{tab.count}</span>
            </button>
          );
        })}
      </div>
    </ScrollFadeRail>
  );
}
