import type { ChatAttachment, ChatSession } from "@/lib/chat-api";
import type { ChatAttachmentDisplay, ChatSessionDisplay } from "@/lib/chat-types";
import { buildChatTitle, isActionSession } from "@/lib/chat-view";

export { isActionSession };

const CONTROLLED_FALLBACK_PREFIXES = [
  "Ответ не прошёл проверку по фактам из кабинета.",
  "Не удалось спланировать запрос к данным кабинета.",
  "Не удалось построить безопасный план чтения данных.",
  "Не удалось получить данные для ответа.",
  "Недостаточно подготовленных данных для ответа.",
  "Достигнут дневной лимит AI-запросов для кабинета.",
  "Не удалось обработать запрос.",
] as const;

const CONTROLLED_FALLBACK_LIMITATION_MARKERS = [
  "Ответ сформирован без вызова OpenAI (controlled fallback).",
  "Ответ сформирован без вызова OpenAI (precheck).",
] as const;

export function isCorruptedChatTitle(title: string): boolean {
  const s = (title ?? "").trim();
  if (!s) return true;

  const questionMarks = (s.match(/\?/g) ?? []).length;
  const cyrillic = s.match(/[\u0400-\u04FF]/g) ?? [];
  const lettersAndDigits = s.match(/[\p{L}\p{N}]/gu) ?? [];

  if (lettersAndDigits.length === 0) return true;
  if (questionMarks >= 3 && cyrillic.length === 0) return true;
  if (questionMarks >= 3 && questionMarks / s.length >= 0.2) return true;

  return false;
}

export function resolveSessionTitle(session: ChatSession): string {
  const raw = (session.title ?? "").trim();
  if (!isCorruptedChatTitle(raw)) {
    return buildChatTitle({
      actionTitle: isActionSession(session) ? raw : null,
      firstUserMessage: isActionSession(session) ? null : raw,
    });
  }
  if (isActionSession(session)) {
    return "Чат по действию";
  }
  return "Новый чат";
}

export function isControlledFallbackContent(content: string): boolean {
  const trimmed = content.trimStart();
  return CONTROLLED_FALLBACK_PREFIXES.some((prefix) => trimmed.startsWith(prefix));
}

export function isControlledFallbackLimitation(limitations: string[] | undefined): boolean {
  if (!limitations?.length) return false;
  return limitations.some((item) =>
    CONTROLLED_FALLBACK_LIMITATION_MARKERS.some((marker) => item.includes(marker)),
  );
}

export function isControlledFallbackMessage(
  content: string,
  limitations?: string[],
): boolean {
  return isControlledFallbackLimitation(limitations) || isControlledFallbackContent(content);
}

export function getInitialSelectedSessionId(initialSessionId?: number): number | null {
  if (initialSessionId != null && initialSessionId > 0) return initialSessionId;
  return null;
}

export function mapSessionToDisplay(session: ChatSession): ChatSessionDisplay {
  return {
    id: session.id,
    title: resolveSessionTitle(session),
    kind: isActionSession(session) ? "action" : "general",
    status: session.status === "archived" ? "archived" : "active",
    badgeLabel: "",
    subtitle: "",
    lastMessageAt: session.last_message_at ?? session.updated_at,
    actionViewId: session.action_view_id ?? undefined,
    raw: session,
  };
}

export function formatSessionTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;

  const now = new Date();
  const sameDay =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate();

  if (sameDay) {
    return date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  }

  return date.toLocaleDateString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

export function formatMessageTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
}

export type SessionGroup = {
  today: ChatSessionDisplay[];
  earlier: ChatSessionDisplay[];
};

function sessionSortTime(session: ChatSessionDisplay): number {
  const iso = session.lastMessageAt ?? session.raw.updated_at;
  const t = new Date(iso).getTime();
  return Number.isNaN(t) ? 0 : t;
}

