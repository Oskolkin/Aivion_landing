"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ChatAssistantMessage } from "@/components/chat/chat-assistant-message";
import { ChatComposer, type ComposerAttachment } from "@/components/chat/chat-composer";
import { ChatContextCard } from "@/components/chat/chat-context-card";
import { ChatEmptyState } from "@/components/chat/chat-empty-state";
import { ChatMessagesSkeleton } from "@/components/chat/chat-messages-skeleton";
import { ChatSessionList } from "@/components/chat/chat-session-list";
import { ChatUserMessage } from "@/components/chat/chat-user-message";
import { PageHeader } from "@/components/ui/page-header";
import {
  createActionChatHandoff,
  archiveChatSession,
  askChatStream,
  deleteChatAttachment,
  deleteChatSession,
  getChatMessages,
  getChatSessions,
  renameChatSession,
  uploadChatAttachment,
  type AskChatResponse,
  type ChatMessage,
  type ChatSession,
} from "@/lib/chat-api";
import {
  ACTION_ASK_MESSAGE,
  type ActionHandoffPhase,
  clearActionHandoffRequestId,
  getOrCreateActionHandoffRequestId,
  handoffInflightKey,
  isActionHandoffBusy,
  withActionHandoffLock,
} from "@/lib/chat-action-handoff";
import {
  mapAttachmentRemovalError,
  validateAttachmentBatchSelection,
} from "@/lib/chat-attachments";
import { chatDebug } from "@/lib/chat-debug";
import {
  completenessWarningCode,
  groupSessionsByDate,
  isActionSession,
  mapAttachmentToDisplay,
  mapSessionToDisplay,
  splitActiveAndArchived,
} from "@/lib/chat-display";
import type { ChatSessionDisplay } from "@/lib/chat-types";
import {
  existingSessionIdFromMode,
  isStaleMessagesResponse,
  shouldClearVisibleMessages,
  shouldFetchMessagesForSession,
} from "@/lib/chat-messages-lifecycle";
import { applyPromptChip, parseSessionId, resolveChatViewMode, shouldShowEmptyState } from "@/lib/chat-view";
import { getActionView } from "@/lib/publication-actions-api";

function looksLikeStackTraceOrNoise(text: string): boolean {
  return (
    text.length > 280 ||
    text.includes("\n\t") ||
    text.includes("goroutine") ||
    text.includes("stacktrace") ||
    text.includes("at go.") ||
    text.includes(".go:")
  );
}

function friendlyChatAskError(raw: string): string {
  const s = raw.toLowerCase();
  if (
    s.includes("503") ||
    s.includes("502") ||
    s.includes("504") ||
    s.includes("service unavailable") ||
    s.includes("temporarily unavailable") ||
    s.includes("unavailable") ||
    s.includes("openai") ||
    s.includes("rate limit") ||
    s.includes("timeout")
  ) {
    return "ИИ временно недоступен. Проверьте настройки OpenAI или попробуйте позже.";
  }
  if (looksLikeStackTraceOrNoise(raw)) {
    return "Что-то пошло не так. Попробуйте позже.";
  }
  return raw;
}

