import type { ChatSession } from "@/lib/chat-api";

export type ChatViewMode =
  | { kind: "new_general" }
  | { kind: "draft_action"; actionKey: string; prefill: string }
  | { kind: "existing_session"; sessionId: number };

export function parseSessionId(raw: string | null | undefined): number | null {
  const parsed = Number.parseInt((raw ?? "").trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export function resolveChatViewMode(params: {
  urlSessionId: number | null;
  draftActionKey: string | null;
  draftPrefill: string;
}): ChatViewMode {
  if (params.urlSessionId != null) {
    return { kind: "existing_session", sessionId: params.urlSessionId };
  }
  const actionKey = (params.draftActionKey ?? "").trim();
  if (actionKey) {
    return { kind: "draft_action", actionKey, prefill: params.draftPrefill };
  }
  return { kind: "new_general" };
}

export function shouldShowSuggestions(
  mode: ChatViewMode,
  messageCount: number,
  loadingMessages: boolean,
): boolean {
  if (loadingMessages) return false;
  if (mode.kind !== "new_general") return false;
  return messageCount === 0;
}

export function shouldShowEmptyState(
  mode: ChatViewMode,
  messageCount: number,
  loadingMessages: boolean,
): boolean {
  if (loadingMessages) return false;
  // Action draft / existing sessions have their own placeholders — never show
  // the general "cabinet chat" empty state with prompt chips there.
  if (mode.kind !== "new_general") return false;
  return messageCount === 0;
}

export function isActionSession(session: ChatSession): boolean {
  return Boolean(session.action_view_id?.trim() || session.binding_id);
}

export {
  ACTION_ASK_MESSAGE,
  ACTION_ASK_PREFILL,
} from "@/lib/chat-action-handoff";

export function applyPromptChip(current: string, prompt: string): string {
  const trimmed = current.trim();
  if (!trimmed) return prompt;
  return `${trimmed}\n${prompt}`;
}

export function buildChatTitle(params: {
  actionTitle?: string | null;
  firstUserMessage?: string | null;
  fallback?: string;
}): string {
  const max = 72;
  const pick = (raw?: string | null) => {
    const s = (raw ?? "").trim();
    if (!s) return "";
    if (s.length <= max) return s;
    return `${s.slice(0, max)}…`;
  };
  return (
    pick(params.actionTitle) ||
    pick(params.firstUserMessage) ||
    pick(params.fallback) ||
    "Новый чат"
  );
}
