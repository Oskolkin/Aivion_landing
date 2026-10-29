"use client";

import { AlertCircle } from "lucide-react";
import { DraftAttachmentCard } from "@/components/chat/draft-attachment-card";
import { formatAttachedFilesCount } from "@/lib/chat-display";
import type { ChatDraftAttachment } from "@/components/chat/chat-composer";

export function DraftAttachmentsSection({
  attachments,
  onRemove,
  onRetry,
  onRemoveAll,
  removingAll,
  disabled,
  selectionError,
  onDismissSelectionError,
}: {
  attachments: ChatDraftAttachment[];
  onRemove: (localId: string) => void;
  onRetry?: (localId: string) => void;
  onRemoveAll?: () => void;
  removingAll?: boolean;
  disabled?: boolean;
  selectionError?: string | null;
  onDismissSelectionError?: () => void;
}) {
  if (attachments.length === 0 && !selectionError) return null;

  return (
    <div className="mb-4">
      {attachments.length > 0 ? (
        <div className="mb-3 flex items-center justify-between gap-3">
          <h3 className="text-[15px] font-semibold text-[#0F172A]">
            {formatAttachedFilesCount(attachments.length)}
          </h3>
          {attachments.length >= 2 && onRemoveAll ? (
            <button
              type="button"
              disabled={disabled || removingAll}
              onClick={onRemoveAll}
              className="text-[13px] font-medium text-[#4B8FE8] transition-colors hover:text-aivion-blue focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-aivion-blue/40 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {removingAll ? "Удаление…" : "Удалить все"}
            </button>
          ) : null}
        </div>
      ) : null}

      {selectionError ? (
        <div
          role="alert"
          aria-live="polite"
          className="mb-3 flex items-start gap-2 rounded-[12px] border border-amber-200 bg-amber-50/90 px-3 py-2.5 text-[12px] leading-relaxed text-[#92400E]"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <p className="min-w-0 flex-1">{selectionError}</p>
          {onDismissSelectionError ? (
            <button
              type="button"
              className="shrink-0 text-[11px] font-medium text-[#B45309] hover:underline"
              onClick={onDismissSelectionError}
            >
              Закрыть
            </button>
          ) : null}
        </div>
      ) : null}

      {attachments.length > 0 ? (
        <div
          className="grid max-h-[232px] grid-cols-1 gap-2 overflow-y-auto pr-0.5 md:grid-cols-2 [scrollbar-width:thin] [scrollbar-color:#CBD5E1_transparent]"
          role="list"
          aria-label="Прикреплённые файлы"
        >
          {attachments.map((item) => (
            <div key={item.localId} role="listitem">
              <DraftAttachmentCard
                attachment={item}
                onRemove={onRemove}
                onRetry={onRetry}
                disabled={disabled || removingAll}
              />
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
