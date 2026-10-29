const CHAT_DEBUG =
  typeof process !== "undefined" &&
  process.env.NODE_ENV === "development" &&
  process.env.NEXT_PUBLIC_CHAT_DEBUG === "1";

export type ChatDebugPayload = Record<string, string | number | boolean | null | undefined>;

export function chatDebug(event: string, payload?: ChatDebugPayload): void {
  if (!CHAT_DEBUG) return;
  if (payload) {
    console.debug(`[chat] ${event}`, payload);
    return;
  }
  console.debug(`[chat] ${event}`);
}
