import {
  COMPARISON_BAR_GRID_CLASS,
  comparisonBarHalfWidthPct,
  divergingBarClass,
  partitionComparisonItems,
  scaleDivergingComparisonBars,
  toneTextClass,
  type ComparisonBarItem,
} from "@/lib/funnel-comparison-bars";
import { cn } from "@/components/ui/cn";
import { InfoNote } from "@/components/ui/info-note";
import { EmptyNote } from "@/components/ui/empty-note";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  title: string;
  commentary?: string;
  items: ComparisonBarItem[];
  groups?: ComparisonBarsGroup[];
  note?: string;
  emptyText?: string;
};

export type ComparisonBarsGroup = {
  heading: string;
  subtitle?: string;
  items: ComparisonBarItem[];
};

type ScaledComparisonBarItem = ComparisonBarItem & { barPct: number; signedBarPct: number };

function ComparisonBarTrack({ item }: { item: ScaledComparisonBarItem }) {
  const halfWidth = comparisonBarHalfWidthPct(item.signedBarPct);
  return (
    <div
      className="relative h-2.5 w-full"
      data-testid="funnel-chart-area"
      role="img"
      aria-hidden={!item.delta && !item.value}
    >
      <div className="absolute inset-0 rounded-full bg-slate-100" />
      <div
        className="pointer-events-none absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-slate-400/45"
        data-testid="funnel-zero-axis"
      />
      {item.signedBarPct < 0 ? (
        <div
          className={cn(
            "absolute right-1/2 top-1/2 h-1.5 -translate-y-1/2 rounded-l-full",
            divergingBarClass(item.signedBarPct, item.tone),
          )}
          data-testid="funnel-bar-negative"
          style={{ width: `${halfWidth}%` }}
        />
      ) : null}
      {item.signedBarPct > 0 ? (
        <div
          className={cn(
            "absolute left-1/2 top-1/2 h-1.5 -translate-y-1/2 rounded-r-full",
            divergingBarClass(item.signedBarPct, item.tone),
          )}
          data-testid="funnel-bar-positive"
          style={{ width: `${halfWidth}%` }}
        />
      ) : null}
      {item.signedBarPct === 0 ? (
        <div
          className={cn(
            "absolute left-1/2 top-1/2 h-1.5 w-1 -translate-x-1/2 -translate-y-1/2 rounded-full",
            divergingBarClass(item.signedBarPct, item.tone),
          )}
          data-testid="funnel-bar-neutral"
        />
      ) : null}
    </div>
  );
}

function ComparisonBarValue({ item }: { item: ScaledComparisonBarItem }) {
  return (
    <span
      className={cn(
        "min-w-[4.75rem] shrink-0 text-right font-semibold tabular-nums",
        dashboardTypography.secondary,
        toneTextClass(item.tone),
        !item.delta && !item.value && "font-normal text-slate-500",
      )}
      data-testid="funnel-comparison-value"
    >
      {item.delta || item.value || "—"}
    </span>
  );
}

function ComparisonBarList({ items }: { items: ScaledComparisonBarItem[] }) {
  if (items.length === 0) return null;
  return (
    <ul className={cn("space-y-1", COMPARISON_BAR_GRID_CLASS)} data-testid="funnel-comparison-bar-grid">
      {items.map((item) => (
        <li
          key={item.key}
          className="flex flex-col gap-1 rounded-md border border-slate-200/90 bg-white px-2 py-1 sm:contents sm:border-0 sm:bg-transparent sm:p-0"
        >
          <span
            className={cn("min-w-0 whitespace-normal break-words sm:py-0.5", dashboardTypography.body, "text-slate-800")}
            title={item.titleLabel ?? item.label}
          >
            {item.label}
          </span>
          <ComparisonBarTrack item={item} />
          <ComparisonBarValue item={item} />
        </li>
      ))}
    </ul>
  );
}

