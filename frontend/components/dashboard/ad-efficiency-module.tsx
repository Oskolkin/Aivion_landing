"use client";

import {
  Activity,
  ArrowDown,
  ArrowUp,
  Bot,
  CalendarDays,
  ChevronRight,
  Info,
  Minus,
} from "lucide-react";
import { AdEfficiencyLineChart } from "@/components/dashboard/ad-efficiency-line-chart";
import { AdvertisingMetricCard } from "@/components/dashboard/ad-efficiency-metric-card";
import { ProblemCampaignsPanel } from "@/components/dashboard/problem-campaigns-panel";
import { cn } from "@/components/ui/cn";
import { isWidgetRenderable } from "@/components/dashboard/widget-state";
import {
  AD_EFFICIENCY_DENOMINATOR_NOTE,
  AD_EFFICIENCY_PENDING_CHECKLIST,
  AD_EFFICIENCY_UNAVAILABLE_CHECKLIST,
  buildAdEfficiencyDisplay,
  formatDrrPercent,
  type AdEfficiencyWidget,
} from "@/lib/ad-efficiency-display";
import { asArray } from "@/lib/as-array";

type Props = {
  widget: AdEfficiencyWidget | null | undefined;
};

const MODULE_SHELL =
  "rounded-[24px] border border-slate-200 bg-white p-6 shadow-[0_8px_30px_rgba(15,23,42,0.04)]";

function DeltaIcon({ tone }: { tone: "warning" | "success" | "neutral" }) {
  if (tone === "warning") return <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />;
  if (tone === "success") return <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />;
  return <Minus className="h-3.5 w-3.5" aria-hidden="true" />;
}

function deltaToneClass(tone: "warning" | "success" | "neutral"): string {
  if (tone === "warning") return "text-red-600";
  if (tone === "success") return "text-emerald-700";
  return "text-slate-600";
}

function AdEfficiencyHeader() {
  return (
    <header className="space-y-1.5">
      <h2 className="text-2xl font-bold leading-tight text-slate-950 xl:text-[28px]">
        Реклама: эффективность кампаний
      </h2>
      <p
        className="mt-1.5 text-sm leading-6 text-slate-600 lg:text-[15px]"
        data-testid="ad-efficiency-denominator-note"
      >
        {AD_EFFICIENCY_DENOMINATOR_NOTE}
      </p>
    </header>
  );
}

function AdEfficiencyAIConclusion({
  title,
  body,
  kind,
}: {
  title: string | null;
  body: string;
  kind: "ai" | "pending" | "unavailable";
}) {
  return (
    <div
      className="rounded-[20px] border border-blue-200 bg-gradient-to-r from-blue-50/70 via-white to-blue-50/40 p-6"
      data-testid="ad-efficiency-ai-summary"
    >
      <div className="flex items-start gap-4">
        <div
          className="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-white text-blue-600 shadow-[0_4px_14px_rgba(37,99,235,0.18)]"
          aria-hidden="true"
        >
          <Bot className="h-6 w-6" />
        </div>
        <div className="min-w-0 flex-1 space-y-3">
          <p className="text-[12px] font-medium uppercase tracking-wide text-slate-500 sm:text-[13px]">
            AI-вывод
          </p>
          {title ? (
            <h3
              className="text-lg font-semibold leading-snug text-slate-950 sm:text-[19px]"
              data-testid="ad-efficiency-ai-headline"
            >
              {title}
            </h3>
          ) : null}
          <p
            className={cn(
              "whitespace-normal break-words [overflow-wrap:anywhere] text-[15px] leading-[1.65] sm:text-base",
              kind === "ai" ? "text-slate-800" : "text-slate-600",
            )}
            data-testid="ad-efficiency-ai-explanation"
          >
            {body}
          </p>
        </div>
      </div>
    </div>
  );
}

function PartialBanner({ message }: { message: string }) {
  return (
    <div
      className="flex items-start gap-2 rounded-xl border border-sky-200 bg-sky-50 px-3 py-2.5 text-sm leading-relaxed text-sky-950"
      data-widget-state-tone="info"
      role="status"
      data-testid="ad-efficiency-partial-banner"
    >
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-sky-600" aria-hidden="true" />
      <p>{message}</p>
    </div>
  );
}

