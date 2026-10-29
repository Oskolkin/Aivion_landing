"use client";

import { useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  CircleCheckBig,
  Lightbulb,
  TrendingUp,
} from "lucide-react";
import { ActionStickyFooter } from "@/components/actions/action-cta-buttons";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/components/ui/cn";
import { mapActionToDetail, type ActionMetricTone } from "@/lib/actions-view-model";
import type { ActionItem } from "@/lib/actions-types";

function situationToneClasses(tone: ActionMetricTone): string {
  switch (tone) {
    case "danger":
      return "border-red-100 bg-red-50/70";
    case "warning":
      return "border-amber-200 bg-amber-50/70";
    case "success":
      return "border-emerald-100 bg-emerald-50/50";
    case "info":
      return "border-sky-100 bg-sky-50/60";
    default:
      return "border-slate-200 bg-slate-50/70";
  }
}

function cellDisplay(value: string | undefined): string {
  if (!value || value === "—" || value.trim() === "") return "Нет данных";
  return value;
}

function DetailContentSkeleton() {
  return (
    <div className="space-y-5" aria-busy="true" aria-live="polite">
      <div className="h-20 animate-pulse rounded-[14px] bg-slate-100" />
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="h-40 animate-pulse rounded-[14px] bg-slate-100" />
        <div className="h-40 animate-pulse rounded-[14px] bg-slate-100" />
      </div>
    </div>
  );
}

