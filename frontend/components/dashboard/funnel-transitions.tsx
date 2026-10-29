import type { LucideIcon } from "lucide-react";
import {
  AlertTriangle,
  ArrowRightLeft,
  Circle,
  Eye,
  FileText,
  Package,
  ShoppingCart,
  Truck,
} from "lucide-react";
import type { FunnelTransitionDisplay, FunnelTransitionHighlightKind } from "@/lib/funnel-transitions-display";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import { getFunnelStageIconName, type FunnelStageIconName } from "@/lib/sales-funnel-display";

type FunnelTransitionsProps = {
  transitions: FunnelTransitionDisplay[];
  compact?: boolean;
  comparisonMode?: "default" | "day_over_day";
};

const STAGE_ICONS: Record<FunnelStageIconName, LucideIcon> = {
  eye: Eye,
  "file-text": FileText,
  "shopping-cart": ShoppingCart,
  package: Package,
  truck: Truck,
  circle: Circle,
};

function transitionIcon(toStageKey: string): LucideIcon {
  return STAGE_ICONS[getFunnelStageIconName(toStageKey)] ?? ArrowRightLeft;
}

function conversionDeltaClass(value: number | null | undefined): string {
  if (value == null) return "text-slate-500";
  if (value < 0) return "text-rose-700";
  if (value > 0) return "text-emerald-700";
  return "text-slate-500";
}

function countDeltaClass(value: number | null | undefined): string {
  if (value == null) return "text-slate-500";
  if (value < 0) return "text-rose-700";
  if (value > 0) return "text-emerald-700";
  return "text-slate-500";
}

function compactTransitionLabel(transition: FunnelTransitionDisplay): string {
  const key = transition.key.toLowerCase();
  if (key.includes("add_to_cart") && key.includes("orders")) return "Корзина → заказ";
  if (key.includes("orders") && key.includes("delivered")) return "Заказ → доставка";
  return transition.compactLabel;
}

function highlightBadge(
  kind: Exclude<FunnelTransitionHighlightKind, null>,
): { label: string; tone: "danger" | "neutral" } {
  if (kind === "weak_spot") {
    return { label: "Приоритет №1", tone: "danger" };
  }
  return { label: "Самый узкий переход", tone: "neutral" };
}