function ShellMessage({ title, message }: { title?: string; message: string }) {
  return (
    <div className="rounded-[20px] border border-dashed border-slate-200 bg-slate-50/60 px-5 py-12 text-center">
      {title ? <p className="text-base font-semibold text-slate-800">{title}</p> : null}
      <p className={cn("text-sm leading-relaxed text-slate-600", title ? "mt-2" : "")}>{message}</p>
    </div>
  );
}

function AdvertisingPriorityChecks({
  checklist,
  checklistState,
  actionsHref,
}: {
  checklist: Array<{ title: string; text: string }>;
  checklistState: "items" | "pending" | "unavailable";
  actionsHref: string;
}) {
  return (
    <div
      className="rounded-[20px] border border-slate-200 bg-white p-5 shadow-[0_4px_18px_rgba(15,23,42,0.035)]"
      data-testid="ad-efficiency-checklist"
    >
      <h3 className="text-lg font-bold text-slate-950">Что проверить в первую очередь</h3>

      {checklistState === "items" ? (
        <ol className="mt-5 space-y-5">
          {checklist.map((item, index) => (
            <li key={`${item.title}-${index}`} className="flex gap-3">
              <span
                className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white"
                aria-hidden="true"
              >
                {index + 1}
              </span>
              <div className="min-w-0">
                <p className="text-[15px] font-bold leading-snug text-slate-900 sm:text-base">{item.title}</p>
                {item.text ? (
                  <p className="mt-1 text-sm leading-[1.55] text-slate-600 sm:text-[15px]">{item.text}</p>
                ) : null}
              </div>
            </li>
          ))}
        </ol>
      ) : (
        <p className="mt-4 text-sm leading-[1.6] text-slate-600 sm:text-[15px]">
          {checklistState === "pending" ? AD_EFFICIENCY_PENDING_CHECKLIST : AD_EFFICIENCY_UNAVAILABLE_CHECKLIST}
        </p>
      )}

      <a
        href={actionsHref}
        className="mt-5 inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        data-testid="ad-efficiency-open-actions"
      >
        Открыть действия
        <ChevronRight className="h-4 w-4" aria-hidden="true" />
      </a>
    </div>
  );
}

function AdvertisingDataFootnote({ note }: { note: string }) {
  return (
    <div className="flex items-start gap-2 text-[13px] leading-[1.5] text-slate-500 sm:text-sm">
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
      <p data-testid="ad-efficiency-footer-note">{note}</p>
    </div>
  );
}

export function AdEfficiencySkeleton() {
  return (
    <div className={cn(MODULE_SHELL, "animate-pulse space-y-5")} data-testid="ad-efficiency-skeleton">
      <div className="space-y-2">
        <div className="h-8 w-2/3 rounded-lg bg-slate-100" />
        <div className="h-4 w-1/2 rounded bg-slate-100" />
      </div>
      <div className="grid items-start gap-4 md:grid-cols-2 lg:grid-cols-[minmax(210px,1fr)_minmax(210px,1fr)_minmax(0,1.8fr)]">
        <div className="h-40 rounded-[20px] bg-slate-100 md:col-span-1" />
        <div className="h-40 rounded-[20px] bg-slate-100 md:col-span-1" />
        <div className="h-40 rounded-[20px] bg-slate-100 lg:col-span-1 md:col-span-2 lg:row-start-auto" />
      </div>
      <div className="h-64 rounded-[20px] bg-slate-100" />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(360px,1fr)]">
        <div className="h-72 rounded-[20px] bg-slate-100" />
        <div className="h-72 rounded-[20px] bg-slate-100" />
      </div>
      <div className="h-4 w-2/3 rounded bg-slate-100" />
    </div>
  );
}

