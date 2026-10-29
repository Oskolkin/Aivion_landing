import type { CSSProperties } from "react";
import type { LucideIcon } from "lucide-react";
import { Circle, Eye, FileText, Package, ShoppingCart, Truck } from "lucide-react";
import { cn } from "@/components/ui/cn";
import type { FunnelStage, WidgetStatus } from "@/lib/dashboard-home-types";
import {
  buildSalesFunnelDisplay,
  FUNNEL_WEAK_STAGE_SURFACE,
  type FunnelStageIconName,
} from "@/lib/sales-funnel-display";

type FunnelChartProps = {
  status?: WidgetStatus | string;
  stages: FunnelStage[];
  /** Ключ этапа (`toStage` weak transition) для подсветки в пирамиде. */
  highlightedStageKey?: string | null;
  highlightKind?: "weak_spot" | null;
  compact?: boolean;
};

const STAGE_ICONS: Record<FunnelStageIconName, LucideIcon> = {
  eye: Eye,
  "file-text": FileText,
  "shopping-cart": ShoppingCart,
  package: Package,
  truck: Truck,
  circle: Circle,
};

const deltaBadgeBase =
  "inline-flex max-w-full shrink-0 rounded-md px-2 py-0.5 text-[11px] font-semibold leading-tight tabular-nums ring-1 ring-inset sm:text-xs";

const deltaBadgeByTone = {
  positive: `${deltaBadgeBase} bg-emerald-500/20 text-emerald-50 ring-emerald-200/35`,
  negative: `${deltaBadgeBase} bg-orange-500/20 text-orange-50 ring-orange-200/40`,
  neutral: `${deltaBadgeBase} bg-white/15 text-white/90 ring-white/25`,
  unavailable: `${deltaBadgeBase} bg-white/10 text-white/75 ring-white/20`,
} as const;

const deltaBadgeOnLightByTone = {
  positive: `${deltaBadgeBase} bg-emerald-50 text-emerald-800 ring-emerald-200/80`,
  negative: `${deltaBadgeBase} bg-rose-100 text-rose-800 ring-rose-300/80`,
  neutral: `${deltaBadgeBase} bg-slate-100 text-slate-600 ring-slate-200/90`,
  unavailable: `${deltaBadgeBase} bg-slate-100 text-slate-500 ring-slate-200/90`,
} as const;

const COMPACT_STAGE_SURFACES = [
  "border-blue-300 bg-blue-50/90",
  "border-blue-200 bg-blue-50/60",
  "border-sky-200 bg-sky-50/70",
  "border-cyan-200 bg-cyan-50/70",
  "border-teal-200 bg-teal-50/70",
] as const;

