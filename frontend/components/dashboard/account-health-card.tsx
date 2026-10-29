"use client";

import Link from "next/link";
import {
  AlertCircle,
  CircleX,
  Filter,
  Info,
  Lightbulb,
  Megaphone,
  Package,
  TrendingDown,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/components/ui/cn";
import { EmptyNote } from "@/components/ui/empty-note";
import { StatusPill } from "@/components/ui/status-pill";
import { WidgetState } from "@/components/dashboard/widget-state";
import {
  buildAccountHealthDisplayView,
  type HealthDriverIconCategory,
  type HealthImprovementItemView,
  type HealthSeverityTone,
  type AccountHealthDisplayInput,
} from "@/lib/account-health-display";
import { aiTerms } from "@/lib/ai-terms";
import { renderMetricHighlightText } from "@/lib/metric-highlight";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  data: AccountHealthDisplayInput;
};

const PROGRESS_FILL_CLASS: Record<HealthSeverityTone, string> = {
  critical: "bg-[#b42318]",
  high: "bg-[#c4320a]",
  medium: "bg-[#b54708]",
  attention: "bg-[#0284c7]",
  low: "bg-[#027a48]",
  unknown: "bg-aivion-muted-soft",
};

const DRIVER_ICON_CLASS: Record<HealthDriverIconCategory, string> = {
  advertising: "text-[#b54708] bg-[#fffaeb] ring-[#fedf89]/60",
  stock: "text-[#b42318] bg-[#fef3f2] ring-[#fecdca]/60",
  funnel: "text-[#175cd3] bg-[#eff8ff] ring-[#b2ddff]/60",
  sales: "text-[#b42318] bg-[#fef3f2] ring-[#fecdca]/60",
  cancellations: "text-[#b42318] bg-[#fef3f2] ring-[#fecdca]/60",
  risk: "text-[#b54708] bg-[#fffaeb] ring-[#fedf89]/60",
  neutral: "text-aivion-muted bg-aivion-surface-soft ring-aivion-border/60",
};

const INDEX_DESCRIPTION =
  "Индекс учитывает продажи, рекламу, остатки, сигналы и активные риски.";
const DRIVERS_EMPTY = "Основные причины появятся после обновления сводки.";
const IMPROVEMENT_EMPTY = aiTerms.improvementEmpty;
const SCORE_EMPTY = aiTerms.scoreEmpty;

