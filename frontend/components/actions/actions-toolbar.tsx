"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, Search } from "lucide-react";
import { cn } from "@/components/ui/cn";
import {
  CATEGORY_FILTER_OPTIONS,
  STATUS_FILTER_OPTIONS,
  counterForCategory,
  counterForStatus,
} from "@/lib/actions-filters";
import {
  ACTION_SORT_OPTIONS,
  type ActionCategoryFilter,
  type ActionSortKey,
  type ActionStatusFilter,
} from "@/lib/actions-view-model";
import type { ActionCounters } from "@/lib/actions-types";

function CategoryChip({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex h-9 shrink-0 items-center gap-2 rounded-full border px-3.5 text-[13px] font-medium transition-colors",
        active
          ? "border-aivion-blue bg-[#eff8ff] text-aivion-blue"
          : "border-slate-200 bg-white text-slate-700 hover:border-aivion-blue/40",
      )}
      aria-pressed={active}
    >
      <span>{label}</span>
      {count != null ? (
        <span
          className={cn(
            "inline-flex min-w-[1.25rem] items-center justify-center rounded-full px-1.5 py-0.5 text-[11px] font-semibold tabular-nums leading-none",
            active ? "bg-white/80 text-aivion-blue" : "bg-slate-100 text-slate-600",
          )}
        >
          {count}
        </span>
      ) : null}
    </button>
  );
}

export function ActionsToolbar({
  status,
  category,
  sort,
  query,
  counters,
  onStatusChange,
  onCategoryChange,
  onSortChange,
  onQueryChange,
}: {
  status: ActionStatusFilter;
  category: ActionCategoryFilter;
  sort: ActionSortKey;
  query: string;
  counters: ActionCounters;
  onStatusChange: (value: ActionStatusFilter) => void;
  onCategoryChange: (value: ActionCategoryFilter) => void;
  onSortChange: (value: ActionSortKey) => void;
  onQueryChange: (value: string) => void;
}) {
  const [draftQuery, setDraftQuery] = useState(query);
  const [moreOpen, setMoreOpen] = useState(false);
  const moreRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setDraftQuery(query);
  }, [query]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      if (draftQuery !== query) onQueryChange(draftQuery);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [draftQuery, onQueryChange, query]);

  useEffect(() => {
    if (!moreOpen) return;
    const onDoc = (event: MouseEvent) => {
      if (moreRef.current && !moreRef.current.contains(event.target as Node)) {
        setMoreOpen(false);
      }
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [moreOpen]);

  const { primaryCategories, overflowCategories } = useMemo(() => {
    const withCounts = CATEGORY_FILTER_OPTIONS.map((option) => ({
      ...option,
      count: counterForCategory(option.value, counters),
    }));

    const all = withCounts.find((item) => item.value === "all")!;
    const nonZero = withCounts.filter((item) => item.value !== "all" && item.count > 0);
    const zero = withCounts.filter((item) => item.value !== "all" && item.count === 0);
    const primary = [all, ...nonZero.slice(0, 4)];
    const overflow = [...nonZero.slice(4), ...zero];
    const selectedInOverflow = overflow.find((item) => item.value === category);
    if (selectedInOverflow && !primary.some((item) => item.value === category)) {
      primary.push(selectedInOverflow);
    }
    return {
      primaryCategories: primary,
      overflowCategories: overflow.filter((item) => !primary.some((p) => p.value === item.value)),
    };
  }, [category, counters]);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <nav
          aria-label="Категории действий"
          className="flex min-w-0 flex-1 flex-wrap items-center gap-2"
        >
          {primaryCategories.map((item) => (
            <CategoryChip
              key={item.value}
              label={item.label}
              count={item.count}
              active={category === item.value}
              onClick={() => onCategoryChange(item.value)}
            />
          ))}
          {overflowCategories.length > 0 ? (
            <div className="relative" ref={moreRef}>
              <button
                type="button"
                className="inline-flex h-9 items-center gap-1 rounded-full border border-slate-200 bg-white px-3 text-[13px] font-medium text-slate-700"
                aria-expanded={moreOpen}
                aria-haspopup="menu"
                onClick={() => setMoreOpen((open) => !open)}
              >
                Ещё
                <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
              {moreOpen ? (
                <div
                  role="menu"
                  className="absolute left-0 z-30 mt-2 min-w-[220px] rounded-xl border border-slate-200 bg-white py-1 shadow-lg"
                >
                  {overflowCategories.map((item) => (
                    <button
                      key={item.value}
                      type="button"
                      role="menuitem"
                      className={cn(
                        "flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50",
                        category === item.value ? "text-aivion-blue" : "text-slate-700",
                      )}
                      onClick={() => {
                        onCategoryChange(item.value);
                        setMoreOpen(false);
                      }}
                    >
                      <span>{item.label}</span>
                      <span className="tabular-nums text-slate-500">{item.count}</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </nav>

        <div
          role="group"
          aria-label="Статусы действий"
          className="inline-flex h-10 shrink-0 items-center rounded-full border border-slate-200 bg-white p-1"
        >
          {STATUS_FILTER_OPTIONS.map((item) => {
            const count = counterForStatus(item.value, counters);
            const active = status === item.value;
            return (
              <button
                key={item.value}
                type="button"
                onClick={() => onStatusChange(item.value)}
                className={cn(
                  "inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-[13px] font-medium transition-colors",
                  active ? "bg-[#eff8ff] text-aivion-blue" : "text-slate-600 hover:text-slate-900",
                )}
                aria-pressed={active}
              >
                <span>{item.label}</span>
                <span className="tabular-nums text-[11px] opacity-80">{count}</span>
              </button>
            );
          })}
        </div>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
        <label className="relative min-w-0 flex-1 sm:max-w-[320px]">
          <span className="sr-only">Поиск по действиям</span>
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
            aria-hidden="true"
          />
          <input
            type="search"
            value={draftQuery}
            onChange={(event) => setDraftQuery(event.target.value)}
            placeholder="Поиск по действиям…"
            className="h-10 w-full rounded-xl border border-slate-200 bg-white pl-9 pr-3 text-sm text-slate-800 outline-none ring-aivion-blue/30 placeholder:text-slate-400 focus:ring-2"
          />
        </label>
        <label className="inline-flex h-10 items-center gap-2 rounded-xl border border-slate-200 bg-white px-3 text-sm text-slate-700">
          <span className="whitespace-nowrap text-slate-500">Сортировка:</span>
          <select
            value={sort}
            onChange={(event) => onSortChange(event.target.value as ActionSortKey)}
            className="bg-transparent text-sm font-medium text-slate-800 outline-none"
          >
            {ACTION_SORT_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>
    </div>
  );
}

/** @deprecated Prefer ActionsToolbar */
export { ActionsToolbar as ActionsFilterBar };
