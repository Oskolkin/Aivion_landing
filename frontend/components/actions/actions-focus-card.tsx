"use client";

import { ArrowRight, HelpCircle, Sparkles } from "lucide-react";
import { buttonClassNames } from "@/components/ui/button";
import { cn } from "@/components/ui/cn";
import { mapDailyFocus, type DailyFocusViewModel } from "@/lib/actions-view-model";
import type { ActionItem, ActionsAIFocus } from "@/lib/actions-types";

export function DailyFocusBanner({
  focus,
  onOpenPriority,
}: {
  focus: DailyFocusViewModel;
  onOpenPriority?: () => void;
}) {
  if (focus.empty) {
    return (
      <section className="rounded-[16px] border border-emerald-100 bg-emerald-50/50 px-5 py-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-emerald-700">
          <Sparkles className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>AI-фокус дня</span>
        </div>
        <p className="mt-1.5 text-[15px] font-semibold text-slate-900">{focus.headline}</p>
      </section>
    );
  }

  return (
    <section className="rounded-[16px] border border-sky-100 bg-sky-50/60 px-5 py-4 shadow-[0_2px_10px_rgba(15,23,42,0.03)]">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_auto_auto] lg:items-center">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-semibold text-aivion-blue">
            <Sparkles className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>AI-фокус дня</span>
          </div>
          <h2 className="mt-1.5 text-[17px] font-bold leading-snug text-slate-950 sm:text-lg">
            {focus.headline}
          </h2>
          {focus.recommendation ? (
            <p className="mt-1 text-sm leading-relaxed text-slate-600">{focus.recommendation}</p>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-4 sm:gap-6">
          {focus.affectedLabel ? (
            <div>
              <p className="text-xl font-bold tabular-nums text-slate-950">
                {focus.affectedLabel.split(" ")[0]}
              </p>
              <p className="text-xs text-slate-500">
                {focus.affectedLabel.includes("SKU")
                  ? "SKU затронуто"
                  : focus.affectedLabel.replace(/^\d+\s+/, "")}
              </p>
            </div>
          ) : null}
          {focus.riskLabel ? (
            <div>
              <p className="inline-flex items-center gap-1 text-xl font-bold tabular-nums text-slate-950">
                {focus.riskLabel}
                {focus.riskTooltip ? (
                  <span
                    className="inline-flex text-slate-400"
                    title={focus.riskTooltip}
                    aria-label={focus.riskTooltip}
                  >
                    <HelpCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  </span>
                ) : null}
              </p>
              <p className="text-xs text-slate-500">потенциальный риск потери выручки</p>
            </div>
          ) : null}
          {focus.priorityLabel ? (
            <span
              className={cn(
                "inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ring-1 ring-inset",
                focus.priorityTone?.bg,
                focus.priorityTone?.text,
                focus.priorityTone?.ring,
              )}
            >
              {focus.priorityLabel}
            </span>
          ) : null}
        </div>

        {onOpenPriority ? (
          <button
            type="button"
            className={buttonClassNames(
              "primary",
              "inline-flex h-10 shrink-0 items-center gap-2 self-start px-4 lg:self-center",
            )}
            onClick={onOpenPriority}
          >
            {focus.ctaLabel}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </button>
        ) : null}
      </div>
    </section>
  );
}

/** Backward-compatible wrapper used by older tests and callers. */
export function ActionsFocusCard({
  focus,
  priorityAction,
  hasOpenActions,
  onOpenPriority,
}: {
  focus?: ActionsAIFocus;
  priorityAction?: Pick<ActionItem, "title" | "summary" | "priority" | "next_step" | "evidence" | "affected_entities" | "affected_objects_total" | "scope_type" | "operational_detail" | "category" | "type">;
  priorityCount?: number;
  hasOpenActions: boolean;
  onOpenPriority?: () => void;
}) {
  const vm = mapDailyFocus({
    focus,
    priorityAction: priorityAction as ActionItem | undefined,
    hasOpenActions,
  });
  return <DailyFocusBanner focus={vm} onOpenPriority={onOpenPriority} />;
}
