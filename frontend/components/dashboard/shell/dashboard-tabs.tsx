"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ScrollFadeRail } from "@/components/dashboard/shell/scroll-fade-rail";
import { cn } from "@/components/ui/cn";
import {
  DASHBOARD_TABS,
  getDashboardSectionId,
} from "@/lib/dashboard-routes";

export function DashboardTabs() {
  const pathname = usePathname();
  const activeId = getDashboardSectionId(pathname);

  return (
    <ScrollFadeRail
      aria-label="Разделы дашборда"
      data-testid="dashboard-tabs"
      activeSelector='[aria-current="page"]'
      className="-mx-1"
    >
      <nav aria-label="Разделы дашборда">
        <ul className="flex min-w-max items-stretch gap-1 border-b border-aivion-border px-1">
          {DASHBOARD_TABS.map((tab) => {
            const active = tab.id === activeId;
            return (
              <li key={tab.id} className="shrink-0">
                <Link
                  href={tab.href}
                  aria-current={active ? "page" : undefined}
                  data-testid={`dashboard-tab-${tab.id}`}
                  className={cn(
                    "relative inline-flex h-10 items-center whitespace-nowrap px-3 text-sm font-medium transition-colors",
                    "focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-aivion-blue",
                    active
                      ? "text-aivion-blue"
                      : "text-aivion-muted hover:text-aivion-text",
                  )}
                >
                  {tab.label}
                  {active ? (
                    <span
                      aria-hidden
                      className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-aivion-blue"
                    />
                  ) : null}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </ScrollFadeRail>
  );
}