function CompactFunnelChart({
  status,
  stages,
  highlightedStageKey,
  highlightKind,
}: Omit<FunnelChartProps, "compact">) {
  const displayStages = buildSalesFunnelDisplay(status, stages);
  if (displayStages.length === 0) return null;

  return (
    <div
      className="w-full min-w-0"
      role="img"
      aria-label="Воронка продаж: количество событий на каждом этапе и динамика к прошлому периоду"
      data-testid="funnel-compact-chart"
    >
      <ol className="space-y-1" aria-label="Этапы воронки продаж">
        {displayStages.map((stage, index) => {
          const isWeak =
            highlightKind === "weak_spot" &&
            highlightedStageKey != null &&
            stage.key === highlightedStageKey;
          const widthStyle = {
            "--funnel-stage-width": `${stage.widthPercent}%`,
          } as CSSProperties;

          return (
            <li key={stage.key} className="list-none">
              <div
                className={cn(
                  "relative mx-auto h-[58px] w-full overflow-hidden border transition-colors md:w-[var(--funnel-stage-width)]",
                  isWeak
                    ? "border-rose-300 bg-rose-50"
                    : COMPACT_STAGE_SURFACES[index % COMPACT_STAGE_SURFACES.length],
                )}
                style={{
                  ...widthStyle,
                  clipPath: "polygon(1.5% 0, 98.5% 0, 96.5% 100%, 3.5% 100%)",
                }}
                data-funnel-stage-highlight={isWeak ? "weak_spot" : undefined}
                aria-label={stage.accessibleSummary}
              >
                <div className="grid h-full grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 px-4 sm:px-5">
                  <div className="flex min-w-0 items-center gap-2">
                    <span
                      className={cn(
                        "inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[10px] font-bold",
                        isWeak ? "bg-rose-600 text-white" : "bg-aivion-blue text-white",
                      )}
                      aria-hidden
                    >
                      {index + 1}
                    </span>
                    <div className="min-w-0">
                      <p className="text-xs font-semibold leading-tight text-aivion-text sm:text-[13px]">
                        {stage.label}
                      </p>
                      {isWeak ? (
                        <span
                          className="mt-0.5 inline-flex rounded-full border border-rose-200 bg-white/80 px-1.5 py-px text-[9px] font-semibold text-rose-700"
                          data-funnel-weak-spot-pill="inline"
                        >
                          Узкое место
                        </span>
                      ) : null}
                    </div>
                  </div>
                  <p className="text-base font-bold tabular-nums tracking-tight text-aivion-text sm:text-lg">
                    {stage.valueLabel}
                  </p>
                  <p
                    className={cn(
                      "min-w-[4.2rem] text-right text-[11px] font-semibold tabular-nums sm:text-xs",
                      stage.previousDayChangeTone === "positive"
                        ? "text-emerald-700"
                        : stage.previousDayChangeTone === "negative"
                          ? "text-rose-600"
                          : "text-aivion-muted",
                    )}
                    title={stage.previousDayChangeText}
                  >
                    {stage.previousDayChangePrimary}
                  </p>
                </div>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

export function FunnelChart({ status, stages, highlightedStageKey, highlightKind, compact = false }: FunnelChartProps) {
  if (compact) {
    return (
      <CompactFunnelChart
        status={status}
        stages={stages}
        highlightedStageKey={highlightedStageKey}
        highlightKind={highlightKind}
      />
    );
  }
  const displayStages = buildSalesFunnelDisplay(status, stages);
  if (displayStages.length === 0) return null;

  const showIcons = displayStages.length <= 6;

  return (
    <div className="relative flex w-full max-w-full justify-center overflow-x-clip px-1 py-1 sm:px-2 xl:py-2 xl:pr-32">
      <div
        className="mx-auto flex w-full max-w-full flex-col items-center xl:max-w-[640px]"
        role="img"
        aria-label="Воронка продаж: количество событий на каждом этапе и конверсия с предыдущего шага"
      >
        <ol className="w-full space-y-1.5" aria-label="Этапы воронки продаж">
          {displayStages.map((stage) => {
            const Icon = STAGE_ICONS[stage.iconName] ?? Circle;
            const isWeakHighlight =
              highlightKind === "weak_spot" &&
              highlightedStageKey != null &&
              stage.key === highlightedStageKey;
            const surfaceClass = isWeakHighlight ? FUNNEL_WEAK_STAGE_SURFACE : stage.surfaceClassName;
            const deltaBadgeClass = isWeakHighlight
              ? deltaBadgeOnLightByTone[stage.previousDayChangeTone]
              : deltaBadgeByTone[stage.previousDayChangeTone];

            return (
              <li key={stage.key} className="list-none" aria-label={stage.accessibleSummary}>
                <div
                  className="relative mx-auto transition-all duration-300 ease-out"
                  style={{ width: `${stage.widthPercent}%` }}
                >
                  {/* Decorative trapezoid behind content so text is never clipped by clip-path */}
                  <div
                    className={cn(
                      "pointer-events-none absolute inset-0 shadow-[inset_0_1px_0_rgba(255,255,255,0.18),0_2px_8px_rgba(15,23,42,0.06)]",
                      surfaceClass,
                    )}
                    style={{ clipPath: "polygon(0% 0%, 100% 0%, 94% 100%, 6% 100%)" }}
                    aria-hidden="true"
                  />
                  {!isWeakHighlight ? (
                    <div
                      className="pointer-events-none absolute inset-x-[3%] top-0 h-[40%] bg-gradient-to-b from-white/14 to-transparent"
                      style={{ clipPath: "polygon(0% 0%, 100% 0%, 97% 100%, 3% 100%)" }}
                      aria-hidden="true"
                    />
                  ) : null}

                  <div
                    className="relative z-10 px-3 py-2.5 sm:px-4 sm:py-3"
                    data-funnel-stage-highlight={isWeakHighlight ? "weak_spot" : undefined}
                  >
                    <div className="flex flex-col items-center gap-1 text-center sm:gap-1.5">
                      <div className="flex flex-wrap items-center justify-center gap-1.5">
                        {showIcons ? (
                          <span
                            className={cn(
                              "inline-flex h-6 w-6 items-center justify-center rounded-full sm:h-7 sm:w-7",
                              isWeakHighlight ? "bg-rose-100/90" : "bg-white/15",
                            )}
                            aria-hidden="true"
                          >
                            <Icon
                              className={cn(
                                "h-3.5 w-3.5 sm:h-4 sm:w-4",
                                isWeakHighlight ? "text-rose-500" : "text-white/85",
                              )}
                            />
                          </span>
                        ) : null}
                        <p
                          className={cn(
                            "text-xs font-semibold leading-snug sm:text-[13px]",
                            isWeakHighlight ? "text-slate-700" : "text-white/92",
                          )}
                        >
                          {stage.label}
                        </p>
                        {isWeakHighlight ? (
                          <span
                            className="inline-flex items-center rounded-full bg-rose-600 px-2 py-0.5 text-[11px] font-semibold text-white sm:hidden"
                            data-funnel-weak-spot-pill="inline"
                          >
                            Слабое место
                          </span>
                        ) : null}
                      </div>

                      <p
                        className={cn(
                          "text-[22px] font-bold tabular-nums tracking-tight sm:text-[28px] lg:text-[30px]",
                          isWeakHighlight ? "text-slate-900" : "text-white",
                        )}
                      >
                        {stage.valueLabel}
                      </p>

                      <p
                        className={cn(
                          "text-[13px] tabular-nums sm:text-sm",
                          isWeakHighlight ? "text-slate-600" : "text-white/80",
                        )}
                        title={stage.conversionTitle ?? undefined}
                      >
                        {stage.conversionLabel}
                        {stage.conversionCaption ? (
                          <span className={isWeakHighlight ? "text-slate-500" : "text-white/65"}>
                            {" "}
                            {stage.conversionCaption}
                          </span>
                        ) : null}
                      </p>

                      <div
                        className={cn(deltaBadgeClass, "flex flex-col items-center gap-0.5 text-center")}
                        title={stage.previousDayChangeText}
                        aria-label={stage.previousDayChangeText}
                      >
                        <span className="tabular-nums">{stage.previousDayChangePrimary}</span>
                        {stage.previousDayChangeSecondary ? (
                          <span className="max-w-[14rem] text-[10px] font-medium leading-tight opacity-90 sm:text-[11px]">
                            {stage.previousDayChangeSecondary}
                          </span>
                        ) : null}
                      </div>
                    </div>
                  </div>

                  {isWeakHighlight ? (
                    <div
                      className="pointer-events-none absolute left-full top-1/2 z-20 ml-2 hidden -translate-y-1/2 items-center xl:flex"
                      data-funnel-weak-spot-pill="side"
                      aria-hidden="true"
                    >
                      <span className="mr-2 h-px w-5 border-t border-dashed border-rose-400" />
                      <span className="rounded-full bg-rose-600 px-2.5 py-1 text-[11px] font-semibold whitespace-nowrap text-white shadow-sm">
                        Слабое место
                      </span>
                    </div>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ol>
      </div>
    </div>
  );
}
