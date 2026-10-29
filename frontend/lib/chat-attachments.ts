import { CHAT_ATTACHMENT_MAX_FILES } from "@/lib/chat-display";

export const CHAT_ATTACHMENT_DELETE_TIMEOUT_MS = 15_000;

export function buildAttachmentLimitError({
  selectedCount,
  existingCount,
  maxCount = CHAT_ATTACHMENT_MAX_FILES,
}: {
  selectedCount: number;
  existingCount: number;
  maxCount?: number;
}): string {
  const remaining = Math.max(0, maxCount - existingCount);

  if (existingCount >= maxCount) {
    return `Достигнут лимит: можно прикрепить не более ${maxCount} файлов.`;
  }

  if (existingCount === 0) {
    return `Можно прикрепить не более ${maxCount} файлов. Вы выбрали ${selectedCount} — файлы не были добавлены.`;
  }

  const remainingLabel =
    remaining === 1 ? "1 файл" : remaining >= 2 && remaining <= 4 ? `${remaining} файла` : `${remaining} файлов`;

  return `Можно добавить ещё ${remainingLabel}. Вы выбрали ${selectedCount} — файлы не были добавлены.`;
}

export function mapAttachmentRemovalError(error: unknown): string {
  if (error instanceof Error) {
    if (error.name === "AbortError" || error.message.includes("AbortError")) {
      return "Не удалось удалить файл. Повторите попытку.";
    }
    return error.message || "Не удалось удалить файл. Повторите попытку.";
  }
  return "Не удалось удалить файл. Повторите попытку.";
}

export function validateAttachmentBatchSelection({
  selectedCount,
  existingCount,
  maxCount = CHAT_ATTACHMENT_MAX_FILES,
}: {
  selectedCount: number;
  existingCount: number;
  maxCount?: number;
}): { ok: true } | { ok: false; message: string } {
  const remainingSlots = maxCount - existingCount;
  if (selectedCount <= 0) {
    return { ok: true };
  }
  if (selectedCount > remainingSlots) {
    return {
      ok: false,
      message: buildAttachmentLimitError({ selectedCount, existingCount, maxCount }),
    };
  }
  return { ok: true };
}
