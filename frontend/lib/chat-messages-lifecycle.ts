import type { ChatViewMode } from "@/lib/chat-view";

export function existingSessionIdFromMode(mode: ChatViewMode): number | null {
  return mode.kind === "existing_session" ? mode.sessionId : null;
}

/** True when the visible message list should be cleared (session/mode actually changed). */
export function shouldClearVisibleMessages(
  prevSessionId: number | null,
  nextSessionId: number | null,
  nextKind: ChatViewMode["kind"],
): boolean {
  if (nextKind !== "existing_session") return true;
  if (prevSessionId === null) return nextSessionId !== null;
  return prevSessionId !== nextSessionId;
}

/** Effect should fetch only when the primitive session id changed. */
export function shouldFetchMessagesForSession(
  kind: ChatViewMode["kind"],
  sessionId: number | null,
  lastFetchedSessionId: number | null,
): boolean {
  if (kind !== "existing_session" || sessionId == null) return false;
  return lastFetchedSessionId !== sessionId;
}

export function isStaleMessagesResponse(
  requestVersion: number,
  currentVersion: number,
  requestedSessionId: number,
  activeSessionId: number | null,
): boolean {
  if (requestVersion !== currentVersion) return true;
  if (activeSessionId == null) return true;
  return requestedSessionId !== activeSessionId;
}
