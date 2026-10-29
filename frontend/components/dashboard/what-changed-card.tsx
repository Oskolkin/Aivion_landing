"use client";

import { useState } from "react";
import Link from "next/link";
import {
  AlertCircle,
  AlertTriangle,
  BarChart3,
  Bell,
  CheckCircle2,
  Megaphone,
  MinusCircle,
  Package,
  Shield,
  Sparkles,
  TrendingDown,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/components/ui/cn";
import { renderMetricHighlightText } from "@/lib/metric-highlight";
import { dashboardTypography } from "@/lib/dashboard-typography";
import { WidgetState } from "@/components/dashboard/widget-state";
import {
  buildWhatChangedItemDisplay,
  buildWhatChangedView,
  getWhatChangedToneAccent,
  getWhatChangedToneBackground,
  getWhatChangedToneBorder,
  inferWhatChangedItemIconCategory,
  severityBadgeLabel,
  splitMainConclusionParagraphs,
  type WhatChangedGroupView,
  type WhatChangedTone,
} from "@/lib/what-changed-view";
import type { DashboardHomeResponse, WhatChangedItem } from "@/lib/dashboard-home-types";

type Props = {
  whatChanged: DashboardHomeResponse["ai_diagnostics"]["what_changed"];
};

const HEADER_ICON_WRAP: Record<WhatChangedTone, string> = {
  negative: "bg-[#f04438] text-white shadow-[0_2px_8px_rgb(240_68_56/0.28)]",
  warning: "bg-[#f79009] text-white shadow-[0_2px_8px_rgb(247_144_9/0.28)]",
  neutral: "bg-slate-500 text-white shadow-[0_2px_8px_rgb(100_116_139/0.22)]",
  positive: "bg-[#12b76a] text-white shadow-[0_2px_8px_rgb(18_183_106/0.28)]",
  empty_positive: "bg-slate-300 text-white shadow-none",
};

export function WhatChangedCard({ whatChanged }: Props) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const view = buildWhatChangedView(whatChanged, { expandedGroups });

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <Card
      data-testid="what-changed-card"
      className="flex h-full min-h-0 flex-1 flex-col rounded-[22px] border border-aivion-border bg-white shadow-[0_4px_18px_rgb(6_27_69/0.06)]"
    >
      <CardHeader className="shrink-0 border-b-0 px-6 pb-0 pt-6 sm:px-7">
        <CardTitle className={dashboardTypography.cardTitle}>Что изменилось за неделю</CardTitle>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col px-6 pb-6 pt-5 sm:px-7 sm:pb-7">
        <WidgetState widget={whatChanged}>
          <div className="flex min-h-0 flex-1 flex-col">
            <MainConclusionPlaque text={view.summary} />

            {/* Desktop/tablet contract: 2×2 equal-height groups. Never 4 columns. */}
            <div
              data-testid="what-changed-groups-grid"
              className="grid min-h-0 flex-1 grid-cols-1 gap-4 sm:grid-cols-2 sm:grid-rows-2"
            >
              {view.groups.map((group) => (
                <WhatChangedGroupCard
                  key={group.key}
                  group={group}
                  expanded={expandedGroups.has(group.key)}
                  onToggleExpand={() => toggleGroup(group.key)}
                />
              ))}
            </div>
          </div>
        </WidgetState>
      </CardContent>
    </Card>
  );
}