export function AdEfficiencyModule({ widget }: Props) {
  if (!widget) return null;

  const status = widget.status ?? "available";
  const display = buildAdEfficiencyDisplay(widget);
  const highDrrRows = asArray(widget.high_drr_campaigns);
  const noOrderRows = asArray(widget.no_order_campaigns);

  if (status === "not_enough_data") {
    return (
      <section className={MODULE_SHELL} data-testid="ad-efficiency-module">
        <AdEfficiencyHeader />
        <div className="mt-5">
          <ShellMessage
            message={
              widget.message?.trim() || "Рекламные кампании пока недоступны или не синхронизированы."
            }
          />
        </div>
      </section>
    );
  }

  if (!isWidgetRenderable(status)) {
    return (
      <section className={MODULE_SHELL} data-testid="ad-efficiency-module">
        <AdEfficiencyHeader />
        <div className="mt-5">
          <ShellMessage
            title="Не удалось загрузить данные по рекламе"
            message={widget.message?.trim() || "Повторите попытку позже"}
          />
        </div>
      </section>
    );
  }

  if (!display.hasBody) {
    return (
      <section className={MODULE_SHELL} data-testid="ad-efficiency-module">
        <AdEfficiencyHeader />
        <div className="mt-5 space-y-4">
          {display.partialMessage ? <PartialBanner message={display.partialMessage} /> : null}
          <ShellMessage message="Данные по рекламной эффективности пока недоступны." />
        </div>
      </section>
    );
  }

  const deltaSubtitle = display.deltaVsPrevious
    ? `${display.deltaVsPrevious} к ${display.previousCompletedDateLabel ?? "предыдущему дню"}`
    : null;

  return (
    <section className={cn(MODULE_SHELL, "space-y-5")} data-testid="ad-efficiency-module">
      <AdEfficiencyHeader />

      {display.partialMessage ? <PartialBanner message={display.partialMessage} /> : null}

      <div
        className="grid items-start gap-4 md:grid-cols-2 lg:grid-cols-[minmax(210px,1fr)_minmax(210px,1fr)_minmax(0,1.8fr)]"
        data-testid="ad-efficiency-overview-row"
      >
        <AdvertisingMetricCard
          testId="ad-efficiency-drr-card"
          label="ДРР ко вчера"
          value={display.currentDrr}
          valueTone="primary"
          infoTooltip={display.todayTooltip}
          supportingText={
            deltaSubtitle ? (
              <span className={cn("inline-flex items-center gap-1", deltaToneClass(display.deltaVsPreviousTone))}>
                <DeltaIcon tone={display.deltaVsPreviousTone} />
                {deltaSubtitle}
              </span>
            ) : (
              "Нет данных"
            )
          }
          icon={Activity}
        />

        <AdvertisingMetricCard
          testId="ad-efficiency-average-drr-card"
          label="Средний ДРР за 7 дней"
          value={formatDrrPercent(display.averageDrrPct)}
          supportingText={display.weeklyDelta ? `${display.weeklyDelta} к прошлой неделе` : "недостаточно данных"}
          infoTooltip={display.periodTooltip}
          icon={CalendarDays}
        />

        <div className="md:col-span-2 lg:col-span-1">
          <AdEfficiencyAIConclusion
            title={display.aiConclusion.title}
            body={display.aiConclusion.body}
            kind={display.aiConclusion.kind}
          />
        </div>
      </div>

      <AdEfficiencyLineChart
        points={display.trendPoints}
        hasTrendData={display.hasTrendData}
        averageDrrPct={display.averageDrrPct}
        sampleDays={display.sampleDays}
      />

      <div
        className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(360px,1fr)]"
        data-testid="ad-efficiency-bottom-grid"
      >
        <ProblemCampaignsPanel
          highDrrRows={highDrrRows}
          noOrderRows={noOrderRows}
          highDrrCount={display.highDrrCount}
          noOrdersCount={display.noOrdersCount}
          thresholdPct={display.highDrrThresholdPct}
        />

        <AdvertisingPriorityChecks
          checklist={display.checklist}
          checklistState={display.checklistState}
          actionsHref={display.actionsHref}
        />
      </div>

      <AdvertisingDataFootnote note={display.footerNote} />
    </section>
  );
}
