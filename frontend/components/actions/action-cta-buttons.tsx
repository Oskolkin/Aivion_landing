"use client";

import { useEffect, useRef, useState } from "react";
import { Check, RotateCcw } from "lucide-react";
import { ActionAskAIButtonWithRouter } from "@/components/actions/action-ask-ai-router-button";
import { Button, buttonClassNames } from "@/components/ui/button";
import type { ActionItem } from "@/lib/actions-types";
import { mapActionToDetail, type ActionDetailViewModel } from "@/lib/actions-view-model";

function DismissDialog({
  open,
  onClose,
  onConfirm,
  isMutating,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: (reason: string, comment: string) => void;
  isMutating: boolean;
}) {
  const [reason, setReason] = useState("уже_решено");
  const [comment, setComment] = useState("");
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    dialogRef.current?.querySelector<HTMLElement>("button, select, textarea")?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previouslyFocused?.focus();
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-4" role="presentation">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="dismiss-action-title"
        className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-5 shadow-xl"
      >
        <h3 id="dismiss-action-title" className="text-lg font-bold text-slate-950">
          Отметить действие неактуальным?
        </h3>
        <p className="mt-1 text-sm text-slate-600">
          Действие будет перемещено в раздел «Неактуальные» и перестанет учитываться среди активных
          рекомендаций.
        </p>
        <label className="mt-4 block text-sm font-medium text-slate-700">
          Причина
          <select
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            className="mt-1 h-10 w-full rounded-xl border border-slate-200 px-3 text-sm"
          >
            <option value="уже_решено">Проблема уже решена другим способом</option>
            <option value="ошибка_данных">Данные устарели</option>
            <option value="не_актуально">Рекомендация неприменима</option>
            <option value="отложено">Решение отложено</option>
            <option value="другое">Другое</option>
          </select>
        </label>
        <label className="mt-3 block text-sm font-medium text-slate-700">
          Комментарий (необязательно)
          <textarea
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            rows={3}
            className="mt-1 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm"
            placeholder="Кратко опишите контекст"
          />
        </label>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={isMutating}>
            Отмена
          </Button>
          <Button
            variant="primary"
            onClick={() => onConfirm(reason, comment)}
            disabled={isMutating}
            className="bg-slate-800 hover:bg-slate-900"
          >
            {isMutating ? "Обновление…" : "Отметить неактуальным"}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ActionStickyFooter({
  action,
  detail,
  isMutating,
  onToggleChecked,
  onMarkIrrelevant,
  onAskAI,
}: {
  action: ActionItem;
  detail: ActionDetailViewModel;
  isMutating: boolean;
  onToggleChecked: () => void;
  onMarkIrrelevant: () => void;
  onAskAI?: () => Promise<void>;
}) {
  const [dismissOpen, setDismissOpen] = useState(false);
  const isOpen = action.status !== "checked" && action.status !== "dismissed";
  const isCompleted = action.status === "checked";

  return (
    <>
      <footer className="sticky bottom-0 z-20 border-t border-slate-200 bg-white px-5 py-3.5 shadow-[0_-6px_16px_rgba(15,23,42,0.04)] lg:px-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            {isOpen ? (
              onAskAI ? (
                <button
                  type="button"
                  className={buttonClassNames(
                    "secondary",
                    "inline-flex h-11 items-center justify-center px-4",
                  )}
                  onClick={() => void onAskAI()}
                  disabled={isMutating}
                >
                  Спросить AI
                </button>
              ) : (
                <ActionAskAIButtonWithRouter actionViewId={action.action_view_id ?? ""} disabled={isMutating} variant="secondary" />
              )
            ) : null}
            {isCompleted ? (
              <p className="flex items-center gap-1.5 text-sm font-semibold text-emerald-700">
                <Check className="h-4 w-4" aria-hidden="true" />
                Выполнено
                {detail.createdAtLabel ? ` · ${detail.createdAtLabel}` : ""}
              </p>
            ) : null}
            {action.status === "dismissed" ? (
              <p className="text-sm font-semibold text-slate-600">Неактуально</p>
            ) : null}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {isOpen ? (
              <>
                <Button
                  variant="secondary"
                  onClick={onToggleChecked}
                  disabled={isMutating}
                  className="inline-flex h-10 items-center gap-2"
                >
                  <Check className="h-4 w-4" aria-hidden="true" />
                  {isMutating ? "Обновление…" : "Отметить выполненным"}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => setDismissOpen(true)}
                  disabled={isMutating}
                  className="inline-flex h-10 items-center gap-2 border-red-200 text-red-700 hover:bg-red-50"
                >
                  Отметить неактуальным
                </Button>
              </>
            ) : (
              <Button
                variant="secondary"
                onClick={onToggleChecked}
                disabled={isMutating}
                className="inline-flex h-10 items-center gap-2"
              >
                <RotateCcw className="h-4 w-4" aria-hidden="true" />
                Вернуть в работу
              </Button>
            )}
          </div>
        </div>
      </footer>

      <DismissDialog
        open={dismissOpen}
        isMutating={isMutating}
        onClose={() => setDismissOpen(false)}
        onConfirm={() => {
          setDismissOpen(false);
          onMarkIrrelevant();
        }}
      />
    </>
  );
}

/** Legacy export kept for tests that import ActionCTAButtons. */
export function ActionCTAButtons({
  action,
  onToggleChecked,
  onMarkIrrelevant,
  isMutating,
  onAskAI,
}: {
  action: ActionItem;
  onToggleChecked: () => void;
  onMarkIrrelevant: () => void;
  isMutating: boolean;
  onAskAI?: () => Promise<void>;
}) {
  const detail = mapActionToDetail(action);
  return (
    <ActionStickyFooter
      action={action}
      detail={detail}
      isMutating={isMutating}
      onToggleChecked={onToggleChecked}
      onMarkIrrelevant={onMarkIrrelevant}
      onAskAI={onAskAI}
    />
  );
}

export const ACTION_ASK_MESSAGE =
  "Помоги разобраться с этим действием и составь пошаговый план выполнения.";