export function groupSessionsByDate(sessions: ChatSession[]): SessionGroup {
  const now = new Date();
  const displays = sessions.map(mapSessionToDisplay).sort((a, b) => sessionSortTime(b) - sessionSortTime(a));

  const today: ChatSessionDisplay[] = [];
  const earlier: ChatSessionDisplay[] = [];

  for (const session of displays) {
    const iso = session.lastMessageAt ?? session.raw.updated_at;
    const date = new Date(iso);
    const sameDay =
      !Number.isNaN(date.getTime()) &&
      date.getFullYear() === now.getFullYear() &&
      date.getMonth() === now.getMonth() &&
      date.getDate() === now.getDate();

    if (sameDay) today.push(session);
    else earlier.push(session);
  }

  return { today, earlier };
}

export function splitActiveAndArchived(sessions: ChatSession[]): {
  active: ChatSession[];
  archived: ChatSession[];
} {
  const active: ChatSession[] = [];
  const archived: ChatSession[] = [];
  for (const session of sessions) {
    if (session.status === "archived") archived.push(session);
    else active.push(session);
  }
  return { active, archived };
}

export const EMPTY_STATE_PROMPT_CHIPS = [
  "Сделай ABC-анализ товарной базы",
  "Какие SKU требуют внимания?",
  "Где я теряю деньги на рекламе?",
  "Выгрузи список SKU для поставки",
] as const;

export function isSessionListItemSelected(
  viewModeKind: "new_general" | "draft_action" | "existing_session",
  selectedSessionId: number | null,
  sessionId: number,
): boolean {
  if (viewModeKind !== "existing_session") return false;
  return selectedSessionId === sessionId;
}

export const CHAT_ATTACHMENT_MAX_FILES = 5;

export const SPREADSHEET_TRUNCATED_WARNING_CODE = "spreadsheet_truncated_for_direct_ai_analysis";

export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes === 0) return "0 Б";
  if (bytes < 1024) return `${bytes} Б`;
  if (bytes < 1024 * 1024) {
    const kb = bytes / 1024;
    return `${kb.toLocaleString("ru-RU", { maximumFractionDigits: 1 })} КБ`;
  }
  const mb = bytes / (1024 * 1024);
  return `${mb.toLocaleString("ru-RU", { maximumFractionDigits: 1 })} МБ`;
}

export function formatAttachedFilesCount(count: number): string {
  const n = Math.abs(count);
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return `Прикреплён ${n} файл`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `Прикреплено ${n} файла`;
  return `Прикреплено ${n} файлов`;
}

export type AttachmentVisualKind = {
  label: string;
  iconClassName: string;
  containerClassName: string;
};

function extensionFromName(filename?: string): string {
  const ext = (filename ?? "").split(".").pop()?.toLowerCase() ?? "";
  return ext;
}

export function getAttachmentVisualKind({
  mimeType,
  filename,
  format,
}: {
  mimeType?: string;
  filename?: string;
  format?: string;
}): AttachmentVisualKind {
  const mime = (mimeType ?? "").toLowerCase();
  const ext = extensionFromName(filename) || (format ?? "").toLowerCase();

  if (mime.includes("pdf") || ext === "pdf") {
    return { label: "PDF", iconClassName: "text-white", containerClassName: "bg-[#E85D5D]" };
  }
  if (
    mime.includes("spreadsheet") ||
    mime.includes("excel") ||
    ext === "xlsx" ||
    ext === "xls"
  ) {
    return { label: "XLSX", iconClassName: "text-white", containerClassName: "bg-[#3DB87A]" };
  }
  if (mime.includes("csv") || ext === "csv") {
    return { label: "CSV", iconClassName: "text-white", containerClassName: "bg-[#2BA8B8]" };
  }
  if (mime.includes("wordprocessingml") || ext === "docx" || ext === "doc") {
    return { label: "DOCX", iconClassName: "text-white", containerClassName: "bg-[#4B8FE8]" };
  }
  if (mime.includes("json") || ext === "json") {
    return { label: "JSON", iconClassName: "text-white", containerClassName: "bg-[#7C6FE8]" };
  }
  if (mime.includes("markdown") || ext === "md") {
    return { label: "MD", iconClassName: "text-white", containerClassName: "bg-[#8B95A8]" };
  }
  if (mime.includes("plain") || ext === "txt") {
    return { label: "TXT", iconClassName: "text-white", containerClassName: "bg-[#7B8DA6]" };
  }
  const fallback = ext.length > 0 && ext.length <= 5 ? ext.toUpperCase() : "FILE";
  return { label: fallback, iconClassName: "text-white", containerClassName: "bg-[#94A3B8]" };
}