export function AccountHealthCard({ data }: Props) {
  const acc = data.accountSummary;
  const view = buildAccountHealthDisplayView(data);
  const scoreMissing = view.scoreLabel == null;

  return (
    <Card className="flex h-full min-h-0 flex-col rounded-[22px] border border-aivion-border bg-white shadow-[0_4px_18px_rgb(6_27_69/0.06)]">
      <CardHeader className="border-b-0 px-6 pb-0 pt-6 sm:px-7">
        <div className="flex items-start justify-between gap-3">
          <CardTitle className={dashboardTypography.cardTitle}>
            Индекс состояния кабинета
          </CardTitle>
          <span title={INDEX_DESCRIPTION} className="shrink-0">
            <Info className="h-4 w-4 text-aivion-muted-soft" aria-hidden="true" />
          </span>
        </div>
      </CardHeader>
      <CardContent className="px-6 pb-6 pt-4 sm:px-7 sm:pb-7">
        <WidgetState widget={acc}>
          {scoreMissing ? (
            <EmptyNote variant="soft">{SCORE_EMPTY}</EmptyNote>
          ) : (
            <div className="space-y-5">
              <div className="flex flex-wrap items-center gap-3.5">
                <p className={cn("text-[3.25rem] sm:text-[3.5rem]", dashboardTypography.kpiValue)}>{view.scoreLabel}</p>
                <StatusPill
                  variant={view.severityPillVariant}
                  size="sm"
                  className={cn("shrink-0 px-3 py-1", dashboardTypography.button)}
                >
                  {view.severityLabel}
                </StatusPill>
              </div>

              <div className="space-y-2">
                <div className="h-2.5 overflow-hidden rounded-full bg-[#eef2f6]" aria-hidden="true">
                  <div
                    className={cn(
                      "h-full rounded-full transition-[width] duration-300",
                      PROGRESS_FILL_CLASS[view.severityTone],
                    )}
                    style={{ width: `${view.scorePercent}%` }}
                  />
                </div>
                <div className={cn("grid grid-cols-3", dashboardTypography.metadata)}>
                  <span>0</span>
                  <span className="text-center">50</span>
                  <span className="text-right">100</span>
                </div>
              </div>

              <p className={dashboardTypography.body}>{INDEX_DESCRIPTION}</p>

              <div className="border-t border-aivion-border/80 pt-5">
                <p className={dashboardTypography.sectionTitle}>Основные причины снижения</p>
                {view.drivers.length > 0 ? (
                  <ul className="mt-4 space-y-3">
                    {view.drivers.map((driver) => (
                      <li key={`${driver.title}-${driver.reason}`} className="flex items-start gap-3">
                        <DriverIconBadge category={driver.iconCategory} />
                        <div className="min-w-0 flex-1 space-y-1">
                          <p className={cn("whitespace-normal break-words", dashboardTypography.listItemTitle)}>
                            {renderMetricHighlightText(driver.title)}
                          </p>
                          {driver.reason ? (
                            <p className={cn("whitespace-normal break-words", dashboardTypography.body)}>
                              {renderMetricHighlightText(driver.reason)}
                            </p>
                          ) : null}
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <EmptyNote variant="soft" className="mt-4">
                    {DRIVERS_EMPTY}
                  </EmptyNote>
                )}
              </div>

              <div className="border-t border-aivion-border/80 pt-5" data-testid="health-improvement-block">
                <p className={dashboardTypography.sectionTitle}>Что улучшит индекс</p>
                {view.improvementIntro || view.improvementItems.length > 0 ? (
                  <div className="mt-4 space-y-4">
                    {view.improvementIntro ? (
                      <p
                        className={cn("whitespace-normal break-words leading-relaxed", dashboardTypography.body, "text-slate-800")}
                        data-testid="health-improvement-intro"
                      >
                        {renderMetricHighlightText(view.improvementIntro)}
                      </p>
                    ) : null}
                    {view.improvementItems.length > 0 ? (
                      <ul className="space-y-3" data-testid="health-improvement-items">
                        {view.improvementItems.map((item) => (
                          <ImprovementItemRow key={`${item.title}-${item.explanation}`} item={item} />
                        ))}
                      </ul>
                    ) : null}
                  </div>
                ) : (
                  <EmptyNote variant="soft" className="mt-4">
                    {IMPROVEMENT_EMPTY}
                  </EmptyNote>
                )}
              </div>
            </div>
          )}
        </WidgetState>
      </CardContent>
    </Card>
  );
}

function ImprovementItemRow({ item }: { item: HealthImprovementItemView }) {
  const content = (
    <div className="flex items-start gap-3 rounded-xl border border-[#e8eef5] bg-[#f8fafc]/90 p-4">
      <span
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#eff8ff] text-[#175cd3] ring-1 ring-inset ring-[#b2ddff]/50"
        aria-hidden="true"
      >
        <Lightbulb className="h-5 w-5" />
      </span>
      <div className="min-w-0 flex-1 space-y-1.5">
        <p className={cn(dashboardTypography.listItemTitle)}>{item.title}</p>
        <p className={cn("whitespace-normal break-words leading-relaxed", dashboardTypography.body, "text-slate-800")}>
          {renderMetricHighlightText(item.explanation)}
        </p>
        {item.factorLabel && !/^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(item.factorLabel) ? (
          <p className={cn(dashboardTypography.metadata, "text-slate-500")}>{item.factorLabel}</p>
        ) : null}
        {item.actionHref ? (
          <p className={cn(dashboardTypography.metadata, "font-medium text-[#175cd3]")}>Открыть действие</p>
        ) : null}
      </div>
    </div>
  );

  if (item.actionHref) {
    return (
      <li>
        <Link href={item.actionHref} className="block transition hover:opacity-95">
          {content}
        </Link>
      </li>
    );
  }
  return <li>{content}</li>;
}

function DriverIconBadge({ category }: { category: HealthDriverIconCategory }) {
  const className = "h-5 w-5";

  return (
    <span
      className={cn(
        "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ring-1 ring-inset",
        DRIVER_ICON_CLASS[category],
      )}
      aria-hidden="true"
    >
      {category === "advertising" ? <Megaphone className={className} /> : null}
      {category === "stock" ? <Package className={className} /> : null}
      {category === "funnel" ? <Filter className={className} /> : null}
      {category === "sales" ? <TrendingDown className={className} /> : null}
      {category === "cancellations" ? <CircleX className={className} /> : null}
      {category === "risk" ? <AlertCircle className={className} /> : null}
      {category === "neutral" ? <Info className={className} /> : null}
    </span>
  );
}