function ActionPlanTimeline({
  steps,
}: {
  steps: ReturnType<typeof mapActionToDetail>["steps"];
}) {
  if (!steps.length) return null;
  return (
    <section>
      <h3 className="text-base font-bold text-slate-950">План действий</h3>
      <ol className="relative mt-4 space-y-0">
        {steps.map((step, index) => (
          <li key={step.id} className="relative flex gap-4 pb-6 last:pb-0">
            {index < steps.length - 1 ? (
              <span
                className="absolute left-[15px] top-8 bottom-0 w-px bg-slate-200"
                aria-hidden="true"
              />
            ) : null}
            <div className="relative z-10 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-aivion-blue text-sm font-bold text-white">
              {step.order}
            </div>
            <div className="min-w-0 flex-1 pt-0.5">
              <p className="text-[15px] font-semibold leading-snug text-slate-900">{step.title}</p>
              {step.description ? (
                <p className="mt-1 text-sm leading-relaxed text-slate-600">{step.description}</p>
              ) : null}
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

function AffectedEntitiesTable({
  entities,
}: {
  entities: NonNullable<ReturnType<typeof mapActionToDetail>["affectedEntities"]>;
}) {
  const [expanded, setExpanded] = useState(false);
  const limit = 5;
  const visible = expanded ? entities.rows : entities.rows.slice(0, limit);
  const hidden = Math.max(0, entities.rows.length - limit);

  const columnLabels =
    entities.type === "sku"
      ? [
          "SKU и товар",
          "Остаток",
          "Продажи за 7 дней",
          "Покрытие",
          "В пути",
          "Рекомендуется поставить",
          "Заказать до",
          "Приоритет",
        ]
      : entities.type === "campaign"
        ? ["Кампания", "Показатели", "Рекомендация"]
        : ["Позиция", "Показатели", "Рекомендация"];

  return (
    <section id="action-affected-entities">
      <h3 className="text-base font-bold text-slate-950">{entities.label}</h3>
      {entities.coverageNote ? (
        <p className="mt-1 text-sm text-slate-500">{entities.coverageNote}</p>
      ) : null}

      {entities.rows.length === 0 ? (
        <p className="mt-3 text-sm text-slate-600">Для всех позиций применяется общий план действий.</p>
      ) : (
        <>
          {hidden > 0 && !expanded ? (
            <p className="mt-1 text-xs text-slate-500">
              Показано {visible.length} из {entities.count}
            </p>
          ) : null}
          <div className="mt-3 hidden overflow-hidden rounded-[12px] border border-slate-200 md:block">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-xs font-semibold text-slate-600">
                  {columnLabels.map((label) => (
                    <th key={label} className="px-3.5 py-2.5 font-semibold">
                      {label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {visible.map((row) => {
                  const byCode = (code: string) =>
                    row.cells.find((cell) => {
                      const key = (cell.id || "").toLowerCase();
                      return key.includes(code.toLowerCase());
                    });
                  const stock =
                    byCode("available_stock") ||
                    row.cells.find((cell) => /^остаток$/i.test(cell.label));
                  const sales =
                    byCode("orders_7d") ||
                    row.cells.find((cell) => /продажи за 7|заказы 7/i.test(cell.label));
                  const coverage =
                    byCode("stock_coverage") ||
                    row.cells.find((cell) => /^покрытие$/i.test(cell.label));
                  const recommended =
                    byCode("recommended_supply") ||
                    row.cells.find((cell) => /рекомендуется поставить/i.test(cell.label));
                  const orderBy =
                    byCode("recommended_order") ||
                    row.cells.find((cell) => /заказать до/i.test(cell.label));
                  const inTransit =
                    byCode("in_transit") ||
                    row.cells.find((cell) => /^в пути$/i.test(cell.label));
                  return (
                    <tr key={row.id} className="border-b border-slate-100 align-top last:border-0">
                      <td className="px-3.5 py-3">
                        <p className="font-semibold text-slate-900">{row.primaryLabel}</p>
                        {row.secondaryLabel ? (
                          <p className="mt-0.5 text-xs text-slate-500">{row.secondaryLabel}</p>
                        ) : null}
                        {row.recommendation ? (
                          <p className="mt-1 text-xs font-medium text-slate-700">{row.recommendation}</p>
                        ) : null}
                      </td>
                      {entities.type === "sku" ? (
                        <>
                          <td
                            className={cn(
                              "px-3.5 py-3 tabular-nums",
                              stock?.tone === "danger" ? "font-semibold text-red-600" : "text-slate-800",
                            )}
                          >
                            {cellDisplay(stock?.value)}
                          </td>
                          <td className="px-3.5 py-3 tabular-nums text-slate-800">
                            {cellDisplay(sales?.value)}
                          </td>
                          <td className="px-3.5 py-3 tabular-nums text-slate-800">
                            {cellDisplay(coverage?.value)}
                          </td>
                          <td className="px-3.5 py-3 tabular-nums text-slate-800">
                            {cellDisplay(inTransit?.value ?? "0 шт.")}
                          </td>
                          <td className="px-3.5 py-3 tabular-nums text-slate-800">
                            {cellDisplay(recommended?.value)}
                          </td>
                          <td className="px-3.5 py-3 tabular-nums text-slate-800">
                            {cellDisplay(orderBy?.value)}
                          </td>
                          <td className="px-3.5 py-3">
                            {row.priorityLabel ? (
                              <span
                                className={cn(
                                  "inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold",
                                  row.priorityTone === "danger"
                                    ? "bg-red-50 text-red-700"
                                    : row.priorityTone === "warning"
                                      ? "bg-orange-50 text-orange-700"
                                      : "bg-slate-100 text-slate-700",
                                )}
                              >
                                {row.priorityLabel}
                              </span>
                            ) : (
                              "—"
                            )}
                          </td>
                        </>
                      ) : (
                        <>
                          <td className="px-3.5 py-3">
                            <div className="space-y-1">
                              {row.cells.map((cell) => (
                                <p key={cell.id} className="tabular-nums text-slate-800">
                                  <span className="text-slate-500">{cell.label}: </span>
                                  {cellDisplay(cell.value)}
                                </p>
                              ))}
                            </div>
                          </td>
                          <td className="px-3.5 py-3 text-slate-600">
                            {row.recommendation || "Нет данных"}
                          </td>
                        </>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="mt-3 space-y-2 md:hidden">
            {visible.map((row) => (
              <article key={row.id} className="rounded-[12px] border border-slate-200 bg-white p-3.5">
                <p className="font-semibold text-slate-900">{row.primaryLabel}</p>
                {row.secondaryLabel ? (
                  <p className="mt-0.5 text-xs text-slate-500">{row.secondaryLabel}</p>
                ) : null}
                {row.cells.length > 0 ? (
                  <dl className="mt-2 grid grid-cols-2 gap-2">
                    {row.cells.map((cell) => (
                      <div key={cell.id}>
                        <dt className="text-xs text-slate-500">{cell.label}</dt>
                        <dd
                          className={cn(
                            "text-sm font-medium tabular-nums",
                            cell.tone === "danger" ? "text-red-600" : "text-slate-800",
                          )}
                        >
                          {cellDisplay(cell.value)}
                        </dd>
                      </div>
                    ))}
                  </dl>
                ) : (
                  <p className="mt-2 text-sm text-slate-500">Нет данных</p>
                )}
              </article>
            ))}
          </div>

          {hidden > 0 ? (
            <button
              type="button"
              className="mt-2 inline-flex items-center gap-1 text-sm font-medium text-aivion-blue hover:underline"
              onClick={() => setExpanded((value) => !value)}
            >
              {expanded
                ? "Свернуть"
                : `Показать все ${entities.count} ${entities.type === "campaign" ? "кампаний" : "товаров"}`}
              <ChevronDown
                className={cn("h-4 w-4 transition-transform", expanded ? "rotate-180" : "")}
                aria-hidden="true"
              />
            </button>
          ) : null}
        </>
      )}
    </section>
  );
}

function ActionDetailBody({
  action,
  loading,
  mutationError,
}: {
  action: ActionItem;
  loading: boolean;
  mutationError: string;
}) {
  if (loading) return <DetailContentSkeleton />;
  const detail = mapActionToDetail(action);

  return (
    <div className="space-y-6">
      <div className="action-detail-primary-grid action-detail-primary-grid--split">
        <div className="space-y-4">
          {detail.situation ? (
            <section className={cn("rounded-[14px] border p-4", situationToneClasses(detail.situation.tone))}>
              <h3 className="flex items-center gap-2 text-sm font-bold text-slate-900">
                <Lightbulb className="h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" />
                {detail.situation.title}
              </h3>
              <p className="mt-2 text-[15px] leading-relaxed text-slate-800">{detail.situation.body}</p>
            </section>
          ) : null}

          {detail.importanceFacts.length > 0 || detail.importanceText ? (
            <section className="rounded-[14px] border border-sky-100 bg-sky-50/40 p-4">
              <h3 className="flex items-center gap-2 text-sm font-bold text-slate-900">
                <AlertCircle className="h-4 w-4 text-sky-600" aria-hidden="true" />
                Почему это важно
              </h3>
              {detail.importanceFacts.length > 0 ? (
                <ul className="mt-2 space-y-1.5">
                  {detail.importanceFacts.map((fact) => (
                    <li key={fact} className="text-[15px] leading-relaxed text-slate-700">
                      • {fact}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-[15px] leading-relaxed text-slate-700">{detail.importanceText}</p>
              )}
            </section>
          ) : null}

          <ActionPlanTimeline steps={detail.steps} />
        </div>

        <div className="space-y-4">
          {detail.affectedEntities ? <AffectedEntitiesTable entities={detail.affectedEntities} /> : null}

          <div className="grid gap-4">
            {detail.expectedResult ? (
              <section className="rounded-[14px] border border-emerald-100 bg-emerald-50/50 p-4">
                <h3 className="flex items-center gap-2 text-sm font-bold text-slate-900">
                  <TrendingUp className="h-4 w-4 text-emerald-600" aria-hidden="true" />
                  Ожидаемый результат
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-700">{detail.expectedResult}</p>
              </section>
            ) : null}

            {detail.completionCriteria.length > 0 ? (
              <section className="rounded-[14px] border border-blue-100 bg-blue-50/50 p-4">
                <h3 className="flex items-center gap-2 text-sm font-bold text-slate-900">
                  <CircleCheckBig className="h-4 w-4 text-aivion-blue" aria-hidden="true" />
                  Действие выполнено, когда
                </h3>
                <ul className="mt-2 space-y-2">
                  {detail.completionCriteria.map((criterion) => (
                    <li key={criterion.id} className="text-sm leading-relaxed text-slate-700">
                      <span className="inline-flex items-start gap-2">
                        <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-aivion-blue" aria-hidden="true" />
                        <span>{criterion.label}</span>
                      </span>
                      {criterion.technicalExplanation ? (
                        <details className="mt-1 pl-6">
                          <summary className="cursor-pointer text-xs font-medium text-slate-500 hover:text-slate-700">
                            Как это проверяется
                          </summary>
                          <p className="mt-1 text-xs text-slate-500">{criterion.technicalExplanation}</p>
                        </details>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
          </div>
        </div>
      </div>

      {mutationError ? <p className="text-sm text-aivion-danger">{mutationError}</p> : null}
    </div>
  );
}

export function ActionDetailPanel({
  action,
  loading,
  notFound,
  mutationError,
  isMutating,
  onToggleChecked,
  onMarkIrrelevant,
  onBackToList,
  onAskAI,
}: {
  action: ActionItem | null;
  loading: boolean;
  notFound?: boolean;
  mutationError: string;
  isMutating: boolean;
  onToggleChecked: () => void;
  onMarkIrrelevant: () => void;
  onBackToList?: () => void;
  onAskAI?: () => Promise<void>;
}) {
  if (notFound) {
    return (
      <EmptyState
        title="Не удалось загрузить выбранное действие"
        message="Возможно, оно было удалено или ссылка устарела."
        action={
          onBackToList ? (
            <button type="button" className="text-sm font-semibold text-aivion-blue" onClick={onBackToList}>
              Назад к действиям
            </button>
          ) : undefined
        }
        className="flex min-h-[320px] flex-col justify-center rounded-[20px] border border-slate-200 bg-white"
      />
    );
  }

  if (!action) {
    return (
      <EmptyState
        title="Выберите действие слева"
        message="Здесь появится план, затронутые позиции и критерий выполнения."
        className="flex min-h-[320px] flex-col justify-center rounded-[20px] border border-slate-200 bg-white"
      />
    );
  }

  const detail = mapActionToDetail(action);

  return (
    <article className="action-detail-container flex min-h-0 flex-1 flex-col overflow-hidden rounded-[20px] border border-slate-200 bg-white shadow-[0_2px_12px_rgba(15,23,42,0.05)]">
      {onBackToList ? (
        <div className="border-b border-slate-200 px-4 py-2.5 xl:hidden">
          <button type="button" onClick={onBackToList} className="text-sm font-semibold text-aivion-blue">
            ← Назад к действиям
          </button>
        </div>
      ) : null}

      <header className="border-b border-slate-200 px-5 py-4 lg:px-6">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5 text-xs">
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 font-semibold ring-1 ring-inset",
                detail.priority.tone.bg,
                detail.priority.tone.text,
                detail.priority.tone.ring,
              )}
            >
              {detail.priority.label}
            </span>
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 font-semibold ring-1 ring-inset",
                detail.category.tone.bg,
                detail.category.tone.text,
                detail.category.tone.ring,
              )}
            >
              {detail.category.label}
            </span>
            <span
              className={cn(
                "inline-flex rounded-full px-2 py-0.5 font-semibold ring-1 ring-inset",
                detail.status.tone.bg,
                detail.status.tone.text,
                detail.status.tone.ring,
              )}
            >
              {detail.status.label}
            </span>
            {detail.createdAtLabel ? (
              <span className="text-slate-500">Сформировано {detail.createdAtLabel}</span>
            ) : null}
          </div>
          <h2 className="mt-2 text-[24px] font-bold leading-[1.25] text-slate-950 sm:text-[26px]">
            {detail.title}
          </h2>
          {detail.summary ? (
            <p className="mt-1.5 max-w-3xl text-[15px] leading-relaxed text-slate-600 line-clamp-2">
              {detail.summary}
            </p>
          ) : null}
        </div>
      </header>

      <div className="actions-scroll min-h-0 flex-1 overflow-y-auto px-5 py-5 pb-8 lg:px-6">
        <ActionDetailBody
          key={action.action_key}
          action={action}
          loading={loading}
          mutationError={mutationError}
        />
      </div>

      <ActionStickyFooter
        key={action.action_key}
        action={action}
        detail={detail}
        isMutating={isMutating || loading}
        onToggleChecked={onToggleChecked}
        onMarkIrrelevant={onMarkIrrelevant}
        onAskAI={onAskAI}
      />
    </article>
  );
}