function CompactFunnelTransitions({
  transitions,
}: {
  transitions: FunnelTransitionDisplay[];
}) {
  if (transitions.length === 0) {
    return (
      <section aria-label="Конверсия и динамика к предыдущему дню">
        <h3 className={dashboardTypography.sectionTitle}>Конверсия и динамика к предыдущему дню</h3>
        <p className={cn("mt-2", dashboardTypography.emptyState)}>
          Переходы появятся, когда в воронке будет больше одного этапа.
        </p>
      </section>
    );
  }

  return (
    <section aria-label="Конверсия и динамика к предыдущему дню" data-testid="funnel-compact-losses">
      <header className="mb-2 flex items-end justify-between gap-3">
        <div>
          <h3 className={cn(dashboardTypography.sectionTitle, "text-base")}>Конверсия и динамика к предыдущему дню</h3>
        </div>
      </header>

      <div className="hidden grid-cols-[minmax(0,1.35fr)_minmax(5.5rem,.62fr)_minmax(6.5rem,.72fr)_minmax(6.5rem,.72fr)] gap-3 px-3 pb-1 text-[10px] font-medium uppercase tracking-wide text-aivion-muted lg:grid">
        <span>Переход</span>
        <span>Конверсия</span>
        <span>Изменение, п.п.</span>
        <span>Изменение количества</span>
      </div>

      <ul className="space-y-1.5">
        {transitions.map((transition) => {
          const isWeak = transition.highlightKind === "weak_spot";
          return (
            <li key={transition.key} className="list-none">
              <article
                className={cn(
                  "grid min-h-[62px] grid-cols-2 items-center gap-x-3 gap-y-2 rounded-[10px] border px-3 py-2 lg:grid-cols-[minmax(0,1.35fr)_minmax(5.5rem,.62fr)_minmax(6.5rem,.72fr)_minmax(6.5rem,.72fr)] lg:gap-y-0",
                  isWeak ? "border-rose-300 bg-rose-50/75" : "border-aivion-border bg-white",
                )}
                data-testid="funnel-loss-row"
                data-funnel-transition-highlight={transition.highlightKind ?? undefined}
                aria-label={`Переход ${transition.label}, конверсия ${transition.conversionLabel}`}
              >
                <div className="col-span-2 min-w-0 lg:col-span-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <p className="text-xs font-semibold leading-snug text-aivion-text sm:text-[13px]">
                      {compactTransitionLabel(transition)}
                    </p>
                    {isWeak ? (
                      <Badge tone="danger" className="px-1.5 py-px text-[9px]">
                        Узкое место
                      </Badge>
                    ) : null}
                  </div>
                  <p className="mt-0.5 text-[10px] tabular-nums text-aivion-muted sm:text-[11px]">
                    {transition.valuesSubline}
                  </p>
                </div>

                <div>
                  <p className="text-[10px] text-aivion-muted lg:hidden">Конверсия</p>
                  <p className={cn("text-sm font-bold tabular-nums", isWeak ? "text-rose-700" : "text-aivion-text")}>
                    {transition.conversionLabel}
                  </p>
                  <p className="hidden text-[10px] leading-tight text-aivion-muted 2xl:block">
                    {transition.conversionCaption}
                  </p>
                </div>

                <div>
                  <p className="text-[10px] text-aivion-muted lg:hidden">Изменение, п.п.</p>
                  <p className={cn("text-xs font-semibold tabular-nums", conversionDeltaClass(transition.conversionDeltaPp))}>
                    {transition.conversionDeltaLabel ?? "—"}
                  </p>
                </div>

                <div className="col-span-2 lg:col-span-1">
                  <p className="text-[10px] text-aivion-muted lg:hidden">Изменение количества</p>
                  <p
                    className={cn("text-xs font-semibold tabular-nums", countDeltaClass(transition.volumeDeltaPct))}
                    title={transition.volumeDeltaFullLabel}
                  >
                    {transition.volumeDeltaPrimary ?? "—"}
                  </p>
                </div>
              </article>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

export function FunnelTransitions({ transitions, compact = false, comparisonMode = "default" }: FunnelTransitionsProps) {
  if (compact) return <CompactFunnelTransitions transitions={transitions} />;
  if (transitions.length === 0) {
    return (
      <section aria-label="Где теряется воронка">
        <header className="mb-3 space-y-1">
          <h3 className={cn(dashboardTypography.sectionTitle, "text-lg lg:text-xl")}>Где теряется воронка</h3>
          <p className={dashboardTypography.sectionDescription}>
            Появятся, когда в воронке будет больше одного этапа.
          </p>
        </header>
      </section>
    );
  }

  return (
    <section aria-label="Где теряется воронка">
      <header className="mb-3 space-y-1">
        <h3 className={cn(dashboardTypography.sectionTitle, "text-lg lg:text-xl")}>Где теряется воронка</h3>
        <p className={dashboardTypography.sectionDescription}>Конверсия, динамика и приоритет анализа</p>
      </header>
      <ul className="space-y-2">
        {transitions.map((transition) => {
          const isWeak = transition.highlightKind === "weak_spot";
          const badge = transition.highlightKind ? highlightBadge(transition.highlightKind) : null;
          const Icon = isWeak ? AlertTriangle : transitionIcon(transition.toStageKey);

          return (
            <li key={transition.key} className="list-none">
              <article
                className={cn(
                  "rounded-xl border px-3 py-3 transition-colors sm:px-3.5",
                  isWeak
                    ? "border-rose-300 bg-rose-50/80"
                    : transition.highlightKind === "bottleneck"
                      ? "border-sky-200 bg-sky-50/50"
                      : "border-slate-200/90 bg-white",
                )}
                aria-label={`Переход ${transition.label}, конверсия ${transition.conversionLabel}`}
                data-funnel-transition-highlight={transition.highlightKind ?? undefined}
              >
                <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 sm:grid-cols-[auto_minmax(0,1fr)_minmax(8.5rem,auto)] sm:items-start">
                  <span
                    className={cn(
                      "mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full",
                      isWeak ? "bg-rose-100 text-rose-600" : "bg-slate-100 text-slate-500",
                    )}
                    aria-hidden="true"
                  >
                    <Icon className="h-4 w-4" />
                  </span>

                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <p className={cn("break-words", dashboardTypography.listItemTitle)}>{transition.compactLabel}</p>
                      {badge ? (
                        <Badge tone={badge.tone} className={cn("shrink-0", dashboardTypography.badge)}>
                          {badge.label}
                        </Badge>
                      ) : null}
                    </div>
                    <p className={cn("mt-0.5 tabular-nums", dashboardTypography.secondary)}>
                      {transition.valuesSubline}
                    </p>

                    {/* Mobile: textual breakdown under the absolute values */}
                    <div className="mt-2 space-y-1.5 sm:hidden">
                      <p className={cn(dashboardTypography.metadata)}>
                        <span className="font-semibold text-slate-800">{transition.conversionLabel}</span>
                        {" · "}
                        {transition.conversionCaption}
                      </p>
                      {transition.conversionDeltaLabel ? (
                        <p className={cn(dashboardTypography.metadata, conversionDeltaClass(transition.conversionDeltaPp))}>
                          <span className="font-semibold">{transition.conversionDeltaLabel}</span>
                          {transition.conversionDeltaSecondary ? ` · ${transition.conversionDeltaSecondary}` : null}
                        </p>
                      ) : null}
                      {transition.volumeDeltaPrimary ? (
                        <p
                          className={cn(dashboardTypography.metadata, countDeltaClass(transition.volumeDeltaPct))}
                          title={transition.volumeDeltaFullLabel}
                        >
                          <span className="font-semibold">{transition.volumeDeltaPrimary}</span>
                          {transition.volumeDeltaSecondary ? ` · ${transition.volumeDeltaSecondary}` : null}
                        </p>
                      ) : null}
                    </div>
                  </div>

                  <div className="col-span-2 hidden min-w-0 space-y-1.5 sm:col-span-1 sm:block sm:text-right">
                    <div>
                      <p
                        className={cn(
                          "text-xl font-bold tabular-nums tracking-tight sm:text-2xl",
                          isWeak ? "text-rose-700" : "text-slate-950",
                        )}
                      >
                        {transition.conversionLabel}
                      </p>
                      <p className={cn(dashboardTypography.metadata, "text-slate-500")}>{transition.conversionCaption}</p>
                    </div>
                    {transition.conversionDeltaLabel ? (
                      <div>
                        <p
                          className={cn(
                            "font-semibold tabular-nums",
                            dashboardTypography.kpiDelta,
                            conversionDeltaClass(transition.conversionDeltaPp),
                          )}
                        >
                          {transition.conversionDeltaLabel}
                        </p>
                        {transition.conversionDeltaSecondary ? (
                          <p className={cn(dashboardTypography.metadata, "text-slate-500")}>
                            {transition.conversionDeltaSecondary}
                          </p>
                        ) : null}
                      </div>
                    ) : null}
                    {transition.volumeDeltaPrimary ? (
                      <div title={transition.volumeDeltaFullLabel}>
                        <p
                          className={cn(
                            "font-semibold tabular-nums",
                            dashboardTypography.kpiDelta,
                            countDeltaClass(transition.volumeDeltaPct),
                          )}
                        >
                          {transition.volumeDeltaPrimary}
                        </p>
                        {transition.volumeDeltaSecondary ? (
                          <p className={cn(dashboardTypography.metadata, "text-slate-500")}>
                            {transition.volumeDeltaSecondary}
                          </p>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </div>
              </article>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