function MainConclusionPlaque({ text }: { text: string }) {
  const paragraphs = splitMainConclusionParagraphs(text);

  return (
    <div
      data-testid="what-changed-main-conclusion"
      className="mb-5 shrink-0 rounded-[18px] border border-[#93c5fd] bg-[#eff8ff] px-5 py-4 sm:px-5 sm:py-[18px]"
    >
      <div className="flex min-w-0 items-start gap-4">
        <div
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-[#2563eb] text-white shadow-[0_4px_14px_rgb(37_99_235/0.35)]"
          aria-hidden="true"
        >
          <Sparkles className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1 space-y-2">
          <p className={dashboardTypography.aiSummaryLabel}>Главный вывод</p>
          {paragraphs.map((paragraph, index) => (
            <p
              key={`${index}-${paragraph.slice(0, 24)}`}
              className={cn(
                "whitespace-normal break-words text-[14px] leading-[1.5] text-slate-800",
                index === 0 ? "font-medium" : null,
                dashboardTypography.aiSummary,
              )}
            >
              {renderMetricHighlightText(paragraph)}
            </p>
          ))}
        </div>
      </div>
    </div>
  );
}

function WhatChangedGroupCard({
  group,
  expanded,
  onToggleExpand,
}: {
  group: WhatChangedGroupView;
  expanded: boolean;
  onToggleExpand: () => void;
}) {
  const hasItems = group.items.length > 0;

  return (
    <div
      data-testid={`what-changed-group-${group.key}`}
      className={cn(
        "flex h-full min-h-[12rem] flex-col rounded-[18px] border p-[18px]",
        getWhatChangedToneBorder(group.tone),
        getWhatChangedToneBackground(group.tone),
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <span
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-full",
            HEADER_ICON_WRAP[group.tone],
          )}
          aria-hidden="true"
        >
          <GroupHeaderIcon group={group} />
        </span>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <p className={cn("min-w-0 truncate text-[16px] font-semibold leading-tight text-slate-900", dashboardTypography.listItemTitle)}>
            {group.label}
          </p>
          <span
            className={cn(
              "inline-flex h-5 shrink-0 items-center justify-center rounded-full px-1.5 text-[11px] font-semibold text-white",
              getWhatChangedToneAccent(group.tone),
            )}
          >
            {group.totalCount}
          </span>
        </div>
      </div>

      {hasItems ? (
        <div className="mt-3.5 flex min-h-0 flex-1 flex-col gap-3">
          {group.items.map((item, index) => (
            <WhatChangedItemCard
              key={`${group.key}-${item.change_id || item.label}-${index}`}
              item={item}
              tone={group.tone}
            />
          ))}
          {group.remainingCount > 0 ? (
            <button
              type="button"
              onClick={onToggleExpand}
              className="mt-auto self-start pt-1 text-left text-[13px] font-medium text-slate-600 underline-offset-2 hover:text-slate-900 hover:underline"
            >
              {expanded ? "Свернуть" : `Показать ещё ${group.remainingCount}`}
            </button>
          ) : null}
        </div>
      ) : (
        <div className="mt-3.5 flex min-h-0 flex-1 flex-col justify-start rounded-xl border border-white/80 bg-white/90 px-3.5 py-3">
          <p className="whitespace-normal break-words text-[13px] leading-[1.5] text-slate-600">
            {renderMetricHighlightText(group.summary)}
          </p>
        </div>
      )}
    </div>
  );
}

function GroupHeaderIcon({ group }: { group: WhatChangedGroupView }) {
  const className = "h-5 w-5";
  if (group.key === "improved" && group.items.length === 0) {
    return <MinusCircle className={className} />;
  }
  if (group.key === "worsened") return <TrendingDown className={className} />;
  if (group.key === "new_risks") return <Bell className={className} />;
  if (group.key === "unchanged_risks") return <Shield className={className} />;
  if (group.key === "improved") return <CheckCircle2 className={className} />;
  return <AlertTriangle className={className} />;
}

function WhatChangedItemCard({ item, tone }: { item: WhatChangedItem; tone: WhatChangedTone }) {
  const display = buildWhatChangedItemDisplay(item);
  const category = inferWhatChangedItemIconCategory(item);
  const badge = severityBadgeLabel(display.severity);

  return (
    <div className="relative w-full rounded-xl border border-white bg-white px-3.5 py-3 shadow-[0_1px_2px_rgb(6_27_69/0.04)]">
      <span
        className={cn("absolute inset-y-0 left-0 w-[3px] rounded-l-xl", getWhatChangedToneAccent(tone))}
        aria-hidden="true"
      />
      <div className="flex w-full items-start gap-2.5 pl-1">
        <span
          className={cn(
            "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg",
            tone === "negative" && "bg-[#fef3f2] text-[#b42318]",
            tone === "warning" && "bg-[#fffaeb] text-[#b54708]",
            tone === "neutral" && "bg-slate-100 text-slate-600",
            tone === "positive" && "bg-[#ecfdf3] text-[#027a48]",
            tone === "empty_positive" && "bg-slate-100 text-slate-500",
          )}
          aria-hidden="true"
        >
          <ItemCategoryIcon category={category} />
        </span>
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="flex min-w-0 flex-wrap items-start gap-2">
            <p className="min-w-0 flex-1 text-[14px] font-semibold leading-snug text-slate-900">
              {renderMetricHighlightText(display.title)}
            </p>
            {badge ? (
              <span className="shrink-0 rounded-md bg-slate-100 px-1.5 py-0.5 text-[11px] font-medium text-slate-700">
                {badge}
              </span>
            ) : null}
          </div>
          {display.subtitle ? (
            <p className="whitespace-normal break-words text-[13px] leading-[1.5] text-slate-700">
              {renderMetricHighlightText(display.subtitle)}
            </p>
          ) : null}
          {display.comparison ? (
            <p className="text-[12.5px] font-medium leading-snug text-slate-800">{display.comparison}</p>
          ) : null}
          {display.impact ? (
            <p className="whitespace-normal break-words text-[12.5px] leading-[1.45] text-slate-600">
              {renderMetricHighlightText(display.impact)}
            </p>
          ) : null}
          {display.entityLine ? (
            <p className="whitespace-normal break-words text-[12px] leading-snug text-slate-500">{display.entityLine}</p>
          ) : null}
          {display.actionHref ? (
            <Link
              href={display.actionHref}
              className="inline-flex items-center gap-1 pt-0.5 text-[12.5px] font-semibold text-[#2563eb] hover:underline"
            >
              {display.actionLabel ?? "Открыть действие"}
              <span aria-hidden="true">→</span>
            </Link>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ItemCategoryIcon({ category }: { category: string }) {
  const className = "h-4 w-4";
  switch (category) {
    case "advertising":
      return <Megaphone className={className} />;
    case "stock":
      return <Package className={className} />;
    case "funnel":
      return <BarChart3 className={className} />;
    case "cancellations":
      return <AlertCircle className={className} />;
    case "sales":
      return <TrendingDown className={className} />;
    default:
      return <AlertTriangle className={className} />;
  }
}