function newLocalId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random()}`;
}

export default function ChatScreen({
  initialSessionId: _initialSessionId,
  initialActionViewId,
  initialDraftMode,
  isLiveReview = false,
}: {
  initialSessionId?: number;
  initialActionViewId?: string;
  initialDraftMode?: boolean;
  isLiveReview?: boolean;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const scrollRef = useRef<HTMLDivElement>(null);
  const submitLockRef = useRef(false);
  const isNearBottomRef = useRef(true);
  const messagesRequestVersionRef = useRef(0);
  const lastFetchedSessionIdRef = useRef<number | null>(null);
  const activeSessionIdRef = useRef<number | null>(null);
  const inFlightSessionIdsRef = useRef(new Set<number>());

  const rawSessionParam = searchParams.get("session");
  const urlSessionId = useMemo(() => parseSessionId(rawSessionParam), [rawSessionParam]);

  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [draftActionKey, setDraftActionKey] = useState<string | null>(() =>
    initialDraftMode && initialActionViewId?.trim() ? initialActionViewId.trim() : null,
  );
  const [draftActionTitle, setDraftActionTitle] = useState<string | null>(null);
  const [draftActionLoading, setDraftActionLoading] = useState(false);
  const [draftActionError, setDraftActionError] = useState<string | null>(null);
  const [handoffPhase, setHandoffPhase] = useState<ActionHandoffPhase>(() =>
    initialDraftMode && initialActionViewId?.trim() ? "resolving_action" : "idle",
  );
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [messageMetaById, setMessageMetaById] = useState<Record<number, { limitations: string[] }>>({});
  const [inputValue, setInputValue] = useState(() =>
    initialDraftMode && initialActionViewId?.trim() ? ACTION_ASK_MESSAGE : "",
  );
  const [composerAttachments, setComposerAttachments] = useState<ComposerAttachment[]>([]);
  const [removingAllAttachments, setRemovingAllAttachments] = useState(false);
  const [attachmentSelectionError, setAttachmentSelectionError] = useState<string | null>(null);
  const removingLocalIdsRef = useRef(new Set<string>());
  const uploadAbortControllersRef = useRef(new Map<string, AbortController>());
  const uploadGenerationRef = useRef(new Map<string, number>());
  const askAbortControllerRef = useRef<AbortController | null>(null);
  const applyAskResponseRef = useRef<
    ((response: AskChatResponse, previousSessionId: number | null) => Promise<void>) | null
  >(null);
  const [showArchived, setShowArchived] = useState(false);
  const [mobileSessionsOpen, setMobileSessionsOpen] = useState(false);

  const [sessionsInitialLoading, setSessionsInitialLoading] = useState(false);
  const [messagesInitialLoading, setMessagesInitialLoading] = useState(false);
  const [messagesRefreshing, setMessagesRefreshing] = useState(false);
  const [messageSending, setMessageSending] = useState(false);
  const [streamingDraft, setStreamingDraft] = useState<string | null>(null);
  const [archivingSessionId, setArchivingSessionId] = useState<number | null>(null);
  const [renamingSessionId, setRenamingSessionId] = useState<number | null>(null);
  const [deletingSessionId, setDeletingSessionId] = useState<number | null>(null);

  const [sessionsError, setSessionsError] = useState<string | null>(null);
  const [messagesError, setMessagesError] = useState<string | null>(null);
  const [askError, setAskError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  const viewMode = resolveChatViewMode({
    urlSessionId,
    draftActionKey,
    draftPrefill: ACTION_ASK_MESSAGE,
  });
  const viewModeKind = viewMode.kind;
  const existingSessionId = existingSessionIdFromMode(viewMode);

  activeSessionIdRef.current = existingSessionId;

  const { active: activeSessions, archived: archivedSessions } = useMemo(
    () => splitActiveAndArchived(sessions),
    [sessions],
  );
  const visibleSessions = showArchived ? sessions : activeSessions;
  const sessionGroups = useMemo(() => groupSessionsByDate(visibleSessions), [visibleSessions]);

  const selectedSession = useMemo(
    () => (existingSessionId != null ? sessions.find((s) => s.id === existingSessionId) ?? null : null),
    [sessions, existingSessionId],
  );
  const selectedSessionDisplay = useMemo(
    () => (selectedSession ? mapSessionToDisplay(selectedSession) : null),
    [selectedSession],
  );

  const contextSession: ChatSessionDisplay | null =
    viewMode.kind === "draft_action"
      ? {
          id: 0,
          title: draftActionTitle ?? "",
          kind: "action",
          status: "active",
          badgeLabel: "",
          subtitle: "",
          actionViewId: viewMode.actionKey,
          raw: {
            id: 0,
            title: draftActionTitle ?? "",
            action_view_id: viewMode.actionKey,
            status: "active",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        }
      : viewMode.kind === "existing_session"
        ? selectedSessionDisplay
        : null;

  const contextVariant =
    viewMode.kind === "new_general"
      ? "hero"
      : viewMode.kind === "existing_session" && selectedSession && !isActionSession(selectedSession)
        ? "compact"
        : "hero";

  const selectedIsArchived = selectedSession?.status === "archived";
  const showEmptyState = shouldShowEmptyState(viewMode, messages.length, messagesInitialLoading);
  const handoffBusy = isActionHandoffBusy(handoffPhase);
  const lastMessageId = messages.length > 0 ? messages[messages.length - 1]?.id ?? null : null;

  const scrollConversationToBottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior });
  }, []);

  const loadSessions = useCallback(async () => {
    setSessionsInitialLoading(true);
    setSessionsError(null);
    try {
      const data = await getChatSessions({ limit: 50, offset: 0 });
      setSessions(data.items ?? []);
    } catch (e: unknown) {
      setSessions([]);
      setSessionsError(e instanceof Error ? e.message : "Не удалось загрузить сеансы чата");
    } finally {
      setSessionsInitialLoading(false);
    }
  }, []);

  const fetchMessagesForSession = useCallback(
    async (sessionId: number, options?: { background?: boolean; trigger?: string }) => {
      if (inFlightSessionIdsRef.current.has(sessionId)) {
        chatDebug("load messages skipped in-flight", { session_id: sessionId, trigger: options?.trigger });
        return;
      }

      const version = ++messagesRequestVersionRef.current;
      const background = options?.background ?? lastFetchedSessionIdRef.current === sessionId;

      chatDebug("load messages start", {
        session_id: sessionId,
        trigger: options?.trigger ?? "unknown",
        request_version: version,
        initial_loading: !background,
      });

      if (background) {
        setMessagesRefreshing(true);
      } else {
        setMessagesInitialLoading(true);
      }
      setMessagesError(null);
      inFlightSessionIdsRef.current.add(sessionId);

      try {
        const data = await getChatMessages(sessionId, { limit: 200, offset: 0 });

        if (
          isStaleMessagesResponse(
            version,
            messagesRequestVersionRef.current,
            sessionId,
            activeSessionIdRef.current,
          )
        ) {
          chatDebug("load messages ignored as stale", {
            session_id: sessionId,
            request_version: version,
          });
          return;
        }

        setMessages(data.items ?? []);
        lastFetchedSessionIdRef.current = sessionId;
        chatDebug("load messages success", {
          session_id: sessionId,
          request_version: version,
          initial_loading: !background,
        });
      } catch (e: unknown) {
        if (
          isStaleMessagesResponse(
            version,
            messagesRequestVersionRef.current,
            sessionId,
            activeSessionIdRef.current,
          )
        ) {
          return;
        }
        setMessages([]);
        setMessagesError(e instanceof Error ? e.message : "Не удалось загрузить сообщения");
      } finally {
        inFlightSessionIdsRef.current.delete(sessionId);
        if (version === messagesRequestVersionRef.current) {
          setMessagesInitialLoading(false);
          setMessagesRefreshing(false);
        }
      }
    },
    [],
  );

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  // Action → chat handoff. Stable primitive deps only — never depend on `viewMode` object.
  // Duplicate asks are prevented by backend client_request_id idempotency + sessionStorage key
  // + module-level inflight lock (Strict Mode / remount safe).
  useEffect(() => {
    if (!initialDraftMode) return;
    if (urlSessionId != null) {
      setHandoffPhase("ready");
      return;
    }

    let cancelled = false;
    const lockKey = handoffInflightKey({
      actionViewId: initialActionViewId ?? null,
    });

    async function runHandoff() {
      setHandoffPhase("resolving_action");
      setDraftActionLoading(true);
      setDraftActionError(null);
      setAskError(null);
      setInputValue(ACTION_ASK_MESSAGE);

      try {
        const actionViewId = (initialActionViewId ?? "").trim();
        if (!actionViewId) {
          throw new Error("Не указан идентификатор действия для чата");
        }
        let actionTitle: string | null = null;
        try {
          const detail = await getActionView(actionViewId);
          actionTitle = detail.group?.display_title ?? detail.history?.display_title ?? null;
        } catch {
          actionTitle = null;
        }
        if (cancelled) return;
        setDraftActionKey(actionViewId);
        setDraftActionTitle(actionTitle);
        setDraftActionLoading(false);

        if (isLiveReview) {
          setHandoffPhase("ready");
          setInputValue("");
          setStatusMessage("Существующий чат по этому действию не найден. Создание новых ответов в режиме просмотра недоступно.");
          return;
        }

        setHandoffPhase("creating_or_loading_conversation");
        const clientRequestId = getOrCreateActionHandoffRequestId(actionViewId);
        const handoff = await createActionChatHandoff({
          action_view_id: actionViewId,
          client_request_id: clientRequestId,
        });
        if (cancelled) return;
        setDraftActionTitle(handoff.title || actionTitle);
        setHandoffPhase("sending_initial_message");
        setMessageSending(true);
        let response: AskChatResponse | null = null;
        const controller = new AbortController();
        askAbortControllerRef.current = controller;
        setStreamingDraft("");
        await askChatStream({
          session_id: handoff.session_id,
          question: ACTION_ASK_MESSAGE,
          client_request_id: `${clientRequestId}:ask`,
        }, {
          signal: controller.signal,
          onDelta: (delta) => setStreamingDraft((current) => `${current ?? ""}${delta}`),
          onDone: (result) => {
            response = result;
          },
        });
        if (!response) throw new Error("Поток ответа завершился без результата");
        clearActionHandoffRequestId(actionViewId);
        setHandoffPhase("ready");
        await applyAskResponseRef.current?.(response, null);
        setInputValue("");
        setComposerAttachments([]);
        await loadSessions();
      } catch (e: unknown) {
        if (cancelled) return;
        if (e instanceof DOMException && e.name === "AbortError") {
          setHandoffPhase("ready");
          setStatusMessage("Генерация ответа отменена.");
          return;
        }
        const message = e instanceof Error ? e.message : "Не удалось открыть чат по действию";
        setDraftActionError(message);
        setAskError(friendlyChatAskError(message));
        setHandoffPhase("failed");
        setDraftActionLoading(false);
        chatDebug("action handoff failed", { error: message });
      } finally {
        askAbortControllerRef.current = null;
        setStreamingDraft(null);
        setMessageSending(false);
      }
    }

    void withActionHandoffLock(lockKey, runHandoff);

    return () => {
      cancelled = true;
      askAbortControllerRef.current?.abort();
    };
    // applyAskResponse / loadSessions / router are stable enough for this mount-bound handoff.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialDraftMode, initialActionViewId, urlSessionId, isLiveReview]);

  // Title refresh for draft/action context — keyed by stable actionKey string only.
  useEffect(() => {
    if (viewModeKind !== "draft_action" || !draftActionKey) {
      return;
    }
    if (draftActionTitle) {
      return;
    }

    let cancelled = false;
    setDraftActionLoading(true);
    setDraftActionError(null);

    void getActionView(draftActionKey)
      .then((detail) => {
        if (cancelled) return;
        setDraftActionTitle(detail.group?.display_title ?? detail.history?.display_title ?? draftActionKey);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setDraftActionError(e instanceof Error ? e.message : "Не удалось загрузить действие");
      })
      .finally(() => {
        if (!cancelled) setDraftActionLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [viewModeKind, draftActionKey, draftActionTitle]);

  useEffect(() => {
    chatDebug("session mode changed", {
      kind: viewModeKind,
      session_id: existingSessionId,
    });

    if (shouldClearVisibleMessages(lastFetchedSessionIdRef.current, existingSessionId, viewModeKind)) {
      if (viewModeKind !== "existing_session") {
        setMessages([]);
        lastFetchedSessionIdRef.current = null;
      } else if (lastFetchedSessionIdRef.current !== existingSessionId) {
        setMessages([]);
      }
    }

    if (!shouldFetchMessagesForSession(viewModeKind, existingSessionId, lastFetchedSessionIdRef.current)) {
      return;
    }

    const sessionId = existingSessionId as number;
    const controller = new AbortController();
    const version = ++messagesRequestVersionRef.current;

    chatDebug("load messages start", {
      session_id: sessionId,
      trigger: "session_id_changed",
      request_version: version,
      initial_loading: true,
    });

    setMessagesInitialLoading(true);
    setMessagesError(null);
    inFlightSessionIdsRef.current.add(sessionId);

    void getChatMessages(sessionId, { limit: 200, offset: 0, signal: controller.signal })
      .then((data) => {
        if (
          isStaleMessagesResponse(
            version,
            messagesRequestVersionRef.current,
            sessionId,
            activeSessionIdRef.current,
          )
        ) {
          chatDebug("load messages ignored as stale", {
            session_id: sessionId,
            request_version: version,
          });
          return;
        }
        setMessages(data.items ?? []);
        lastFetchedSessionIdRef.current = sessionId;
        chatDebug("load messages success", {
          session_id: sessionId,
          request_version: version,
          initial_loading: true,
        });
      })
      .catch((e: unknown) => {
        if (e instanceof DOMException && e.name === "AbortError") {
          chatDebug("load messages aborted", { session_id: sessionId, request_version: version });
          return;
        }
        if (
          isStaleMessagesResponse(
            version,
            messagesRequestVersionRef.current,
            sessionId,
            activeSessionIdRef.current,
          )
        ) {
          return;
        }
        setMessages([]);
        setMessagesError(e instanceof Error ? e.message : "Не удалось загрузить сообщения");
      })
      .finally(() => {
        inFlightSessionIdsRef.current.delete(sessionId);
        if (version === messagesRequestVersionRef.current) {
          setMessagesInitialLoading(false);
        }
      });

    return () => {
      controller.abort();
    };
  }, [viewModeKind, existingSessionId]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const onScroll = () => {
      const threshold = 120;
      isNearBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    if (messagesInitialLoading) return;
    if (!messageSending && !isNearBottomRef.current) return;
    const frame = requestAnimationFrame(() => {
      scrollConversationToBottom(messageSending ? "smooth" : "auto");
    });
    return () => cancelAnimationFrame(frame);
  }, [lastMessageId, messagesInitialLoading, messageSending, scrollConversationToBottom]);

  const applyAskResponse = useCallback(
    async (response: AskChatResponse, previousSessionId: number | null) => {
      setDraftActionKey(null);
      setDraftActionTitle(null);
      setDraftActionLoading(false);
      setDraftActionError(null);
      setHandoffPhase("ready");

      if (previousSessionId !== response.session_id) {
        chatDebug("router session change", {
          from_session_id: previousSessionId,
          to_session_id: response.session_id,
        });
        router.replace(`/app/chat?session=${response.session_id}`);
      }

      if (response.assistant_message_id != null) {
        setMessageMetaById((prev) => ({
          ...prev,
          [response.assistant_message_id as number]: {
            limitations: response.limitations ?? [],
          },
        }));
      }

      if (previousSessionId === response.session_id) {
        await fetchMessagesForSession(response.session_id, {
          background: true,
          trigger: "ask_response_same_session",
        });
      }
    },
    [fetchMessagesForSession, router],
  );
  applyAskResponseRef.current = applyAskResponse;

  const uploadComposerFile = useCallback(async (localId: string, file: File) => {
    const generation = uploadGenerationRef.current.get(localId) ?? 0;
    const controller = new AbortController();
    uploadAbortControllersRef.current.set(localId, controller);

    setComposerAttachments((prev) =>
      prev.map((item) =>
        item.localId === localId ? { ...item, status: "uploading", error: undefined, removalError: undefined } : item,
      ),
    );

    try {
      const response = await uploadChatAttachment(file, { signal: controller.signal });
      if ((uploadGenerationRef.current.get(localId) ?? 0) !== generation) {
        if (response.attachment.id) {
          try {
            await deleteChatAttachment(response.attachment.id);
          } catch {
            // Best-effort cleanup for cancelled upload.
          }
        }
        return;
      }

      setComposerAttachments((prev) => {
        const current = prev.find((item) => item.localId === localId);
        if (!current || (uploadGenerationRef.current.get(localId) ?? 0) !== generation) {
          return prev;
        }
        return prev.map((item) =>
          item.localId === localId
            ? {
                ...item,
                status: "uploaded",
                attachmentId: response.attachment.id,
                name: response.attachment.name || item.name,
                mimeType: response.attachment.mime_type,
                sizeBytes: response.attachment.size_bytes ?? item.sizeBytes,
                warningCode: response.attachment.warning_code ?? completenessWarningCode(response.attachment.completeness),
                completeness: response.attachment.completeness,
                extractionStatus: response.attachment.extraction_status,
                error: undefined,
                removalError: undefined,
              }
            : item,
        );
      });
    } catch (e: unknown) {
      if ((uploadGenerationRef.current.get(localId) ?? 0) !== generation) {
        return;
      }
      if (e instanceof DOMException && e.name === "AbortError") {
        return;
      }
      setComposerAttachments((prev) => {
        const current = prev.find((item) => item.localId === localId);
        if (!current || (uploadGenerationRef.current.get(localId) ?? 0) !== generation) {
          return prev;
        }
        return prev.map((item) =>
          item.localId === localId
            ? {
                ...item,
                status: "error",
                error: e instanceof Error ? e.message : "Не удалось загрузить файл. Повторите попытку.",
              }
            : item,
        );
      });
    } finally {
      uploadAbortControllersRef.current.delete(localId);
    }
  }, []);

  const cancelLocalAttachmentUpload = useCallback((localId: string) => {
    uploadGenerationRef.current.set(localId, (uploadGenerationRef.current.get(localId) ?? 0) + 1);
    uploadAbortControllersRef.current.get(localId)?.abort();
    uploadAbortControllersRef.current.delete(localId);
  }, []);

  const handlePickFiles = useCallback(
    (files: FileList) => {
      const selected = Array.from(files);
      if (selected.length === 0) return;

      const validation = validateAttachmentBatchSelection({
        selectedCount: selected.length,
        existingCount: composerAttachments.length,
      });

      if (!validation.ok) {
        setAttachmentSelectionError(validation.message);
        return;
      }

      setAttachmentSelectionError(null);

      for (const file of selected) {
        const localId = newLocalId();
        uploadGenerationRef.current.set(localId, 0);
        setComposerAttachments((prev) => [
          ...prev,
          {
            localId,
            name: file.name,
            sizeBytes: file.size,
            mimeType: file.type || undefined,
            status: "local_selected" as const,
            file,
          },
        ]);
        void uploadComposerFile(localId, file);
      }
    },
    [composerAttachments.length, uploadComposerFile],
  );

  const handleRemoveAttachment = useCallback(
    async (localId: string) => {
      if (removingLocalIdsRef.current.has(localId)) return;

      const target = composerAttachments.find((item) => item.localId === localId);
      if (!target) return;

      if (!target.attachmentId) {
        cancelLocalAttachmentUpload(localId);
        setComposerAttachments((prev) => prev.filter((item) => item.localId !== localId));
        return;
      }

      removingLocalIdsRef.current.add(localId);
      setComposerAttachments((prev) =>
        prev.map((item) =>
          item.localId === localId
            ? { ...item, status: "removing", removalError: undefined, error: undefined }
            : item,
        ),
      );

      try {
        await deleteChatAttachment(target.attachmentId);
        setComposerAttachments((prev) => prev.filter((item) => item.localId !== localId));
      } catch (error: unknown) {
        setComposerAttachments((prev) =>
          prev.map((item) =>
            item.localId === localId
              ? {
                  ...item,
                  status: "uploaded",
                  removalError: mapAttachmentRemovalError(error),
                }
              : item,
          ),
        );
      } finally {
        removingLocalIdsRef.current.delete(localId);
      }
    },
    [cancelLocalAttachmentUpload, composerAttachments],
  );

  const handleRetryAttachment = useCallback(
    (localId: string) => {
      const target = composerAttachments.find((item) => item.localId === localId);
      if (!target?.file) return;
      uploadGenerationRef.current.set(localId, 0);
      void uploadComposerFile(localId, target.file);
    },
    [composerAttachments, uploadComposerFile],
  );

  const handleRemoveAllAttachments = useCallback(async () => {
    if (removingAllAttachments) return;
    const snapshot = [...composerAttachments];
    if (snapshot.length === 0) return;

    setRemovingAllAttachments(true);

    try {
      for (const item of snapshot) {
        if (item.status === "uploading" || item.status === "local_selected") {
          cancelLocalAttachmentUpload(item.localId);
        }
      }

      const serverBacked = snapshot.filter((item) => item.attachmentId);
      const removeImmediately = new Set(
        snapshot
          .filter((item) => !item.attachmentId || item.status === "uploading" || item.status === "local_selected")
          .map((item) => item.localId),
      );

      setComposerAttachments((prev) =>
        prev
          .filter((item) => !removeImmediately.has(item.localId))
          .map((item) =>
            serverBacked.some((source) => source.localId === item.localId)
              ? { ...item, status: "removing" as const, removalError: undefined }
              : item,
          ),
      );

      const failed: ComposerAttachment[] = [];
      await Promise.allSettled(
        serverBacked.map(async (item) => {
          try {
            await deleteChatAttachment(item.attachmentId as string);
          } catch (error: unknown) {
            failed.push({
              ...item,
              status: "uploaded",
              removalError: mapAttachmentRemovalError(error),
            });
          }
        }),
      );

      setComposerAttachments(failed);
    } finally {
      setRemovingAllAttachments(false);
    }
  }, [cancelLocalAttachmentUpload, composerAttachments, removingAllAttachments]);

  const sendQuestion = useCallback(
    async (question: string) => {
      const q = question.trim();
      if (!q || messageSending || selectedIsArchived || submitLockRef.current) return;
      if (
        composerAttachments.some(
          (item) =>
            item.status === "uploading" ||
            item.status === "local_selected" ||
            item.status === "error" ||
            item.status === "removing" ||
            Boolean(item.removalError),
        )
      ) {
        setAskError("Дождитесь завершения загрузки файлов или удалите проблемные вложения.");
        return;
      }

      submitLockRef.current = true;
      setMessageSending(true);
      setAskError(null);
      setStatusMessage(null);
      const clientRequestId = newLocalId();
      const previousSessionId = existingSessionId;
      const controller = new AbortController();
      askAbortControllerRef.current = controller;
      setStreamingDraft("");
      try {
        const attachmentIds = composerAttachments
          .filter((item) => item.status === "uploaded" && item.attachmentId)
          .map((item) => item.attachmentId as string);

        let response: AskChatResponse | null = null;
        await askChatStream({
          session_id: viewMode.kind === "existing_session" ? viewMode.sessionId : undefined,
          question: q,
          attachment_ids: attachmentIds,
          client_request_id: clientRequestId,
        }, {
          signal: controller.signal,
          onDelta: (delta) => setStreamingDraft((current) => `${current ?? ""}${delta}`),
          onDone: (result) => {
            response = result;
          },
        });
        if (!response) throw new Error("Поток ответа завершился без результата");
        await applyAskResponse(response, previousSessionId);
        setInputValue("");
        setComposerAttachments([]);
        setAttachmentSelectionError(null);
        await loadSessions();
      } catch (e: unknown) {
        if (e instanceof DOMException && e.name === "AbortError") {
          setStatusMessage("Генерация ответа отменена.");
          return;
        }
        const raw = e instanceof Error ? e.message : "Не удалось отправить вопрос";
        setAskError(friendlyChatAskError(raw));
      } finally {
        askAbortControllerRef.current = null;
        setStreamingDraft(null);
        setMessageSending(false);
        submitLockRef.current = false;
      }
    },
    [
      applyAskResponse,
      composerAttachments,
      existingSessionId,
      loadSessions,
      messageSending,
      selectedIsArchived,
      viewMode,
    ],
  );

  function handleNewChat() {
    setDraftActionKey(null);
    setDraftActionTitle(null);
    setDraftActionLoading(false);
    setDraftActionError(null);
    setHandoffPhase("idle");
    setMessages([]);
    lastFetchedSessionIdRef.current = null;
    setInputValue("");
    setComposerAttachments([]);
    setAttachmentSelectionError(null);
    setAskError(null);
    setMessagesError(null);
    setStatusMessage(null);
    setMessageMetaById({});
    chatDebug("router session change", { from_session_id: urlSessionId, to_session_id: null });
    router.replace("/app/chat");
  }

  function handleSelectSession(id: number) {
    if (urlSessionId === id) return;
    setDraftActionKey(null);
    setDraftActionTitle(null);
    setDraftActionLoading(false);
    setDraftActionError(null);
    setHandoffPhase("ready");
    setComposerAttachments([]);
    setAttachmentSelectionError(null);
    chatDebug("router session change", { from_session_id: urlSessionId, to_session_id: id });
    router.replace(`/app/chat?session=${id}`);
  }

  async function handleArchiveSession(sessionId: number) {
    setArchivingSessionId(sessionId);
    setSessionsError(null);
    try {
      await archiveChatSession(sessionId);
      await loadSessions();
      if (urlSessionId === sessionId) {
        handleNewChat();
      }
    } catch (e: unknown) {
      setSessionsError(e instanceof Error ? e.message : "Не удалось архивировать сеанс");
    } finally {
      setArchivingSessionId(null);
    }
  }

  async function handleRenameSession(sessionId: number, title: string) {
    setRenamingSessionId(sessionId);
    setSessionsError(null);
    try {
      await renameChatSession(sessionId, title);
      await loadSessions();
    } catch (e: unknown) {
      setSessionsError(e instanceof Error ? e.message : "Не удалось переименовать чат");
    } finally {
      setRenamingSessionId(null);
    }
  }

  async function handleDeleteSession(sessionId: number) {
    setDeletingSessionId(sessionId);
    setSessionsError(null);
    try {
      await deleteChatSession(sessionId);
      await loadSessions();
      if (urlSessionId === sessionId) {
        handleNewChat();
      }
    } catch (e: unknown) {
      setSessionsError(e instanceof Error ? e.message : "Не удалось удалить чат");
    } finally {
      setDeletingSessionId(null);
    }
  }

  function handlePromptChipClick(prompt: string) {
    if (messageSending || selectedIsArchived) return;
    setInputValue((current) => applyPromptChip(current, prompt));
  }

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-[#F8FBFF] px-3 pb-3 pt-3 sm:px-6 sm:pb-5 sm:pt-5 lg:px-8">
      <div className="shrink-0 pb-3 sm:pb-4">
        <div className="flex items-start justify-between gap-3">
          <PageHeader
            title="AI-чат"
            subtitle="Задавайте вопросы о продажах, остатках, рекламе и рекомендациях — Aivion ответит на основе данных вашего кабинета."
            className="min-w-0 flex-1 pb-0 [&_p]:hidden sm:[&_p]:block"
          />
          <button
            type="button"
            className="inline-flex h-10 shrink-0 items-center rounded-xl border border-[#E2EAF5] bg-white px-3 text-sm font-medium text-[#0F172A] shadow-aivion-sm lg:hidden"
            aria-expanded={mobileSessionsOpen}
            aria-controls="chat-session-drawer"
            onClick={() => setMobileSessionsOpen((v) => !v)}
          >
            Чаты
          </button>
        </div>
      </div>

      <section className="grid min-h-0 flex-1 grid-cols-1 gap-4 overflow-hidden lg:grid-cols-[280px_minmax(0,1fr)] lg:gap-6 xl:grid-cols-[300px_minmax(0,1fr)]">
        {/* Desktop session sidebar */}
        <div className="hidden min-h-0 flex-col overflow-hidden rounded-[18px] border border-[#E2EAF5] bg-white p-4 shadow-aivion-sm lg:flex lg:max-h-full">
          <ChatSessionList
            groups={sessionGroups}
            archivedCount={archivedSessions.length}
            showArchived={showArchived}
            onToggleArchived={() => setShowArchived((value) => !value)}
            viewMode={viewMode}
            selectedSessionId={urlSessionId}
            onNewChat={handleNewChat}
            onSelect={handleSelectSession}
            onArchive={(id) => void handleArchiveSession(id)}
            onRename={(id, title) => void handleRenameSession(id, title)}
            onDelete={(id) => void handleDeleteSession(id)}
            archivingSessionId={archivingSessionId}
            renamingSessionId={renamingSessionId}
            deletingSessionId={deletingSessionId}
            loading={sessionsInitialLoading}
            error={sessionsError}
          />
        </div>

        {/* Mobile session drawer — does not push composer below the fold */}
        {mobileSessionsOpen ? (
          <div className="fixed inset-0 z-40 lg:hidden" role="dialog" aria-modal="true">
            <button
              type="button"
              className="absolute inset-0 bg-black/35"
              aria-label="Закрыть список чатов"
              onClick={() => setMobileSessionsOpen(false)}
            />
            <div
              id="chat-session-drawer"
              className="absolute inset-y-0 left-0 flex w-[min(92vw,320px)] flex-col overflow-hidden bg-white p-4 shadow-xl"
            >
              <div className="mb-3 flex items-center justify-between">
                <p className="text-sm font-semibold text-[#0F172A]">Чаты</p>
                <button
                  type="button"
                  className="rounded-lg px-2 py-1 text-sm text-[#64748B]"
                  onClick={() => setMobileSessionsOpen(false)}
                >
                  Закрыть
                </button>
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto">
                <ChatSessionList
                  groups={sessionGroups}
                  archivedCount={archivedSessions.length}
                  showArchived={showArchived}
                  onToggleArchived={() => setShowArchived((value) => !value)}
                  viewMode={viewMode}
                  selectedSessionId={urlSessionId}
                  onNewChat={() => {
                    setMobileSessionsOpen(false);
                    handleNewChat();
                  }}
                  onSelect={(id) => {
                    setMobileSessionsOpen(false);
                    handleSelectSession(id);
                  }}
                  onArchive={(id) => void handleArchiveSession(id)}
                  onRename={(id, title) => void handleRenameSession(id, title)}
                  onDelete={(id) => void handleDeleteSession(id)}
                  archivingSessionId={archivingSessionId}
                  renamingSessionId={renamingSessionId}
                  deletingSessionId={deletingSessionId}
                  loading={sessionsInitialLoading}
                  error={sessionsError}
                />
              </div>
            </div>
          </div>
        ) : null}

        <section className="flex min-h-0 min-w-0 flex-col gap-2 overflow-hidden">
          <div className="shrink-0">
            <ChatContextCard
              session={contextSession}
              variant={contextVariant}
              actionHeaderLoading={
                (viewMode.kind === "draft_action" && (draftActionLoading || handoffBusy)) ||
                handoffPhase === "resolving_action"
              }
              actionHeaderError={
                viewMode.kind === "draft_action" || handoffPhase === "failed"
                  ? draftActionError
                  : null
              }
            />
          </div>

          {(askError || messagesError || statusMessage) && (
            <div className="shrink-0 space-y-1 rounded-xl border border-[#E2EAF5] bg-white px-4 py-2 text-sm">
              {askError ? (
                <p className="text-aivion-warning" role="alert">
                  {askError}
                </p>
              ) : null}
              {messagesError ? <p className="text-aivion-danger">{messagesError}</p> : null}
              {statusMessage ? <p className="text-aivion-success">{statusMessage}</p> : null}
            </div>
          )}

          <div
            ref={scrollRef}
            className="relative min-h-0 flex-1 overflow-y-auto bg-[#F8FBFF] px-1 py-1 sm:px-2"
          >
            {messagesRefreshing ? (
              <div className="pointer-events-none absolute right-3 top-2 z-10 rounded-full border border-[#E2EAF5] bg-white/95 px-2.5 py-1 text-[11px] text-[#64748B] shadow-aivion-sm">
                Обновление…
              </div>
            ) : null}

            <div className="mx-auto w-full max-w-[1040px] space-y-5 px-3 py-4 sm:px-5 sm:py-5">
              {messagesInitialLoading && messages.length === 0 ? (
                <ChatMessagesSkeleton />
              ) : handoffBusy && messages.length === 0 ? (
                <div className="py-10 text-center" aria-busy="true" aria-live="polite">
                  <p className="text-sm font-medium text-[#0F172A]">
                    {handoffPhase === "sending_initial_message" || handoffPhase === "attaching_context"
                      ? "Отправляем стартовый запрос по действию…"
                      : "Открываем чат по действию…"}
                  </p>
                  <p className="mt-2 text-sm text-[#64748B]">
                    Контекст действия подключается один раз. Это не займёт много времени.
                  </p>
                </div>
              ) : showEmptyState ? (
                <ChatEmptyState
                  disabled={messageSending || selectedIsArchived}
                  onChipClick={handlePromptChipClick}
                />
              ) : messages.length === 0 ? (
                <p className="py-8 text-center text-sm text-[#64748B]">
                  {viewMode.kind === "draft_action"
                    ? handoffPhase === "failed"
                      ? "Не удалось автоматически начать диалог. Отредактируйте вопрос и отправьте ещё раз."
                      : "Отредактируйте вопрос и отправьте, когда будете готовы."
                    : "В этом чате пока нет сообщений. Задайте вопрос ниже."}
                </p>
              ) : (
                messages.map((m) => {
                  if (m.message_type === "meta") return null;

                  if (m.role === "user") {
                    return (
                      <ChatUserMessage
                        key={m.id}
                        content={m.content}
                        createdAt={m.created_at}
                        attachments={(m.attachments ?? []).map(mapAttachmentToDisplay)}
                      />
                    );
                  }

                  if (m.role === "assistant") {
                    return (
                      <ChatAssistantMessage
                        key={m.id}
                        message={m}
                        meta={messageMetaById[m.id]}
                        onCopyError={setStatusMessage}
                      />
                    );
                  }

                  return (
                    <div
                      key={m.id}
                      className="rounded-[16px] border border-[#E6EEF8] bg-white px-4 py-3 text-sm text-[#64748B]"
                    >
                      {m.content}
                    </div>
                  );
                })
              )}

              {messageSending ? (
                <div className="flex justify-start">
                  <div className="w-full max-w-[780px]">
                    <div className="mb-2 flex items-center gap-2">
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-aivion-blue text-xs font-bold text-white">
                        A
                      </div>
                      <span className="text-sm font-semibold text-[#0F172A]">Aivion</span>
                    </div>
                    <div className="rounded-[18px] border border-[#E6EEF8] bg-white p-5 shadow-aivion-sm">
                      <div className="flex items-center gap-2 text-sm text-[#64748B]">
                        <span
                          className="inline-flex h-2 w-2 animate-pulse rounded-full bg-aivion-blue"
                          aria-hidden
                        />
                        {streamingDraft ? streamingDraft : "ИИ готовит ответ…"}
                      </div>
                    </div>
                  </div>
                </div>
              ) : null}
            </div>
          </div>

          <div className="shrink-0 space-y-3 border-t border-[#E2EAF5]/80 bg-[#F8FBFF] pt-3">
            {selectedIsArchived ? (
              <p className="text-sm text-[#64748B]">
                Этот чат в архиве. Начните новый чат, чтобы задать ещё вопрос.
              </p>
            ) : null}

            <ChatComposer
              value={inputValue}
              onChange={setInputValue}
              onSubmit={() => void sendQuestion(inputValue)}
              onCancel={() => askAbortControllerRef.current?.abort()}
              disabled={selectedIsArchived || handoffBusy}
              sending={messageSending || handoffBusy}
              isEmptyChat={viewMode.kind === "new_general"}
              attachments={composerAttachments}
              onPickFiles={handlePickFiles}
              onRemoveAttachment={(localId) => void handleRemoveAttachment(localId)}
              onRetryAttachment={handleRetryAttachment}
              onRemoveAllAttachments={() => void handleRemoveAllAttachments()}
              removingAll={removingAllAttachments}
              attachmentSelectionError={attachmentSelectionError}
              onDismissAttachmentSelectionError={() => setAttachmentSelectionError(null)}
            />
          </div>
        </section>
      </section>
    </div>
  );
}
