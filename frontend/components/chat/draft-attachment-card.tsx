"use client";

import { AlertTriangle, Loader2, X } from "lucide-react";
import { AttachmentTypeIcon } from "@/components/chat/attachment-type-icon";
import { cn } from "@/components/ui/cn";
import {
  formatFileSize,
  getAttachmentDisplayName,
  getAttachmentVisualKind,
  getAttachmentWarningLabel,
  getAttachmentWarningTooltip,
} from "@/lib/chat-display";
import type { ChatDraftAttachment } from "@/components/chat/chat-composer";

export type DraftAttachmentCardProps = {
  attachment: ChatDraftAttachment;
  onRemove: (attachmentId: string) => void;
  onRetry?: (attachmentId: string) => void;
  disabled?: boolean;
};

export function DraftAttachmentCard({
  attachment,
  onRemove,
  onRetry,
  disabled,
}: DraftAttachmentCardProps) {
  const displayName = getAttachmentDisplayName({
    name: attachment.name,
    mimeType: attachment.mimeType,
  });
  const visual = getAttachmentVisualKind({
    mimeType: attachment.mimeType,
    filename: attachment.name,
  });
  const warningLabel = getAttachmentWarningLabel(attachment.warningCode);
  const warningTooltip = getAttachmentWarningTooltip(attachment.warningCode);
  const isRemoving = attachment.status === "removing";
  const isUploading = attachment.status === "uploading" || attachment.status === "local_selected";
  const isUploadError = attachment.status === "error" && !attachment.removalError;
  const isRemovalError = Boolean(attachment.removalError);
  const removeDisabled = disabled || isRemoving;

  const statusTestId = isUploading
    ? "draft-attachment-uploading"
    : isUploadError || isRemovalError
      ? "draft-attachment-error"
      : "draft-attachment-ready";

  return (
    <div
      data-testid={statusTestId}
      data-attachment-status={attachment.status}
      className={cn(
        "flex min-h-[64px] items-center gap-2.5 rounded-[12px] border px-2.5 py-2 transition-colors",
        isUploadError || isRemovalError
          ? "border-red-200 bg-red-50/80"
          : "border-[#E2EAF5] bg-white hover:border-[#BFDBFE]",
        isRemoving && "pointer-events-none opacity-60",
      )}
      aria-live={isUploading || isUploadError || isRemovalError ? "polite" : undefined}
    >
      <AttachmentTypeIcon visual={visual} loading={isUploading} />

      <div className="min-w-0 flex-1">
        <p className="truncate text-[13px] font-medium leading-tight text-[#0F172A]" title={displayName}>
          {displayName}
        </p>
        {isRemovalError ? (
          <div className="mt-0.5 flex flex-wrap items-center gap-2">
            <p className="text-[12px] text-red-600">{attachment.removalError ?? "Не удалось удалить"}</p>
            <button
              type="button"
              className="text-[12px] font-medium text-aivion-blue hover:underline"
              onClick={() => onRemove(attachment.localId)}
              disabled={disabled}
            >
              Повторить
            </button>
          </div>
        ) : isUploadError ? (
          <div className="mt-0.5 flex flex-wrap items-center gap-2">
            <p className="text-[12px] text-red-600">Не удалось загрузить</p>
            {onRetry ? (
              <button
                type="button"
                className="text-[12px] font-medium text-aivion-blue hover:underline"
                onClick={() => onRetry(attachment.localId)}
                disabled={disabled}
              >
                Повторить
              </button>
            ) : null}
            <button
              type="button"
              className="text-[12px] font-medium text-red-600 hover:underline"
              onClick={() => onRemove(attachment.localId)}
              disabled={disabled}
            >
              Удалить
            </button>
          </div>
        ) : isUploading ? (
          <p className="mt-0.5 text-[12px] text-[#64748B]">Загрузка…</p>
        ) : (
          <>
            <p className="mt-0.5 text-[12px] text-[#64748B]">{formatFileSize(attachment.sizeBytes)}</p>
            {attachment.extractionStatus ? (
              <p className="mt-0.5 text-[11px] text-[#64748B]">
                Обработка: {attachment.extractionStatus}
              </p>
            ) : null}
            {warningLabel ? (
              <p
                className="mt-0.5 flex items-center gap-1 text-[11px] text-[#B45309]"
                title={warningTooltip ?? undefined}
              >
                <AlertTriangle className="h-3 w-3 shrink-0" aria-hidden="true" />
                <span className="truncate">{warningLabel}</span>
              </p>
            ) : null}
          </>
        )}
      </div>

      {!isUploadError && !isRemovalError ? (
        <button
          type="button"
          aria-label={`Удалить файл ${displayName}`}
          title="Удалить файл"
          disabled={removeDisabled}
          onClick={() => onRemove(attachment.localId)}
          className={cn(
            "inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-[#64748B] transition-colors",
            "hover:bg-red-50 hover:text-red-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-aivion-blue/40",
            "disabled:cursor-not-allowed disabled:opacity-60",
          )}
        >
          {isRemoving ? (
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          ) : (
            <X className="h-4 w-4" aria-hidden="true" />
          )}
        </button>
      ) : null}
    </div>
  );
}
