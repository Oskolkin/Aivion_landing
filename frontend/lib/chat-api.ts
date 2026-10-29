import { apiDelete, apiGet, apiPatch, apiPost, apiUploadForm, type ApiError } from "@/lib/api";
import { CHAT_ATTACHMENT_DELETE_TIMEOUT_MS } from "@/lib/chat-attachments";
import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";

export type ChatSessionStatus = "active" | "archived";
export type ChatMessageRole = "user" | "assistant" | "system";
export type ChatMessageType = "question" | "answer" | "error" | "meta";
export type ChatFeedbackRating = "positive" | "negative" | "neutral";
export type ChatAnswerMode =
  | "PUBLISHED_EXPLANATION"
  | "FACTUAL_QUERY"
  | "AD_HOC_ANALYSIS"
  | "ACTION_ASSISTANCE"
  | "ATTACHMENT_ANALYSIS"
  | "MIXED_ANALYSIS"
  | "EXPORT"
  | "GENERAL_GUIDANCE";
export type ChatCompleteness = "COMPLETE" | "PARTIAL" | "UNREADABLE" | "VISION_REQUIRED";

export interface ChatSession {
  id: number;
  title: string;
  action_view_id?: string | null;
  binding_id?: number | null;
  status: ChatSessionStatus;
  created_at: string;
  updated_at: string;
  last_message_at?: string | null;
}

export interface ChatAttachment {
  id: string;
  file_name: string;
  mime_type: string;
  file_size: number;
  export_type: string;
  format: string;
  download_url: string;
  created_at: string;
  completeness?: ChatCompleteness | string;
  warning?: string;
}

export interface ChatMessage {
  id: number;
  session_id: number;
  role: ChatMessageRole;
  message_type: ChatMessageType;
  content: string;
  created_at: string;
  attachments?: ChatAttachment[];
}

export interface AskChatRequest {
  session_id?: number;
  question: string;
  attachment_ids?: string[];
  client_request_id?: string;
  context_binding_id?: number;
}

export interface ChatPublicationDTO {
  id: string;
  analysis_date: string;
  analysis_cutoff_at: string;
}

export interface AskChatResponse {
  session_id: number;
  session_created?: boolean;
  turn_id?: number;
  user_message_id: number;
  assistant_message_id?: number | null;
  message_id?: number | null;
  answer: string;
  answer_mode?: ChatAnswerMode | string;
  context_mode?: string;
  publication?: ChatPublicationDTO | null;
  referenced_entities?: { kind: string; id: string }[];
  grounding_refs?: { handle: string; type: string; id: string }[];
  limitations: string[];
  attachments?: ChatAttachment[];
  cancelled?: boolean;
}

export type AskStreamStatus =
  | "queued"
  | "resolving_context"
  | "reading_data"
  | "analyzing"
  | "validating"
  | "streaming"
  | "completed"
  | "failed"
  | "cancelled";

export interface AskChatStreamOptions {
  onStatus?: (status: AskStreamStatus) => void;
  onDelta?: (delta: string) => void;
  onDone?: (result: AskChatResponse) => void;
  onError?: (error: string) => void;
  signal?: AbortSignal;
}

type AskStreamEvent = {
  type: "status" | "delta" | "done" | "error";
  status?: AskStreamStatus;
  delta?: string;
  result?: AskChatResponse;
  error?: string;
};

export interface ChatSessionByActionResponse {
  found: boolean;
  session?: ChatSession;
}

export interface ChatInputAttachment {
  id: string;
  name: string;
  mime_type: string;
  size_bytes: number;
  status: string;
  extraction_status?: string | null;
  warning_code?: string | null;
  warning_text?: string | null;
  completeness?: ChatCompleteness | string | null;
}

export interface UploadChatAttachmentResponse {
  attachment: ChatInputAttachment;
}

export interface ListChatSessionsResponse {
  items: ChatSession[];
  limit: number;
  offset: number;
}

export interface ListChatMessagesResponse {
  items: ChatMessage[];
  limit: number;
  offset: number;
}

export interface ChatFeedback {
  id: number;
  session_id: number;
  message_id: number;
  rating: ChatFeedbackRating;
  comment?: string | null;
  created_at: string;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") continue;
    query.set(k, String(v));
  }
  const encoded = query.toString();
  return encoded ? `?${encoded}` : "";
}

export async function askChat(payload: AskChatRequest): Promise<AskChatResponse> {
  return apiPost<AskChatResponse>("/api/v1/chat/ask", payload);
}

export async function createActionChatHandoff(payload: {
  action_view_id: string;
  client_request_id?: string;
}): Promise<{ session_id: number; binding_id: number; title: string; created?: boolean }> {
  return apiPost("/api/v1/chat/action-handoffs", payload);
}

