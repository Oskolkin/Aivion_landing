"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewFocusActionView, OverviewFocusActionsView } from "@/lib/overview-view-model";

type RowProps = {
  action: OverviewFocusActionView;
};

export function OverviewFocusActionRow({ action }: RowProps) {
  return (
    <Link
      href={action.href}
      className={cn(
        "group flex h-11 items-center gap-3 rounded-lg px-1.5 py-0 sm:h-12 sm:px-2",
        "hover:bg-aivion-surface-soft focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-aivion-blue",
      )}
      data-testid={`overview-focus-action-${action.order}`}
    >
      <span
        className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#eff8ff] text-xs font-semibold text-aivion-blue"
        aria-hidden
      >
        {action.order}
      </span>

      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium leading-snug text-aivion-text sm:text-[15px]">
          {action.title}
        </span>
        <span className="mt-1 inline-flex items-center gap-2 sm:hidden">
          <span className="rounded-md border border-aivion-border-strong bg-[#eef2f6] px-2 py-0.5 text-[11px] font-semibold text-aivion-text">
            {action.categoryLabel}
          </span>
        </span>
      </span>

      <span className="hidden shrink-0 rounded-md border border-aivion-border-strong bg-[#eef2f6] px-2 py-0.5 text-[11px] font-semibold text-aivion-text sm:inline-flex">
        {action.categoryLabel}
      </span>
      <ChevronRight
        className="h-4 w-4 shrink-0 text-aivion-muted transition-colors group-hover:text-aivion-blue"
        aria-hidden
      />
    </Link>
  );
}

type Props = {
  focus: OverviewFocusActionsView;
};

export function OverviewFocusActions({ focus }: Props) {
  return (
    <section
      className="flex h-full flex-col rounded-[14px] border border-aivion-border bg-white p-3.5 sm:p-4"
      data-testid="overview-focus-actions"
      aria-labelledby="overview-focus-actions-title"
    >
      <h2 id="overview-focus-actions-title" className={dashboardTypography.cardTitleCompact}>
        Фокус на сегодня
      </h2>

      {focus.empty ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{focus.emptyMessage}</p>
      ) : (
        <ul className="mt-2 divide-y divide-aivion-border/80">
          {focus.actions.map((action) => (
            <li key={action.id}>
              <OverviewFocusActionRow action={action} />
            </li>
          ))}
        </ul>
      )}

      <div className="mt-3">
        <Link
          href={focus.allActionsHref}
          className={cn(dashboardTypography.ctaLink, "inline-flex items-center gap-1")}
          data-testid="overview-all-actions-link"
        >
          Все действия
          <span aria-hidden>→</span>
        </Link>
      </div>
    </section>
  );
}
