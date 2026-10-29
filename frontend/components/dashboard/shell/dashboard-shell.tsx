"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { DashboardDataProvider, useDashboardData } from "@/components/dashboard/shell/dashboard-data-provider";
import { DashboardPeriodControls } from "@/components/dashboard/shell/dashboard-period-controls";
import { DashboardTabs } from "@/components/dashboard/shell/dashboard-tabs";
import { buttonClassNames } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { PageHeader } from "@/components/ui/page-header";
import { buildOverviewPeriodControls } from "@/lib/overview-view-model";
import { DASHBOARD_ROUTES } from "@/lib/dashboard-routes";

function DashboardSkeleton() {
  return (
    <div className="space-y-3" aria-busy="true" aria-live="polite" data-testid="dashboard-skeleton">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div className="h-28 animate-pulse rounded-[14px] bg-slate-100" />
        <div className="h-28 animate-pulse rounded-[14px] bg-slate-100" />
        <div className="h-28 animate-pulse rounded-[14px] bg-slate-100" />
      </div>
      <div className="h-40 animate-pulse rounded-[14px] bg-slate-100" />
    </div>
  );
}

function DashboardShellInner({ children }: { children: ReactNode }) {
  const { data, loading, error, reload } = useDashboardData();
  const pathname = usePathname();
  const isFunnelPage = pathname === DASHBOARD_ROUTES.funnel;
  const isStockPage = pathname === DASHBOARD_ROUTES.stock;
  const period = data ? buildOverviewPeriodControls(data) : null;

  return (
    <main className="min-w-0 max-w-full space-y-4 overflow-x-clip p-5 sm:space-y-5 sm:p-6">
      <PageHeader
        title={isStockPage ? "Остатки" : "Дашборд"}
        subtitle={
          isFunnelPage
            ? "Воронка продаж"
            : isStockPage
              ? "Покрытие и остатки"
              : "Состояние магазина и приоритеты на сегодня"
        }
        className="pb-0"
      >
        <DashboardPeriodControls period={period} variant={isFunnelPage || isStockPage ? "split" : "card"} />
      </PageHeader>

      <DashboardTabs />

      {loading ? (
        <DashboardSkeleton />
      ) : error ? (
        <ErrorState
          title="Дашборд недоступен"
          message={error}
          action={
            <button type="button" className={buttonClassNames("secondary")} onClick={reload}>
              Повторить
            </button>
          }
        />
      ) : !data ? (
        <EmptyState title="Нет данных" message="Данных пока недостаточно для формирования дашборда." />
      ) : data.state === "PRE_ANALYSIS" ? (
        <EmptyState
          title="Анализ ещё не опубликован"
          message="Дашборд появится после первой публикации Aivion. Frontend не подставляет исторические значения."
        />
      ) : data.state === "FAILED" ? (
        <ErrorState
          title="Публикация недоступна"
          message={data.error?.message_key ?? "Не удалось прочитать текущую публикацию."}
          action={
            <button type="button" className={buttonClassNames("secondary")} onClick={reload}>
              Повторить
            </button>
          }
        />
      ) : (
        children
      )}
    </main>
  );
}

export function DashboardShell({ children }: { children: ReactNode }) {
  return (
    <DashboardDataProvider>
      <DashboardShellInner>{children}</DashboardShellInner>
    </DashboardDataProvider>
  );
}