export async function askChatStream(
  payload: AskChatRequest,
  { onStatus, onDelta, onDone, onError, signal }: AskChatStreamOptions,
): Promise<void> {
  const response = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/chat/ask/stream`, {
    method: "POST",
    credentials: "include",
    headers: { Accept: "text/event-stream", "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    signal,
  });
  if (!response.ok) {
    const contentType = response.headers.get("content-type") ?? "";
    const message = contentType.includes("application/json")
      ? ((await response.json() as ApiError).error ?? `Не удалось отправить вопрос (${response.status})`)
      : `Не удалось отправить вопрос (${response.status})`;
    throw new Error(message);
  }
  if (!response.body) throw new Error("Поток ответа недоступен");

  const handleEvent = (raw: string) => {
    const data = raw
      .split(/\r?\n/)
      .find((line) => line.startsWith("data:"))
      ?.slice("data:".length)
      .trim();
    if (!data) return;
    const event = JSON.parse(data) as AskStreamEvent;
    if (event.status) onStatus?.(event.status);
    if (event.type === "delta" && event.delta) onDelta?.(event.delta);
    if (event.type === "error") {
      const message = event.error || "Не удалось обработать вопрос";
      onError?.(message);
      throw new Error(message);
    }
    if (event.type === "done" && event.result) {
      const answer = typeof event.result.answer === "string" ? event.result.answer : "";
      onDone?.({
        ...event.result,
        answer,
        limitations: event.result.limitations ?? [],
        attachments: event.result.attachments ?? [],
      });
    }
  };

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    const events = buffer.split(/\r?\n\r?\n/);
    buffer = events.pop() ?? "";
    events.forEach(handleEvent);
    if (done) break;
  }
  if (buffer.trim()) handleEvent(buffer);
}

export async function getChatSessionByActionId(actionId: number): Promise<ChatSessionByActionResponse> {
  return apiGet<ChatSessionByActionResponse>(`/api/v1/chat/sessions/by-action/${actionId}`);
}

export async function lookupChatSessionByAction(actionKey: string): Promise<ChatSessionByActionResponse> {
  return apiPost<ChatSessionByActionResponse>("/api/v1/chat/sessions/lookup-by-action", {
    action_key: actionKey,
  });
}

export async function getChatSessionByAction(actionKey: string): Promise<ChatSessionByActionResponse> {
  if (actionKey.length > 512) {
    return lookupChatSessionByAction(actionKey);
  }
  const query = new URLSearchParams({ action_key: actionKey });
  return apiGet<ChatSessionByActionResponse>(`/api/v1/chat/sessions/by-action?${query.toString()}`);
}

export async function uploadChatAttachment(
  file: File,
  options?: { signal?: AbortSignal },
): Promise<UploadChatAttachmentResponse> {
  const form = new FormData();
  form.append("file", file);
  return apiUploadForm<UploadChatAttachmentResponse>("/api/v1/chat/attachments", form, options);
}

export async function deleteChatAttachment(
  id: string,
  options?: { signal?: AbortSignal; timeoutMs?: number },
): Promise<void> {
  const timeoutMs = options?.timeoutMs ?? CHAT_ATTACHMENT_DELETE_TIMEOUT_MS;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  if (options?.signal) {
    if (options.signal.aborted) {
      clearTimeout(timeoutId);
      controller.abort();
    } else {
      options.signal.addEventListener("abort", () => controller.abort(), { once: true });
    }
  }

  try {
    const res = await fetch(
      `${PUBLIC_API_BASE_URL}/api/v1/chat/attachments/${encodeURIComponent(id)}`,
      {
        method: "DELETE",
        credentials: "include",
        signal: controller.signal,
      },
    );

    if (res.status === 204 || res.status === 404) {
      return;
    }

    if (!res.ok) {
      const contentType = res.headers.get("content-type") || "";
      if (contentType.includes("application/json")) {
        const data = (await res.json()) as ApiError;
        throw new Error(data.error || "Не удалось удалить файл. Повторите попытку.");
      }
      throw new Error(`Не удалось удалить файл. Повторите попытку. (${res.status})`);
    }

    if (res.status !== 200) {
      return;
    }

    const contentType = res.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      await res.json();
    }
  } catch (error: unknown) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new Error("Не удалось удалить файл. Повторите попытку.");
    }
    throw error;
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function getChatSessions(params?: {
  limit?: number;
  offset?: number;
}): Promise<ListChatSessionsResponse> {
  const query = buildQuery({
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<ListChatSessionsResponse>(`/api/v1/chat/sessions${query}`);
}

export async function getChatSession(id: number): Promise<ChatSession> {
  return apiGet<ChatSession>(`/api/v1/chat/sessions/${id}`);
}

export async function getChatMessages(
  sessionId: number,
  params?: { limit?: number; offset?: number; signal?: AbortSignal },
): Promise<ListChatMessagesResponse> {
  const query = buildQuery({
    limit: params?.limit,
    offset: params?.offset,
  });
  return apiGet<ListChatMessagesResponse>(
    `/api/v1/chat/sessions/${sessionId}/messages${query}`,
    { signal: params?.signal },
  );
}

export async function archiveChatSession(id: number): Promise<ChatSession> {
  return apiPost<ChatSession>(`/api/v1/chat/sessions/${id}/archive`);
}

export async function renameChatSession(id: number, title: string): Promise<ChatSession> {
  return apiPatch<ChatSession>(`/api/v1/chat/sessions/${id}`, { title });
}

export async function deleteChatSession(id: number): Promise<void> {
  await apiDelete(`/api/v1/chat/sessions/${id}`);
}

export async function createChatFeedback(
  messageId: number,
  payload: { session_id: number; rating: ChatFeedbackRating; comment?: string },
): Promise<ChatFeedback> {
  return apiPost<ChatFeedback>(`/api/v1/chat/messages/${messageId}/feedback`, payload);
}
