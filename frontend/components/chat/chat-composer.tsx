"use client";

import { Loader2, Paperclip, Send, Square } from "lucide-react";
import { useEffect, useRef } from "react";
import { DraftAttachmentsSection } from "@/components/chat/draft-attachments-section";
import { cn } from "@/components/ui/cn";
import {
  CHAT_ATTACHMENT_ACCEPT,
  CHAT_ATTACHMENT_MAX_FILES,
} from "@/lib/chat-display";

export type ChatDraftAttachment = {
  localId: string;
  name: string;
  sizeBytes: number;
  mimeType?: string;
  status: "local_selected" | "uploading" | "uploaded" | "error" | "removing";
  attachmentId?: string;
  error?: string;
  removalError?: string;
  warningCode?: string | null;
  extractionStatus?: string | null;
  completeness?: string | null;
  file?: File;
};

/** @deprecated Use ChatDraftAttachment */
export type ComposerAttachment = ChatDraftAttachment;

const PLACEHOLDER_WITH_ATTACHMENTS =
  "Опишите, что нужно сделать с приложенными файлами…";
const PLACEHOLDER_DEFAULT =
  "Задайте вопрос о продажах, остатках, рекламе или действиях…";
const PLACEHOLDER_EMPTY_CHAT =
  "Спросите, например: «Сделай ABC-анализ товарной базы за 30 дней»";

export function ChatComposer({
  value,
  onChange,
  onSubmit,
  onCancel,
  disabled,
  sending,
  isEmptyChat,
  placeholder,
  attachments,
  onPickFiles,
  onRemoveAttachment,
  onRetryAttachment,
  onRemoveAllAttachments,
  removingAll,
  attachmentSelectionError,
  onDismissAttachmentSelectionError,
}: {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel?: () => void;
  disabled?: boolean;
  sending?: boolean;
  isEmptyChat?: boolean;
  placeholder?: string;
  attachments?: ChatDraftAttachment[];
  onPickFiles?: (files: FileList) => void;
  onRemoveAttachment?: (localId: string) => void;
  onRetryAttachment?: (localId: string) => void;
  onRemoveAllAttachments?: () => void;
  removingAll?: boolean;
  attachmentSelectionError?: string | null;
  onDismissAttachmentSelectionError?: () => void;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const draftAttachments = attachments ?? [];
  const hasAttachments = draftAttachments.length > 0;
  const atFileLimit = draftAttachments.length >= CHAT_ATTACHMENT_MAX_FILES;

  const resolvedPlaceholder =
    placeholder ??
    (hasAttachments
      ? PLACEHOLDER_WITH_ATTACHMENTS
      : isEmptyChat
        ? PLACEHOLDER_EMPTY_CHAT
        : PLACEHOLDER_DEFAULT);

  const hasBlockingAttachment = draftAttachments.some(
    (item) =>
      item.status === "uploading" ||
      item.status === "removing" ||
      item.status === "local_selected" ||
      item.status === "error" ||
      Boolean(item.removalError),
  );
  const canSubmit =
    !disabled && !sending && !hasBlockingAttachment && value.trim().length > 0;

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key !== "Enter" || e.shiftKey) return;
    e.preventDefault();
    if (canSubmit) onSubmit();
  }

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    const next = Math.min(Math.max(el.scrollHeight, 60), 152);
    el.style.height = `${next}px`;
  }, [value]);

  return (
    <div
      className={cn(
        "w-full rounded-[20px] border border-[#DCE6F2] bg-white p-4 shadow-[0_8px_30px_rgba(15,23,42,0.04)] transition-[box-shadow,border-color]",
        "focus-within:border-blue-300 focus-within:ring-4 focus-within:ring-blue-50",
      )}
    >
      {hasAttachments || attachmentSelectionError ? (
        <DraftAttachmentsSection
          attachments={draftAttachments}
          onRemove={(id) => onRemoveAttachment?.(id)}
          onRetry={onRetryAttachment}
          onRemoveAll={onRemoveAllAttachments}
          removingAll={removingAll}
          disabled={disabled || sending}
          selectionError={attachmentSelectionError}
          onDismissSelectionError={onDismissAttachmentSelectionError}
        />
      ) : null}

      <textarea
        ref={textareaRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={resolvedPlaceholder}
        disabled={disabled || sending}
        aria-busy={sending}
        aria-label="Сообщение в чат"
        rows={2}
        className={cn(
          "min-h-[60px] max-h-[152px] w-full resize-none border-none bg-transparent p-0 text-sm leading-relaxed text-[#0F172A]",
          "placeholder:text-[#98A2B3] focus:outline-none focus:ring-0 disabled:opacity-60",
          hasAttachments && "mt-1",
        )}
      />

      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 sm:gap-3">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept={CHAT_ATTACHMENT_ACCEPT}
            className="hidden"
            onChange={(e) => {
              if (e.target.files && onPickFiles) onPickFiles(e.target.files);
              e.target.value = "";
            }}
          />
          <button
            type="button"
            disabled={disabled || sending || atFileLimit}
            onClick={() => fileInputRef.current?.click()}
            title={
              atFileLimit
                ? `Можно прикрепить не более ${CHAT_ATTACHMENT_MAX_FILES} файлов`
                : "Прикрепить файл"
            }
            className={cn(
              "inline-flex h-9 shrink-0 items-center gap-1.5 rounded-[9px] px-2.5 text-[13px] font-medium text-aivion-blue transition-colors",
              "hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-aivion-blue/40",
              "disabled:cursor-not-allowed disabled:opacity-55",
            )}
            aria-label="Прикрепить файл"
          >
            <Paperclip className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="hidden sm:inline">Добавить файл</span>
          </button>

          {hasAttachments ? (
            <>
              <span className="hidden h-4 w-px bg-[#DCE6F2] sm:block" aria-hidden="true" />
              <span className="text-[12px] text-[#64748B]">
                {draftAttachments.length} из {CHAT_ATTACHMENT_MAX_FILES}
              </span>
            </>
          ) : null}
        </div>

        <div className="flex items-center gap-2 sm:gap-3">
          <p className="hidden text-[11px] text-[#64748B] lg:block">
            Enter — отправить · Shift+Enter — новая строка
          </p>
          <button
            type="button"
            disabled={sending ? !onCancel : !canSubmit}
            onClick={sending ? onCancel : onSubmit}
            className={cn(
              "inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-[13px] bg-aivion-blue text-white shadow-aivion-sm transition-colors",
              "hover:bg-[#0668e0] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-aivion-blue/50",
              "disabled:cursor-not-allowed disabled:bg-[#93C5FD] disabled:opacity-100",
            )}
            aria-label={sending ? "Отменить генерацию" : "Отправить сообщение"}
          >
            {sending ? (
              onCancel ? (
                <Square className="h-4 w-4 fill-current" aria-hidden="true" />
              ) : (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              )
            ) : (
              <Send className="h-4 w-4" aria-hidden="true" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