function ComparisonGroup({
  heading,
  subtitle,
  items,
}: {
  heading: string;
  subtitle?: string;
  items: ComparisonBarItem[];
}) {
  const scaled = scaleDivergingComparisonBars(items);
  if (scaled.length === 0) return null;
  return (
    <div>
      <p className={dashboardTypography.aiSummaryLabel}>{heading}</p>
      {subtitle ? <p className={dashboardTypography.metadata}>{subtitle}</p> : null}
      <div className="mt-1">
        <ComparisonBarList items={scaled} />
      </div>
    </div>
  );
}

function HistoryNote({ children }: { children: string }) {
  return (
    <InfoNote variant="info" className="mt-1.5">
      {children}
    </InfoNote>
  );
}

function EmptyComparison({ children }: { children: string }) {
  return (
    <EmptyNote variant="dashed" className="mt-2">
      {children}
    </EmptyNote>
  );
}

function MetricsUnavailableNote() {
  return <InfoNote variant="neutral" className="mt-2">Детальные показатели сравнения пока недоступны.</InfoNote>;
}

function hasComparisonContent(
  groups: ComparisonBarsGroup[] | undefined,
  items: ComparisonBarItem[],
): boolean {
  if (groups && groups.length > 0) {
    return groups.some((g) => g.items.length > 0);
  }
  const { stages, conversions } = partitionComparisonItems(items);
  return stages.length > 0 || conversions.length > 0;
}

export function FunnelComparisonBars({ title, commentary, items, groups, note, emptyText = "Нет данных для сравнения." }: Props) {
  const hasCommentary = Boolean(commentary?.trim());
  const hasNote = Boolean(note?.trim());
  const hasMetrics = hasComparisonContent(groups, items);
  const sectionHasContent = hasCommentary || hasNote || hasMetrics;

  if (groups && groups.length > 0) {
    const nonEmpty = groups.filter((g) => g.items.length > 0);
    return (
      <div className="h-full rounded-lg border border-slate-200 bg-slate-50/80 p-3">
        <p className={dashboardTypography.sectionTitle}>{title}</p>
        {commentary ? <p className={cn("mt-1", dashboardTypography.body)}>{commentary}</p> : null}
        {note ? <HistoryNote>{note}</HistoryNote> : null}
        {nonEmpty.length === 0 ? (
          sectionHasContent && (hasCommentary || hasNote) ? (
            <MetricsUnavailableNote />
          ) : (
            <EmptyComparison>{emptyText}</EmptyComparison>
          )
        ) : (
          <div className="mt-2 space-y-2">
            {nonEmpty.map((g) => (
              <ComparisonGroup key={g.heading} heading={g.heading} subtitle={g.subtitle} items={g.items} />
            ))}
          </div>
        )}
      </div>
    );
  }
  const { stages, conversions } = partitionComparisonItems(items);
  const hasStages = stages.length > 0;
  const hasConversions = conversions.length > 0;

  return (
    <div className="h-full rounded-lg border border-slate-200 bg-slate-50/80 p-3">
      <p className={dashboardTypography.sectionTitle}>{title}</p>
      {commentary ? <p className={cn("mt-1", dashboardTypography.body)}>{commentary}</p> : null}
      {note ? <HistoryNote>{note}</HistoryNote> : null}
      {!hasStages && !hasConversions ? (
        sectionHasContent && (hasCommentary || hasNote) ? (
          <MetricsUnavailableNote />
        ) : (
          <EmptyComparison>{emptyText}</EmptyComparison>
        )
      ) : (
        <div className="mt-2 space-y-2">
          {hasStages && hasConversions ? (
            <>
              <ComparisonGroup heading="Этапы" items={stages} />
              <ComparisonGroup heading="Конверсии" items={conversions} />
            </>
          ) : (
            <ComparisonGroup heading={hasConversions ? "Конверсии" : "Этапы"} items={hasConversions ? conversions : stages} />
          )}
        </div>
      )}
    </div>
  );
}
