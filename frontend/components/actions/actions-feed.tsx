"use client";

import { ActionCard } from "@/components/actions/action-card";
import type { ActionItem } from "@/lib/actions-types";
import type { ActionStatusFilter } from "@/lib/actions-view-model";

export function ActionsFeed({
  actions,
  selectedActionKey,
  onSelect,
  statusFilter = "active",
  totalCount,
}: {
  actions: ActionItem[];
  selectedActionKey: string | null;
  onSelect: (actionKey: string) => void;
  statusFilter?: ActionStatusFilter;
  totalCount?: number;
}) {
  const count = totalCount ?? actions.length;

  return (
    <aside className="flex min-h-0 flex-1 flex-col" aria-label="Список действий">
      <div className="sticky top-0 z-10 flex items-center justify-between bg-aivion-bg/95 pb-3 backdrop-blur">
        <h2 className="text-base font-bold text-slate-950 sm:text-lg">
          Действия
          <span className="ml-2 text-sm font-medium tabular-nums text-slate-500">{count}</span>
        </h2>
      </div>
      <div className="actions-scroll min-h-0 flex-1 space-y-2 overflow-y-auto pr-1.5">
        {actions.map((action) => (
          <ActionCard
            key={action.action_key}
            action={action}
            selected={selectedActionKey === action.action_key}
            onSelect={() => onSelect(action.action_key)}
            statusFilter={statusFilter}
          />
        ))}
      </div>
    </aside>
  );
}
