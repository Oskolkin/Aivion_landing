"use client";

import { cn } from "@/components/ui/cn";

export type TabItem = {
  id: string;
  label: string;
  disabled?: boolean;
};

export type TabsProps = {
  tabs: TabItem[];
  activeId: string;
  onChange: (id: string) => void;
  className?: string;
};

export function Tabs({ tabs, activeId, onChange, className }: TabsProps) {
  return (
    <div
      className={cn(
        "inline-flex flex-wrap gap-1 rounded-aivion-lg border border-aivion-border bg-aivion-surface-soft p-1",
        className,
      )}
      role="tablist"
    >
      {tabs.map((tab) => {
        const active = tab.id === activeId;
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={tab.disabled}
            onClick={() => onChange(tab.id)}
            className={cn(
              "rounded-aivion-md px-3.5 py-1.5 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-aivion-blue disabled:cursor-not-allowed disabled:opacity-50",
              active
                ? "bg-aivion-surface text-aivion-text shadow-aivion-sm"
                : "text-aivion-muted hover:text-aivion-text",
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
