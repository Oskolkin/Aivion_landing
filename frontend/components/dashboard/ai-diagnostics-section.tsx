"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AccountHealthCard } from "@/components/dashboard/account-health-card";
import { CategoryStatusCardsSection } from "@/components/dashboard/category-status-cards-section";
import {
  SalesFunnelInfoHint,
  SalesFunnelModule,
  buildSalesFunnelSubtitle,
} from "@/components/dashboard/sales-funnel-module";
import { StockCoverageModule, StockCoverageHeaderHint } from "@/components/dashboard/stock-coverage-module";
import { AdEfficiencyModule } from "@/components/dashboard/ad-efficiency-module";
import { WidgetState } from "@/components/dashboard/widget-state";
import { cn } from "@/components/ui/cn";
import { asArray } from "@/lib/as-array";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";
import { TopActionsSection } from "@/components/dashboard/top-actions-section";
import { WhatChangedCard } from "@/components/dashboard/what-changed-card";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  data: DashboardHomeResponse["ai_diagnostics"];
};

export function AIDiagnosticsSection({ data }: Props) {
  const topActions = asArray(data.top_actions);
  return (
    <section className="min-w-0 max-w-full space-y-6">
      <header className="space-y-1.5">
        <h2 className={dashboardTypography.moduleTitle}>AI-диагностика продаж</h2>
        <p className={dashboardTypography.moduleDescription}>
          Состояние кабинета, ключевые риски и изменения за неделю.
        </p>
      </header>

      <div className="grid min-h-0 min-w-0 max-w-full gap-5 lg:grid-cols-[minmax(0,34%)_minmax(0,1fr)] lg:items-stretch lg:gap-6">
        <AccountHealthCard
          data={{
            accountSummary: data.account_summary,
            whatChanged: data.what_changed,
            stockCoverage: data.stock_coverage,
            adEfficiency: data.ad_efficiency,
            topActions,
          }}
        />
        <WhatChangedCard whatChanged={data.what_changed} />
      </div>

      <TopActionsSection actions={data.top_actions} />

      <div className="grid min-w-0 max-w-full gap-4">
        <Card className="min-w-0 max-w-full overflow-hidden rounded-[20px] border-slate-200 bg-white shadow-[0_8px_28px_rgba(15,23,42,0.04)]">
          <section aria-labelledby="sales-funnel-heading" className="min-w-0 max-w-full">
            <CardHeader className="border-b-0 px-5 pb-3 pt-5 sm:px-6 sm:pt-6">
              <div className="flex min-w-0 items-start justify-between gap-2">
                <div className="min-w-0">
                  <CardTitle id="sales-funnel-heading" className={dashboardTypography.moduleTitle}>
                    Воронка продаж
                  </CardTitle>
                  <CardDescription className={cn(dashboardTypography.moduleDescription, "mt-1 break-words text-pretty")}>
                    {buildSalesFunnelSubtitle(data.sales_funnel)}
                  </CardDescription>
                </div>
                <SalesFunnelInfoHint />
              </div>
            </CardHeader>
            <CardContent className="min-w-0 px-5 pb-5 pt-0 sm:px-6 sm:pb-6">
              <SalesFunnelModule salesFunnel={data.sales_funnel} funnelAnalysis={data.funnel_analysis} />
            </CardContent>
          </section>
        </Card>

        <Card className="min-w-0 max-w-full overflow-hidden rounded-[24px] border-slate-200 bg-white shadow-[0_8px_28px_rgba(15,23,42,0.04)]">
          <CardHeader className="border-b border-slate-200/80 px-6 py-6">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0 space-y-1.5">
                <CardTitle className="text-2xl font-bold leading-tight text-slate-950">
                  Покрытие остатков
                </CardTitle>
                <CardDescription className="text-sm leading-relaxed text-slate-600 sm:text-[15px]">
                  На сколько дней хватит текущих остатков при текущем темпе продаж
                </CardDescription>
              </div>
              <StockCoverageHeaderHint />
            </div>
          </CardHeader>
          <CardContent className="space-y-6 px-6 py-7">
            <WidgetState widget={data.stock_coverage}>
              <StockCoverageModule stockCoverage={data.stock_coverage} />
            </WidgetState>
          </CardContent>
        </Card>

        <AdEfficiencyModule widget={data.ad_efficiency} />
      </div>

      <CategoryStatusCardsSection categories={data.category_summaries} />
    </section>
  );
}

