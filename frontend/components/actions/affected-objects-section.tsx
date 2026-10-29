"use client";

import { ChevronDown, Target } from "lucide-react";
import { cn } from "@/components/ui/cn";
import {
  COMPACT_OBJECT_LIMIT,
  hiddenObjectCount,
} from "@/lib/actions-detail-layout";
import {
  formatAffectedObjectsDetailCaption,
  formatAffectedObjectsHeader,
  formatActionScopeLevelLabel,
  formatMissingObjectActionsMessage,
  normalizeAffectedObjects,
  type ActionAffectedObjectDisplay,
} from "@/lib/actions-display";
import type { OperationalObjectAction } from "@/lib/actions-types";

export type AffectedObjectsCountProps = {
  total: number;
  displayed: number;
  truncated: boolean;
  entityType: string;
  scopeType?: "account" | "entity" | "mixed" | "unknown";
  coverageMode?: string;
  coverageReason?: string;
};

function ObjectsSectionHeader({ counts }: { counts: AffectedObjectsCountProps }) {
  if (counts.scopeType === "account") {
    return (
      <div className="flex items-start justify-between gap-3">
        <h3 className="flex items-center gap-2 text-base font-bold text-slate-900">
          <Target className="h-4 w-4 text-slate-500" aria-hidden="true" />
          Уровень действия
        </h3>
        <p className="text-sm font-medium text-slate-700">Весь кабинет</p>
      </div>
    );
  }

  const headerLabel = counts.scopeType === "mixed" ? "Затронутые объекты" : "Что именно затронуто";
  const header = formatAffectedObjectsHeader(counts.total, counts.entityType);
  const caption = formatAffectedObjectsDetailCaption(
    counts.total,
    counts.displayed,
    counts.truncated,
    counts.entityType,
    counts.coverageMode,
  );

  return (
    <div className="flex items-start justify-between gap-3">
      <h3 className="flex items-center gap-2 text-base font-bold text-slate-900">
        <Target className="h-4 w-4 text-slate-500" aria-hidden="true" />
        {headerLabel}
      </h3>
      <div className="text-right">
        {header ? <p className="text-sm font-medium text-slate-700">{header}</p> : null}
        {caption ? <p className="text-xs text-slate-500">{caption}</p> : null}
        {counts.coverageReason ? (
          <p className="mt-1 text-xs text-slate-500">{counts.coverageReason}</p>
        ) : null}
      </div>
    </div>
  );
}