export function isInternalAttachmentName(name: string): boolean {
  const s = (name ?? "").trim();
  if (!s) return true;
  if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s)) return true;
  if (/^attachment_[0-9a-f]{8,}/i.test(s)) return true;
  if (s.includes("chat-attachments/")) return true;
  if (/^[0-9a-f]{16,}\.[a-z0-9]+$/i.test(s)) return true;
  return false;
}

export function getAttachmentDisplayName({
  name,
  mimeType,
  format,
}: {
  name?: string | null;
  mimeType?: string;
  format?: string;
}): string {
  const candidate = (name ?? "").trim();
  if (candidate && !isInternalAttachmentName(candidate)) return candidate;

  const visual = getAttachmentVisualKind({ mimeType, filename: candidate, format });
  if (visual.label === "PDF") return "Документ PDF";
  if (visual.label === "XLSX") return "Таблица Excel";
  if (visual.label === "CSV") return "CSV-файл";
  if (visual.label === "DOCX") return "Документ Word";
  if (visual.label === "TXT" || visual.label === "MD") return "Текстовый файл";
  if (visual.label === "JSON") return "JSON-файл";
  return "Файл";
}

export function getAttachmentWarningLabel(warningCode?: string | null): string | null {
  if (!warningCode) return null;
  if (warningCode === SPREADSHEET_TRUNCATED_WARNING_CODE || warningCode === "PARTIAL") {
    return "Файл обработан частично";
  }
  if (warningCode === "UNREADABLE") {
    return "Файл не удалось прочитать";
  }
  if (warningCode === "VISION_REQUIRED") {
    return "Нужен просмотр изображения";
  }
  return null;
}

export function getAttachmentWarningTooltip(warningCode?: string | null): string | null {
  if (!warningCode) return null;
  if (warningCode === SPREADSHEET_TRUNCATED_WARNING_CODE || warningCode === "PARTIAL") {
    return "Из файла извлечена только часть данных. Ответ по кабинету по-прежнему доступен.";
  }
  if (warningCode === "UNREADABLE") {
    return "Содержимое файла недоступно для анализа.";
  }
  if (warningCode === "VISION_REQUIRED") {
    return "Этот файл требует визуального разбора; текстовое извлечение ограничено.";
  }
  return null;
}

export function completenessWarningCode(completeness?: string | null): string | null {
  const c = (completeness ?? "").toUpperCase();
  if (c === "PARTIAL" || c === "UNREADABLE" || c === "VISION_REQUIRED") return c;
  return null;
}

export function mapAttachmentToDisplay(att: ChatAttachment): ChatAttachmentDisplay {
  const format = (att.format || "").toLowerCase();
  const visual = getAttachmentVisualKind({
    mimeType: att.mime_type,
    filename: att.file_name,
    format,
  });

  return {
    id: att.id,
    name: getAttachmentDisplayName({
      name: att.file_name,
      mimeType: att.mime_type,
      format,
    }),
    kind: visual.label.toLowerCase() as ChatAttachmentDisplay["kind"],
    visualLabel: visual.label,
    sizeLabel: formatFileSize(att.file_size),
    downloadUrl: att.download_url,
    warningCode: att.warning || completenessWarningCode(att.completeness),
  };
}

export const CHAT_ATTACHMENT_ACCEPT =
  ".pdf,.xlsx,.xls,.csv,.tsv,.docx,.pptx,.txt,.md,.json,.png,.jpeg,.jpg,.webp,.zip,application/pdf,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel,text/csv,text/tab-separated-values,text/plain,text/markdown,application/json,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.presentationml.presentation,image/png,image/jpeg,image/webp,application/zip";
