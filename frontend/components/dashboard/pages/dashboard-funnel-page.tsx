"use client";

import { useDashboardData } from "@/components/dashboard/shell/dashboard-data-provider";
import { formatMetric } from "@/lib/metric-format";
import type { FunnelProjection } from "@/lib/publication-dashboard";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

const STAGE_LABEL: Record<string, string> = {
  PDP_IMPRESSIONS: "Показы карточки",
  PDP_CART_ADDS: "В корзину",
  SEARCH_IMPRESSIONS: "Показы в поиске",
  SEARCH_CART_ADDS: "В корзину из поиска",
};

function FunnelStateBoundary({ funnel }: { funnel: FunnelProjection }) {
  if (funnel.state === "FAILED") {
    return <p className={dashboardTypography.emptyState} data-testid="funnel-failed">Не удалось получить воронку.</p>;
  }
  if (funnel.state === "UNAVAILABLE") {
    return (
      <p className={dashboardTypography.emptyState} data-testid="funnel-unavailable">
        Воронка недоступна. {funnel.reason_code === "SUBSCRIPTION_REQUIRED" ? "Нужна подписка на аналитику." : funnel.reason_code === "NO_COMPATIBLE_CHAIN" ? "Нет совместимой цепочки этапов." : "Метрики недоступны."}
      </p>
    );
  }
  if (funnel.state === "READY_EMPTY") {
    return (
      <p className={dashboardTypography.emptyState} data-testid="funnel-ready-empty">
        За выбранный период активности воронки не было.
      </p>
    );
  }
  const widths = [100, 78, 58, 40];
  return (
    <div data-testid="funnel-visualization" className="space-y-4">
      {funnel.state === "PARTIAL" ? (
        <p className="text-sm text-amber-800" data-testid="funnel-partial">
          Доступна часть воронки
        </p>
      ) : null}
      {funnel.limitations?.includes("INSUFFICIENT_HISTORY") ? (
        <p className="text-sm text-slate-500">Недостаточно истории для сравнения</p>
      ) : null}
      <ul className="space-y-3">
        {funnel.stages.map((stage, i) => (
          <li key={stage.key} className="flex items-center gap-3">
            <div
              className="h-8 rounded-sm bg-sky-400"
              style={{ width: `${widths[Math.min(i, widths.length - 1)]}%` }}
              aria-hidden
            />
            <div className="min-w-[8rem]">
              <p className="text-sm font-medium">{STAGE_LABEL[stage.key] ?? stage.key}</p>
              <p className="text-sm tabular-nums">
                {stage.amount != null ? formatMetric("COUNT", stage.amount) : "—"}
                {stage.previous_amount == null && funnel.limitations?.includes("INSUFFICIENT_HISTORY") ? (
                  <span className="ml-2 text-slate-400">—</span>
                ) : null}
              </p>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function FunnelDashboardContent() {
  const { data } = useDashboardData();
  const funnel = data?.funnel;
  if (!funnel) return null;
  return (
    <div className="min-w-0 space-y-4" data-testid="dashboard-funnel-page">
      <header>
        <h2 className={cn(dashboardTypography.moduleTitle, "text-lg")}>Воронка продаж</h2>
      </header>
      <FunnelStateBoundary funnel={funnel} />
      <div className="grid gap-3 xl:grid-cols-2" data-testid="funnel-bottom-row">
        <section className="rounded-[14px] border border-aivion-border bg-white p-4" data-testid="funnel-insight-card">
          <h3 className="text-sm font-semibold">Инсайт</h3>
          <p className="mt-2 text-sm text-slate-700">
            {funnel.insight?.statement ?? "Инсайт появится из публикации Deep."}
          </p>
        </section>
        <section className="rounded-[14px] border border-aivion-border bg-white p-4" data-testid="funnel-related-actions">
          <h3 className="text-sm font-semibold">Связанные действия</h3>
          <p className="mt-2 text-sm text-slate-600">
            {(funnel.related_action_local_refs ?? []).length} из той же публикации
          </p>
        </section>
      </div>
    </div>
  );
}

export function DashboardFunnelPage() {
  return <FunnelDashboardContent />;
}