function CompactObjectCard({ object }: { object: ActionAffectedObjectDisplay }) {
  const recommendation = object.compactRecommendation ?? object.recommendedAction;

  return (
    <article className="rounded-[12px] border border-slate-200 bg-white p-3.5">
      <header>
        <p className="text-[15px] font-semibold text-slate-900">{object.primaryLabel}</p>
        {object.secondaryLabel ? (
          <p className="mt-0.5 text-xs text-slate-500">{object.secondaryLabel}</p>
        ) : null}
      </header>

      {object.metrics.length > 0 ? (
        <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-3">
          {object.metrics.map((metric) => (
            <div key={metric.key} className="min-w-0">
              <dt className="text-xs text-slate-500">{metric.label}</dt>
              <dd className="mt-0.5 text-sm font-medium tabular-nums text-slate-800">
                {metric.formattedValue}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}

      {object.reason ? (
        <p className="mt-2 text-[13px] leading-relaxed text-slate-500 line-clamp-3">{object.reason}</p>
      ) : null}

      {recommendation ? (
        <p className="mt-3 text-[13px] leading-relaxed text-slate-600 line-clamp-4">{recommendation}</p>
      ) : null}
    </article>
  );
}

function ExpandedObjectCard({ object }: { object: ActionAffectedObjectDisplay }) {
  return (
    <article className="rounded-[12px] border border-slate-200 bg-white p-4">
      <p className="text-[15px] font-semibold text-slate-900">{object.primaryLabel}</p>
      {object.secondaryLabel ? (
        <p className="mt-0.5 text-xs text-slate-500">{object.secondaryLabel}</p>
      ) : null}
      {object.metrics.length > 0 ? (
        <dl className="mt-3 space-y-1.5">
          {object.metrics.map((metric) => (
            <div key={metric.key} className="flex justify-between gap-3 text-sm">
              <dt className="text-slate-600">{metric.label}</dt>
              <dd className="font-medium tabular-nums text-slate-800">{metric.formattedValue}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {object.recommendedAction ? (
        <p className="mt-3 text-sm leading-relaxed text-slate-600">
          <span className="font-medium text-slate-700">Что сделать: </span>
          {object.recommendedAction}
        </p>
      ) : null}
      {object.reason ? (
        <p className="mt-2 text-sm leading-relaxed text-slate-500">
          <span className="font-medium text-slate-600">Почему: </span>
          {object.reason}
        </p>
      ) : null}
    </article>
  );
}

function ExpandedObjectTable({ objects }: { objects: ActionAffectedObjectDisplay[] }) {
  return (
    <div className="affected-objects-expanded-table rounded-[12px] border border-slate-200">
      <table className="w-full table-fixed text-left text-sm">
        <thead>
          <tr className="border-b border-slate-200 bg-slate-50 text-xs font-semibold text-slate-600">
            <th className="w-[34%] px-4 py-2.5">Объект</th>
            <th className="w-[33%] px-4 py-2.5">Показатели</th>
            <th className="w-[33%] px-4 py-2.5">Что сделать</th>
          </tr>
        </thead>
        <tbody>
          {objects.map((object) => (
            <tr key={object.key} className="border-b border-slate-100 align-top">
              <td className="px-4 py-3">
                <p className="font-semibold text-slate-900">{object.primaryLabel}</p>
                {object.secondaryLabel ? (
                  <p className="mt-0.5 text-xs text-slate-500">{object.secondaryLabel}</p>
                ) : null}
              </td>
              <td className="px-4 py-3">
                <div className="space-y-1">
                  {object.metrics.map((metric) => (
                    <p key={metric.key} className="text-sm tabular-nums text-slate-800">
                      <span className="text-slate-600">{metric.label}: </span>
                      {metric.formattedValue}
                    </p>
                  ))}
                </div>
              </td>
              <td className="px-4 py-3 text-sm leading-relaxed text-slate-600">
                {object.recommendedAction || "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ExpandToggle({
  expanded,
  hiddenCount,
  onToggle,
}: {
  expanded: boolean;
  hiddenCount: number;
  onToggle: () => void;
}) {
  if (hiddenCount <= 0 && !expanded) return null;

  return (
    <button
      type="button"
      onClick={onToggle}
      className="mt-2 inline-flex items-center gap-1 text-sm font-medium text-aivion-blue hover:underline"
    >
      {expanded ? "Свернуть" : `Показать ещё ${hiddenCount}`}
      <ChevronDown
        className={cn("h-4 w-4 transition-transform", expanded ? "rotate-180" : "")}
        aria-hidden="true"
      />
    </button>
  );
}

export function AffectedObjectsEmpty({
  counts,
  affectedObjectsSummary,
}: {
  counts?: AffectedObjectsCountProps;
  affectedObjectsSummary?: string;
}) {
  if (counts?.scopeType === "account") {
    return (
      <section>
        <ObjectsSectionHeader counts={counts} />
      </section>
    );
  }

  if (counts && counts.total > 0) {
    return (
      <section>
        <ObjectsSectionHeader counts={{ ...counts, displayed: counts.displayed, truncated: counts.total > 0 && counts.displayed < counts.total }} />
        <p className="mt-3 text-sm leading-relaxed text-slate-600">{formatMissingObjectActionsMessage()}</p>
        {counts.coverageReason ? (
          <p className="mt-2 text-sm leading-relaxed text-slate-500">{counts.coverageReason}</p>
        ) : null}
      </section>
    );
  }

  let message = "Действие относится ко всему кабинету";
  if (affectedObjectsSummary?.trim()) {
    message = affectedObjectsSummary.trim();
  }

  return (
    <section>
      <p className="mt-3 text-sm leading-relaxed text-slate-600">{message}</p>
    </section>
  );
}

export function AffectedObjectsCompact({
  objects,
  counts,
  onExpand,
}: {
  objects: OperationalObjectAction[];
  counts: AffectedObjectsCountProps;
  onExpand: () => void;
}) {
  if (!objects.length && counts.total <= 0) return <AffectedObjectsEmpty counts={counts} />;

  const normalized = normalizeAffectedObjects(objects);
  const hidden = hiddenObjectCount(normalized.length, COMPACT_OBJECT_LIMIT);
  const visible = normalized.slice(0, COMPACT_OBJECT_LIMIT);

  return (
    <section>
      <ObjectsSectionHeader counts={counts} />
      <div className="mt-3 space-y-2.5">
        {visible.map((object) => (
          <CompactObjectCard key={object.key} object={object} />
        ))}
      </div>
      <ExpandToggle expanded={false} hiddenCount={hidden} onToggle={onExpand} />
    </section>
  );
}

export function AffectedObjectsExpanded({
  objects,
  counts,
  onCollapse,
}: {
  objects: OperationalObjectAction[];
  counts: AffectedObjectsCountProps;
  onCollapse: () => void;
}) {
  const normalized = normalizeAffectedObjects(objects);

  return (
    <section className="space-y-3">
      <ObjectsSectionHeader counts={counts} />
      <div className="affected-objects-expanded-grid affected-objects-expanded-grid--table-mode">
        {normalized.map((object) => (
          <ExpandedObjectCard key={object.key} object={object} />
        ))}
      </div>
      <ExpandedObjectTable objects={normalized} />
      <ExpandToggle expanded hiddenCount={0} onToggle={onCollapse} />
    </section>
  );
}

/** @deprecated Use AffectedObjectsCompact / AffectedObjectsExpanded */
export function ObjectActionsTable({
  objects,
  counts,
  expanded,
  onToggleExpanded,
}: {
  objects: OperationalObjectAction[];
  counts?: AffectedObjectsCountProps;
  expanded?: boolean;
  onToggleExpanded?: (expanded: boolean) => void;
}) {
  const resolvedCounts = counts ?? {
    total: objects.length,
    displayed: objects.length,
    truncated: false,
    entityType: objects[0]?.entity_type ?? "sku",
  };
  if (expanded) {
    return (
      <AffectedObjectsExpanded
        objects={objects}
        counts={resolvedCounts}
        onCollapse={() => onToggleExpanded?.(false)}
      />
    );
  }
  return (
    <AffectedObjectsCompact
      objects={objects}
      counts={resolvedCounts}
      onExpand={() => onToggleExpanded?.(true)}
    />
  );
}
