/**
 * Action → AI-chat handoff using Prompt14 action_view_id.
 */

export type ActionHandoffPhase =
  | "idle"
  | "navigating"
  | "resolving_action"
  | "creating_or_loading_conversation"
  | "attaching_context"
  | "sending_initial_message"
  | "ready"
  | "failed";

export const ACTION_ASK_MESSAGE =
  "Помоги выполнить это действие. Проанализируй затронутые объекты, объясни порядок действий и уточни, какие данные нужно проверить в первую очередь.";

/** @deprecated Use ACTION_ASK_MESSAGE; kept as alias for older imports. */
export const ACTION_ASK_PREFILL = ACTION_ASK_MESSAGE;

const HANDOFF_STORAGE_PREFIX = "aivion:action-handoff:";

const inflightHandoffs = new Map<string, Promise<void>>();

export function actionHandoffStorageKey(actionViewId: string): string {
  return `${HANDOFF_STORAGE_PREFIX}${actionViewId}`;
}

export function handoffInflightKey(params: { actionViewId?: string | null }): string {
  const id = (params.actionViewId ?? "").trim();
  return id ? `view:${id}` : "unknown";
}

export async function withActionHandoffLock(key: string, work: () => Promise<void>): Promise<void> {
  const existing = inflightHandoffs.get(key);
  if (existing) {
    await existing.catch(() => undefined);
    // Strict Mode remount: prior pass may have been cancelled before completing.
    if (inflightHandoffs.has(key)) {
      return;
    }
  }
  const run = work().finally(() => {
    if (inflightHandoffs.get(key) === run) {
      inflightHandoffs.delete(key);
    }
  });
  inflightHandoffs.set(key, run);
  await run;
}

export function getOrCreateActionHandoffRequestId(actionViewId: string): string {
  const key = actionHandoffStorageKey(actionViewId);
  try {
    const existing = sessionStorage.getItem(key)?.trim();
    if (existing) return existing;
    const id =
      typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
        ? crypto.randomUUID()
        : `action-handoff-${actionViewId}-${Date.now()}`;
    sessionStorage.setItem(key, id);
    return id;
  } catch {
    return `action-handoff-${actionViewId}`;
  }
}

export function clearActionHandoffRequestId(actionViewId: string): void {
  try {
    sessionStorage.removeItem(actionHandoffStorageKey(actionViewId));
  } catch {
    // ignore
  }
}

export function canStartActionHandoff(phase: ActionHandoffPhase): boolean {
  return phase === "idle" || phase === "failed";
}

export function isActionHandoffBusy(phase: ActionHandoffPhase): boolean {
  return (
    phase === "navigating" ||
    phase === "resolving_action" ||
    phase === "creating_or_loading_conversation" ||
    phase === "attaching_context" ||
    phase === "sending_initial_message"
  );
}

export function shouldAutoSendActionHandoff(phase: ActionHandoffPhase): boolean {
  return phase === "sending_initial_message";
}

export function nextHandoffPhaseAfterResolve(foundExisting: boolean): ActionHandoffPhase {
  return foundExisting ? "ready" : "creating_or_loading_conversation";
}

export function buildActionChatHref(params: {
  sessionId?: number | null;
  actionViewId?: string | null;
}): string {
  if (params.sessionId != null && params.sessionId > 0) {
    return `/app/chat?session=${params.sessionId}`;
  }
  if (params.actionViewId) {
    return `/app/chat?action_view_id=${encodeURIComponent(params.actionViewId)}`;
  }
  return "/app/chat";
}
