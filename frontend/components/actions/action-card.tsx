"use client";

import { ChevronRight } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { mapActionToListCard, type ActionStatusFilter } from "@/lib/actions-view-model";
import type { ActionItem } from "@/lib/actions-types";

export function ActionCard({
  action,
  selected,
  onSelect,
  statusFilter = "active",
}: {
  action: ActionItem;
  selected: boolean;
  onSelect: () => void;
  statusFilter?: ActionStatusFilter;
}) {
  const vm = mapActionToListCard(action, { statusFilter });

  return (
    <button
      type="button"
      data-testid="action-card"
      onClick={onSelect}
      className={cn(
        "group relative w-full rounded-[14px] border border-slate-200 bg-white p-4 text-left transition-all",
        selected
          ? "border-l-[3px] border-l-aivion-blue bg-[#f3f8ff] shadow-[0_1px_4px_rgba(15,23,42,0.04)]"
          : "hover:border-slate-300 hover:bg-slate-50/70",
      )}
      aria-pressed={selected}
      aria-current={selected ? "true" : undefined}
      aria-label={vm.title}
    >
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center gap-1.5">
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ring-1 ring-inset",
                vm.priority.tone.bg,
                vm.priority.tone.text,
                vm.priority.tone.ring,
              )}
            >
              {vm.priority.label}
            </span>
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ring-1 ring-inset",
                vm.category.tone.bg,
                vm.category.tone.text,
                vm.category.tone.ring,
              )}
            >
              {vm.category.label}
            </span>
            {vm.showStatusBadge ? (
              <span
                className={cn(
                  "inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ring-1 ring-inset",
                  vm.status.tone.bg,
                  vm.status.tone.text,
                  vm.status.tone.ring,
                )}
              >
                {vm.status.label}
              </span>
            ) : null}
          </div>

          <h3
            className={cn(
              "text-[15px] font-semibold leading-[1.35] text-slate-950 line-clamp-3",
              selected ? "font-bold" : "",
            )}
          >
            {vm.title}
          </h3>

          {vm.summary ? (
            <p className="text-sm leading-relaxed text-slate-600 line-clamp-2">{vm.summary}</p>
          ) : null}

          <div className="flex items-end justify-between gap-3 pt-0.5">
            {vm.entitiesLabel ? (
              <span className="text-xs font-medium text-slate-600">{vm.entitiesLabel}</span>
            ) : (
              <span />
            )}
            <span className="shrink-0 text-xs text-slate-500">{vm.dateLabel}</span>
          </div>
        </div>
        <ChevronRight
          className={cn(
            "mt-1 h-5 w-5 shrink-0 transition-colors",
            selected ? "text-aivion-blue" : "text-slate-400 group-hover:text-aivion-blue",
          )}
          aria-hidden="true"
        />
      </div>
    </button>
  );
}
