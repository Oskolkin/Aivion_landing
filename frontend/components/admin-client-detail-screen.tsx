"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { buttonClassNames } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import {
  getAdminChatMessages,
  getAdminChatSessions,
  getAdminChatTraceDetail,
  getAdminChatTraces,
  getAdminClientBilling,
  getAdminClientChatFeedback,
  getAdminClientDetail,
  getAdminImportErrors,
  getAdminImportJobs,
  getAdminMe,
  getAdminRecommendationFeedback,
  getAdminRecommendationDetail,
  getAdminRecommendationRunDetail,
  getAdminRecommendationRuns,
  getAdminSyncCursors,
  getAdminSyncJobs,
  rerunAdminAlerts,
  rerunAdminMetrics,
  rerunAdminRecommendations,
  rerunAdminSync,
  resetAdminCursor,
  type AdminActionExecutionResult,
  type AdminActionLog,
  type AdminBillingState,
  type AdminChatMessageItem,
  type AdminChatSessionItem,
  type AdminChatTraceDetail,
  type AdminChatTraceItem,
  type AdminClientDetail,
  type AdminImportErrorItem,
  type AdminImportJobItem,
  type AdminRecommendationDiagnosticItem,
  type AdminRecommendationFeedbackResponse,
  type AdminRecommendationRawAIDetail,
  type AdminRecommendationRunDetail,
  type AdminRecommendationRunItem,
  type AdminSyncCursorItem,
  type AdminSyncJobItem,
} from "@/lib/admin-api";

type Props = { sellerAccountId: number };
type AdminTab =
  | "overview"
  | "sync_import"
  | "cursors"
  | "alerts"
  | "recommendations"
  | "chat_logs"
  | "feedback"
  | "billing"
  | "actions";
type AdminActionKey =
  | "rerun_sync"
  | "reset_cursor"
  | "rerun_metrics"
  | "rerun_alerts"
  | "rerun_recommendations";

function importJobFailureStats(detail: AdminClientDetail): { failedJobs: number; sumFailedRecords: number } {
  const jobs = detail.operational_status.latest_import_jobs ?? [];
  const failedJobs = jobs.filter((j) => (j.status || "").toLowerCase() === "failed").length;
  const sumFailed = jobs.reduce((a, j) => a + (j.records_failed ?? 0), 0);
  return { failedJobs, sumFailedRecords: sumFailed };
}

function jsonTitleNeedsRawAiAuditCopy(title: string): boolean {
  const t = title.toLowerCase();
  return (
    t.includes("raw openai") ||
    t.includes("raw ai response") ||
    t.includes("raw planner") ||
    t.includes("raw answer") ||
    t.includes("сырой ответ openai") ||
    t.includes("сырой ответ ии") ||
    t.includes("сырой ответ планировщика") ||
    t.includes("сырой ответ ассистента")
  );
}

export default function AdminClientDetailScreen({ sellerAccountId }: Props) {
  const [adminReady, setAdminReady] = useState<"loading" | "allowed" | "forbidden" | "error">("loading");
  const [activeTab, setActiveTab] = useState<AdminTab>("overview");
  const [detail, setDetail] = useState<AdminClientDetail | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [syncJobs, setSyncJobs] = useState<AdminSyncJobItem[]>([]);
  const [importJobs, setImportJobs] = useState<AdminImportJobItem[]>([]);
  const [importErrors, setImportErrors] = useState<AdminImportErrorItem[]>([]);
  const [cursors, setCursors] = useState<AdminSyncCursorItem[]>([]);
  const [runs, setRuns] = useState<AdminRecommendationRunItem[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);
  const [selectedRun, setSelectedRun] = useState<AdminRecommendationRunDetail | null>(null);
  const [selectedRunLoading, setSelectedRunLoading] = useState(false);
  const [selectedRunError, setSelectedRunError] = useState<string | null>(null);
  const [selectedRecommendationDetail, setSelectedRecommendationDetail] = useState<AdminRecommendationRawAIDetail | null>(null);
  const [selectedRecommendationLoading, setSelectedRecommendationLoading] = useState(false);
  const [selectedRecommendationError, setSelectedRecommendationError] = useState<string | null>(null);
  const [traces, setTraces] = useState<AdminChatTraceItem[]>([]);
  const [selectedTraceId, setSelectedTraceId] = useState<number | null>(null);
  const [selectedTrace, setSelectedTrace] = useState<AdminChatTraceDetail | null>(null);
  const [selectedTraceLoading, setSelectedTraceLoading] = useState(false);
  const [selectedTraceError, setSelectedTraceError] = useState<string | null>(null);
  const [sessions, setSessions] = useState<AdminChatSessionItem[]>([]);
  const [messages, setMessages] = useState<AdminChatMessageItem[]>([]);
  const [selectedSessionId, setSelectedSessionId] = useState<number | null>(null);
  const [chatFeedback, setChatFeedback] = useState<Array<Record<string, unknown>>>([]);
  const [recFeedback, setRecFeedback] = useState<AdminRecommendationFeedbackResponse | null>(null);
  const [billing, setBilling] = useState<AdminBillingState | null>(null);
  const [billingMissing, setBillingMissing] = useState(false);
  const [tabLoading, setTabLoading] = useState(false);
  const [tabError, setTabError] = useState<string | null>(null);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runsError, setRunsError] = useState<string | null>(null);
  const [tracesLoading, setTracesLoading] = useState(false);
  const [tracesError, setTracesError] = useState<string | null>(null);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionsError, setSessionsError] = useState<string | null>(null);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [messagesError, setMessagesError] = useState<string | null>(null);
  const [traceStatusFilter, setTraceStatusFilter] = useState("");
  const [traceIntentFilter, setTraceIntentFilter] = useState("");
  const [traceSessionFilter, setTraceSessionFilter] = useState("");

  const [actionLoading, setActionLoading] = useState<Record<AdminActionKey, boolean>>({
    rerun_sync: false,
    reset_cursor: false,
    rerun_metrics: false,
    rerun_alerts: false,
    rerun_recommendations: false,
  });
  const [actionSuccess, setActionSuccess] = useState<Record<AdminActionKey, string | null>>({
    rerun_sync: null,
    reset_cursor: null,
    rerun_metrics: null,
    rerun_alerts: null,
    rerun_recommendations: null,
  });
  const [actionErrors, setActionErrors] = useState<Record<AdminActionKey, string | null>>({
    rerun_sync: null,
    reset_cursor: null,
    rerun_metrics: null,
    rerun_alerts: null,
    rerun_recommendations: null,
  });
  const [actionResults, setActionResults] = useState<Record<AdminActionKey, AdminActionLog | null>>({
    rerun_sync: null,
    reset_cursor: null,
    rerun_metrics: null,
    rerun_alerts: null,
    rerun_recommendations: null,
  });
  const [actionRequestPayloads, setActionRequestPayloads] = useState<Record<AdminActionKey, Record<string, unknown> | null>>({
    rerun_sync: null,
    reset_cursor: null,
    rerun_metrics: null,
    rerun_alerts: null,
    rerun_recommendations: null,
  });
  const [syncType, setSyncType] = useState("initial_sync");
  const [cursorDomain, setCursorDomain] = useState("orders");
  const [cursorType, setCursorType] = useState("source_cursor");
  const [cursorValue, setCursorValue] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [asOfDate, setAsOfDate] = useState("");

  useEffect(() => {
    setAdminReady("loading");
    setError(null);
    void getAdminMe()
      .then(() => setAdminReady("allowed"))
      .catch((e: unknown) => {
        if (e instanceof Error && e.message.toLowerCase().includes("forbidden")) {
          setAdminReady("forbidden");
          return;
        }
        setAdminReady("error");
        setError(e instanceof Error ? e.message : "Не удалось проверить доступ администратора");
      });
  }, []);

  useEffect(() => {
    if (adminReady !== "allowed") return;
    setLoadingDetail(true);
    setError(null);
    void getAdminClientDetail(sellerAccountId)
      .then((res) => setDetail(res))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Не удалось загрузить клиента"))
      .finally(() => setLoadingDetail(false));
  }, [adminReady, sellerAccountId]);

  useEffect(() => {
    if (adminReady !== "allowed" || !detail) return;
    setTabLoading(true);
    setTabError(null);
    const load = async () => {
      if (activeTab === "sync_import") {
        const [sj, ij, ie] = await Promise.all([
          getAdminSyncJobs(sellerAccountId, { limit: 20 }),
          getAdminImportJobs(sellerAccountId, { limit: 20 }),
          getAdminImportErrors(sellerAccountId, { limit: 20 }),
        ]);
        setSyncJobs(sj.items);
        setImportJobs(ij.items);
        setImportErrors(ie.items);
      } else if (activeTab === "cursors") {
        const data = await getAdminSyncCursors(sellerAccountId, { limit: 50 });
        setCursors(data.items);
      } else if (activeTab === "recommendations") {
        setRunsLoading(true);
        setRunsError(null);
        try {
          const data = await getAdminRecommendationRuns(sellerAccountId, { limit: 20 });
          setRuns(data.items);
        } catch (e: unknown) {
          setRuns([]);
          setRunsError(e instanceof Error ? e.message : "Не удалось загрузить прогоны рекомендаций");
        } finally {
          setRunsLoading(false);
        }
      } else if (activeTab === "chat_logs") {
        setTracesLoading(true);
        setTracesError(null);
        setSessionsLoading(true);
        setSessionsError(null);
        try {
          const [tr, ss] = await Promise.all([
            getAdminChatTraces(sellerAccountId, {
              limit: 20,
              status: traceStatusFilter || undefined,
              intent: traceIntentFilter || undefined,
              session_id: traceSessionFilter ? Number(traceSessionFilter) : undefined,
            }),
            getAdminChatSessions(sellerAccountId, { limit: 20 }),
          ]);
          setTraces(tr.items);
          setSessions(ss.items);
        } catch (e: unknown) {
          const msg = e instanceof Error ? e.message : "Не удалось загрузить диагностику чата";
          setTracesError(msg);
          setSessionsError(msg);
          setTraces([]);
          setSessions([]);
        } finally {
          setTracesLoading(false);
          setSessionsLoading(false);
        }
      } else if (activeTab === "feedback") {
        const [cf, rf] = await Promise.all([
          getAdminClientChatFeedback(sellerAccountId, { limit: 20 }),
          getAdminRecommendationFeedback(sellerAccountId, { limit: 20 }),
        ]);
        setChatFeedback(cf.items);
        setRecFeedback(rf);
      } else if (activeTab === "billing") {
        try {
          const b = await getAdminClientBilling(sellerAccountId);
          setBilling(b);
          setBillingMissing(false);
        } catch (e: unknown) {
          if (e instanceof Error && e.message.toLowerCase().includes("not found")) {
            setBilling(null);
            setBillingMissing(true);
          } else {
            throw e;
          }
        }
      }
    };
    void load()
      .catch((e: unknown) => setTabError(e instanceof Error ? e.message : "Не удалось загрузить вкладку"))
      .finally(() => setTabLoading(false));
  }, [activeTab, adminReady, detail, sellerAccountId, traceIntentFilter, traceSessionFilter, traceStatusFilter]);

  async function runAction(
    key: AdminActionKey,
    requestPayload: Record<string, unknown>,
    fn: () => Promise<AdminActionExecutionResult>,
  ) {
    setActionLoading((prev) => ({ ...prev, [key]: true }));
    setActionSuccess((prev) => ({ ...prev, [key]: null }));
    setActionErrors((prev) => ({ ...prev, [key]: null }));
    setActionRequestPayloads((prev) => ({ ...prev, [key]: requestPayload }));
    try {
      const result = await fn();
      setActionResults((prev) => ({ ...prev, [key]: result.action }));
      if (result.ok) {
        setActionSuccess((prev) => ({
          ...prev,
          [key]: `Действие поставлено в очередь/выполнено. Статус: ${result.action?.status ?? "unknown"}`,
        }));
      } else {
        setActionErrors((prev) => ({ ...prev, [key]: result.error ?? "Действие не выполнено" }));
      }
      await maybeRefreshAfterAction(key, result);
    } catch (e: unknown) {
      setActionErrors((prev) => ({
        ...prev,
        [key]: e instanceof Error ? e.message : "Действие не выполнено",
      }));
    } finally {
      setActionLoading((prev) => ({ ...prev, [key]: false }));
    }
  }

  async function maybeRefreshAfterAction(key: AdminActionKey, result: AdminActionExecutionResult) {
    if (!result.ok) return;
    const refreshDetail: AdminActionKey[] = ["rerun_sync", "rerun_metrics", "rerun_alerts", "rerun_recommendations"];
    if (refreshDetail.includes(key)) {
      try {
        const updated = await getAdminClientDetail(sellerAccountId);
        setDetail(updated);
      } catch {
        /* ignore */
      }
    }
    if (key === "reset_cursor" && activeTab === "actions") {
      const data = await getAdminSyncCursors(sellerAccountId, { limit: 50 });
      setCursors(data.items);
    }
    if (key === "rerun_recommendations" && activeTab === "actions") {
      const data = await getAdminRecommendationRuns(sellerAccountId, { limit: 20 });
      setRuns(data.items);
    }
  }

  async function openRunDiagnostics(runId: number) {
    setSelectedRunId(runId);
    setSelectedRunLoading(true);
    setSelectedRunError(null);
    setSelectedRecommendationDetail(null);
    setSelectedRecommendationError(null);
    try {
      const data = await getAdminRecommendationRunDetail(sellerAccountId, runId);
      setSelectedRun(data);
    } catch (e: unknown) {
      setSelectedRun(null);
      setSelectedRunError(e instanceof Error ? e.message : "Не удалось загрузить диагностику прогона");
    } finally {
      setSelectedRunLoading(false);
    }
  }

  async function openRecommendationRaw(recommendationId: number) {
    setSelectedRecommendationLoading(true);
    setSelectedRecommendationError(null);
    try {
      const data = await getAdminRecommendationDetail(sellerAccountId, recommendationId);
      setSelectedRecommendationDetail(data);
    } catch (e: unknown) {
      setSelectedRecommendationDetail(null);
      setSelectedRecommendationError(e instanceof Error ? e.message : "Не удалось загрузить сырой ответ ИИ для рекомендации");
    } finally {
      setSelectedRecommendationLoading(false);
    }
  }

  async function openTraceDiagnostics(traceId: number) {
    setSelectedTraceId(traceId);
    setSelectedTraceLoading(true);
    setSelectedTraceError(null);
    try {
      const data = await getAdminChatTraceDetail(sellerAccountId, traceId);
      setSelectedTrace(data);
    } catch (e: unknown) {
      setSelectedTrace(null);
      setSelectedTraceError(e instanceof Error ? e.message : "Не удалось загрузить диагностику трейса");
    } finally {
      setSelectedTraceLoading(false);
    }
  }

  async function loadSessionMessages(sessionId: number) {
    setSelectedSessionId(sessionId);
    setMessagesLoading(true);
    setMessagesError(null);
    try {
      const data = await getAdminChatMessages(sellerAccountId, sessionId, { limit: 50 });
      setMessages(data.items);
    } catch (e: unknown) {
      setMessages([]);
      setMessagesError(e instanceof Error ? e.message : "Не удалось загрузить сообщения сессии");
    } finally {
      setMessagesLoading(false);
    }
  }

  if (adminReady === "loading") return <main className="p-6 text-sm">Проверка доступа администратора...</main>;
  if (adminReady === "forbidden") return <main className="p-6 text-sm text-red-700">Требуется доступ администратора.</main>;
  if (adminReady === "error") return <main className="p-6 text-sm text-red-700">{error ?? "Не удалось проверить доступ."}</main>;
  if (loadingDetail) return <main className="p-6 text-sm">Загрузка карточки клиента...</main>;
  if (!detail) return <main className="p-6 text-sm text-red-700">{error ?? "Клиент не найден."}</main>;

  return (
    <main className="space-y-4 p-6">
      <div>
        <Link href="/app/admin" className="text-sm text-blue-700 underline">
          ← К списку клиентов
        </Link>
      </div>

      <ClientSummaryStrip detail={detail} />

      <AdminTabBar activeTab={activeTab} onTabChange={setActiveTab} />

      {tabError ? <p className="text-sm text-red-700">{tabError}</p> : null}
      {tabLoading ? <p className="text-sm">Загрузка вкладки...</p> : null}

      {!tabLoading && activeTab === "overview" ? (
        <section className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Card title="Продавец">
            <p>Статус: {detail.overview.seller_status}</p>
            <p>Владелец: {detail.overview.owner_email ?? "—"}</p>
            <p>Создан: {fmtDate(detail.overview.created_at)}</p>
          </Card>
          <Card title="Подключения">
            {detail.connections.length === 0 ? <p className="text-aivion-muted">Подключений нет.</p> : detail.connections.map((c) => (
              <div key={c.provider} className="text-sm space-y-1 border-b border-aivion-border pb-2 mb-2 last:border-0 last:pb-0 last:mb-0">
                <p><span className="font-medium">{c.provider}</span> — Seller API: {c.status}</p>
                <p className="text-aivion-text">Performance API: {c.performance_connection_status}{c.performance_token_set ? " (токен задан)" : " (токен не задан)"}</p>
              </div>
            ))}
          </Card>
          <Card title="Последняя синхронизация">
            {detail.operational_status.latest_sync_job ? <p>#{detail.operational_status.latest_sync_job.id} {detail.operational_status.latest_sync_job.status}</p> : <p className="text-aivion-muted">Задач синхронизации нет.</p>}
          </Card>
          <Card title="Биллинг (кратко)">
            {detail.billing ? <p>{detail.billing.plan_code} / {detail.billing.status}</p> : <p className="text-aivion-muted">Данные биллинга отсутствуют.</p>}
          </Card>
        </section>
      ) : null}

      {!tabLoading && activeTab === "sync_import" ? (
        <section className="space-y-4">
          <Card title="Задачи синхронизации">
            {syncJobs.length === 0 ? (
              <EmptyState title="Нет задач синхронизации" message="Для этого аккаунта задач не вернули (лимит 20)." />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-sm">
                  <thead>
                    <tr className="border-b text-xs uppercase text-aivion-muted-soft">
                      <th className="px-2 py-2">ID</th>
                      <th className="px-2 py-2">Тип</th>
                      <th className="px-2 py-2">Статус</th>
                      <th className="px-2 py-2">Начало</th>
                      <th className="px-2 py-2">Конец</th>
                      <th className="px-2 py-2">Ошибка</th>
                    </tr>
                  </thead>
                  <tbody>
                    {syncJobs.map((x) => (
                      <tr key={x.id} className="border-b align-top">
                        <td className="px-2 py-2 font-mono">#{x.id}</td>
                        <td className="px-2 py-2">{x.type}</td>
                        <td className="px-2 py-2">
                          <Badge value={x.status} />
                        </td>
                        <td className="px-2 py-2 text-xs">{fmtDate(x.started_at)}</td>
                        <td className="px-2 py-2 text-xs">{fmtDate(x.finished_at)}</td>
                        <td className="max-w-md px-2 py-2 text-xs text-red-800">
                          {x.error_message ? (
                            <pre className="max-h-28 overflow-auto whitespace-pre-wrap rounded border bg-red-50/80 p-2">
                              {x.error_message}
                            </pre>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
          <Card title="Задачи импорта">
            {importJobs.length === 0 ? (
              <EmptyState title="Нет задач импорта" message="В последнем запросе задач импорта не было (лимит 20)." />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-sm">
                  <thead>
                    <tr className="border-b text-xs uppercase text-aivion-muted-soft">
                      <th className="px-2 py-2">ID</th>
                      <th className="px-2 py-2">Синхр.</th>
                      <th className="px-2 py-2">Домен</th>
                      <th className="px-2 py-2">Статус</th>
                      <th className="px-2 py-2">Получено / ОК / сбой</th>
                      <th className="px-2 py-2">Ошибка</th>
                    </tr>
                  </thead>
                  <tbody>
                    {importJobs.map((x) => (
                      <tr key={x.id} className="border-b align-top">
                        <td className="px-2 py-2 font-mono">#{x.id}</td>
                        <td className="px-2 py-2 font-mono">#{x.sync_job_id}</td>
                        <td className="px-2 py-2">{x.domain}</td>
                        <td className="px-2 py-2">
                          <Badge value={x.status} />
                        </td>
                        <td className="px-2 py-2 text-xs">
                          {x.records_received}/{x.records_imported}/{x.records_failed}
                        </td>
                        <td className="max-w-md px-2 py-2 text-xs text-red-800">
                          {x.error_message ? (
                            <pre className="max-h-28 overflow-auto whitespace-pre-wrap rounded border bg-red-50/80 p-2">
                              {x.error_message}
                            </pre>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
          <Card title="Ошибки импорта">
            {importErrors.length === 0 ? (
              <EmptyState
                title="Нет ошибок импорта"
                message="В этом запросе строк в import_errors нет (лимит 20). Сбои могут отображаться как error_message у задачи выше."
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-sm">
                  <thead>
                    <tr className="border-b text-xs uppercase text-aivion-muted-soft">
                      <th className="px-2 py-2">Задача импорта</th>
                      <th className="px-2 py-2">Синхр.</th>
                      <th className="px-2 py-2">Домен</th>
                      <th className="px-2 py-2">Статус</th>
                      <th className="px-2 py-2">Сбойных записей</th>
                      <th className="px-2 py-2">Сообщение</th>
                    </tr>
                  </thead>
                  <tbody>
                    {importErrors.map((x) => (
                      <tr key={`${x.import_job_id}-${x.domain}-${x.started_at}`} className="border-b align-top">
                        <td className="px-2 py-2 font-mono">#{x.import_job_id}</td>
                        <td className="px-2 py-2 font-mono">#{x.sync_job_id}</td>
                        <td className="px-2 py-2">{x.domain}</td>
                        <td className="px-2 py-2">
                          <Badge value={x.status} />
                        </td>
                        <td className="px-2 py-2">{x.records_failed}</td>
                        <td className="max-w-lg px-2 py-2 text-xs">
                          <pre className="max-h-36 overflow-auto whitespace-pre-wrap rounded border bg-aivion-surface-soft p-2">
                            {x.error_message}
                          </pre>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </section>
      ) : null}

      {!tabLoading && activeTab === "cursors" ? (
        <Card title="Курсоры синхронизации">
          {cursors.length === 0 ? (
            <EmptyState title="Нет курсоров" message="Для этого аккаунта строк курсоров нет (лимит 50)." />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full text-left text-sm">
                <thead>
                  <tr className="border-b text-xs uppercase text-aivion-muted-soft">
                    <th className="px-2 py-2">Домен</th>
                    <th className="px-2 py-2">Тип курсора</th>
                    <th className="px-2 py-2">Значение</th>
                    <th className="px-2 py-2">Обновлено</th>
                  </tr>
                </thead>
                <tbody>
                  {cursors.map((x) => (
                    <tr key={`${x.domain}-${x.cursor_type}`} className="border-b align-top">
                      <td className="px-2 py-2 font-medium">{x.domain}</td>
                      <td className="px-2 py-2">{x.cursor_type}</td>
                      <td className="max-w-md px-2 py-2 font-mono text-xs break-all">{x.cursor_value ?? "—"}</td>
                      <td className="px-2 py-2 text-xs">{fmtDate(x.updated_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      ) : null}

      {!tabLoading && activeTab === "alerts" ? (
        <Card title="Алерты">
          <p className="text-sm">Открытых алертов: {detail.operational_status.open_alerts_count}</p>
          <p className="text-sm text-aivion-muted mt-1">Детальный эндпоинт прогонов алертов в admin HTTP API на этом этапе не выставлен.</p>
        </Card>
      ) : null}

      {!tabLoading && activeTab === "recommendations" ? (
        <section className="space-y-3">
          <Card title="Прогоны рекомендаций">
            {runsError ? <p className="mb-2 text-sm text-red-700">{runsError}</p> : null}
            {runsLoading ? (
              <p className="text-sm">Загрузка прогонов...</p>
            ) : runs.length === 0 ? (
              <p className="text-sm text-aivion-muted">Прогонов рекомендаций не найдено.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="border-b text-left">
                      <th className="px-2 py-2">Прогон</th>
                      <th className="px-2 py-2">Статус</th>
                      <th className="px-2 py-2">На дату</th>
                      <th className="px-2 py-2">Модель / промпт</th>
                      <th className="px-2 py-2">Токены / стоимость</th>
                      <th className="px-2 py-2">Счётчики</th>
                      <th className="px-2 py-2">Время</th>
                      <th className="px-2 py-2">Ошибка</th>
                      <th className="px-2 py-2">Действие</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runs.map((r) => (
                      <tr key={r.id} className="border-b align-top">
                        <td className="px-2 py-2">#{r.id}<br />{r.run_type}</td>
                        <td className="px-2 py-2"><Badge value={r.status} /></td>
                        <td className="px-2 py-2">{r.as_of_date ?? "—"}</td>
                        <td className="px-2 py-2 text-xs">
                          <div>{r.ai_model ?? "—"}</div>
                          <div className="text-aivion-muted">{r.ai_prompt_version ?? "—"}</div>
                        </td>
                        <td className="px-2 py-2">
                          <div className="flex flex-wrap gap-1">
                            <MetricChip label="вход" value={r.input_tokens} />
                            <MetricChip label="выход" value={r.output_tokens} />
                            <MetricChip label="стоим." value={r.estimated_cost} />
                          </div>
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>Сгенерировано: {r.generated_recommendations_count}</div>
                          <div>Принято: {r.accepted_recommendations_count}</div>
                          <div>Отклонено: {r.rejected_recommendations_count ?? "—"}</div>
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>{fmtDate(r.started_at)}</div>
                          <div>{fmtDate(r.finished_at)}</div>
                        </td>
                        <td className="px-2 py-2 text-xs text-red-700">{r.error_message ?? "—"}</td>
                        <td className="px-2 py-2">
                          <button
                            type="button"
                            className="rounded border px-2 py-1 text-xs hover:bg-aivion-surface-soft"
                            onClick={() => void openRunDiagnostics(r.id)}
                          >
                            Диагностика
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
          <Card title="Диагностика прогона рекомендаций">
            {selectedRunId != null ? <p className="mb-2 text-xs text-aivion-muted">Выбран прогон: #{selectedRunId}</p> : null}
            {selectedRunLoading ? <p className="text-sm">Загрузка диагностики прогона...</p> : null}
            {selectedRunError ? <p className="text-sm text-red-700">{selectedRunError}</p> : null}
            {!selectedRunLoading && !selectedRunError && !selectedRun ? (
              <p className="text-sm text-aivion-muted">Выберите прогон для просмотра диагностики.</p>
            ) : null}
            {selectedRun ? (
              <div className="space-y-3 text-sm">
                <div className="rounded border bg-aivion-surface-soft p-3">
                  <p className="font-medium">Прогон #{selectedRun.run.id} ({selectedRun.run.run_type})</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Badge value={selectedRun.run.status} />
                    <MetricChip label="вход. токены" value={selectedRun.run.input_tokens} />
                    <MetricChip label="выход. токены" value={selectedRun.run.output_tokens} />
                    <MetricChip label="оц. стоимость" value={selectedRun.run.estimated_cost} />
                    <MetricChip label="сгенер." value={selectedRun.run.generated_recommendations_count} />
                    <MetricChip label="принято" value={selectedRun.run.accepted_recommendations_count} />
                    <MetricChip label="отклон." value={selectedRun.run.rejected_recommendations_count ?? "—"} />
                  </div>
                  <p className="mt-2 text-xs text-aivion-text">
                    as_of_date={selectedRun.run.as_of_date ?? "—"}, model={selectedRun.run.ai_model ?? "—"}, prompt={selectedRun.run.ai_prompt_version ?? "—"}
                  </p>
                  {selectedRun.run.error_message ? (
                    <ErrorBox message={selectedRun.run.error_message} />
                  ) : null}
                </div>

                {selectedRun.limitations?.length ? (
                  <div className="rounded border border-yellow-300 bg-yellow-50 p-3 text-xs text-yellow-900">
                    {selectedRun.limitations.map((l, i) => (
                      <p key={`${i}-${l}`}>{l}</p>
                    ))}
                  </div>
                ) : null}

                <div>
                  <h3 className="mb-2 font-medium">Связанные рекомендации</h3>
                  {selectedRun.recommendations.length === 0 ? (
                    <p className="text-xs text-aivion-muted">Связанных рекомендаций нет.</p>
                  ) : (
                    <div className="space-y-2">
                      {selectedRun.recommendations.map((rec) => (
                        <div key={rec.id} className="rounded border p-2 text-xs">
                          <div className="flex items-start justify-between gap-2">
                            <div>
                              <p className="font-medium">#{rec.id} {rec.title}</p>
                              <p className="text-aivion-text">
                                {rec.recommendation_type} · {rec.priority_level}/{rec.confidence_level} · {rec.status}
                              </p>
                              <p className="text-aivion-text">сущность: {formatEntity(rec.entity_type, rec.entity_id, rec.entity_sku, rec.entity_offer_id)}</p>
                            </div>
                            <button
                              type="button"
                              className="rounded border px-2 py-1 hover:bg-aivion-surface-soft"
                              onClick={() => void openRecommendationRaw(rec.id)}
                            >
                              Сырой ответ ИИ
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div>
                  <h3 className="mb-2 font-medium">Диагностика</h3>
                  {selectedRun.diagnostics.length === 0 ? (
                    <p className="text-xs text-aivion-muted">Записей диагностики нет.</p>
                  ) : (
                    <div className="space-y-3">
                      {selectedRun.diagnostics.map((diag: AdminRecommendationDiagnosticItem) => (
                        <div key={diag.id} className="rounded border p-3">
                          <p className="text-xs font-medium">Диагностика #{diag.id} · request={diag.openai_request_id ?? "—"}</p>
                          <div className="mt-2 flex flex-wrap gap-2">
                            <MetricChip label="вход. токены" value={diag.input_tokens} />
                            <MetricChip label="выход. токены" value={diag.output_tokens} />
                            <MetricChip label="оц. стоимость" value={diag.estimated_cost} />
                          </div>
                          <p className="mt-2 text-xs text-aivion-text">
                            model={diag.ai_model ?? "—"}, prompt={diag.prompt_version ?? "—"}, created={fmtDate(diag.created_at)}
                          </p>
                          {(diag.error_message || diag.error_stage) ? (
                            <ErrorBox message={diag.error_message ?? "Ошибка диагностики"} stage={diag.error_stage ?? undefined} />
                          ) : null}
                          <InternalSupportDataNotice />
                          <div className="mt-2 space-y-2">
                            <JsonDetailsBlock title="Сводка контекста (payload)" data={diag.context_payload_summary} />
                            <JsonDetailsBlock title="Сырой ответ OpenAI" data={diag.raw_openai_response} />
                            <JsonDetailsBlock title="Результат валидации" data={diag.validation_result_payload} />
                            <JsonDetailsBlock title="Отклонённые элементы" data={diag.rejected_items_payload} />
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ) : null}
          </Card>

          <Card title="Сырой ответ ИИ по рекомендации">
            {selectedRecommendationLoading ? <p className="text-sm">Загрузка деталей рекомендации...</p> : null}
            {selectedRecommendationError ? <p className="text-sm text-red-700">{selectedRecommendationError}</p> : null}
            {!selectedRecommendationLoading && !selectedRecommendationError && !selectedRecommendationDetail ? (
              <p className="text-sm text-aivion-muted">Выберите рекомендацию в диагностике прогона.</p>
            ) : null}
            {selectedRecommendationDetail ? (
              <div className="space-y-3 text-sm">
                <div className="rounded border bg-aivion-surface-soft p-3">
                  <p className="font-medium">
                    #{selectedRecommendationDetail.recommendation.id} {selectedRecommendationDetail.recommendation.title}
                  </p>
                  <p className="text-xs text-aivion-text">
                    {selectedRecommendationDetail.recommendation.recommendation_type} · {selectedRecommendationDetail.recommendation.priority_level}/{selectedRecommendationDetail.recommendation.confidence_level} · {selectedRecommendationDetail.recommendation.status}
                  </p>
                  <p className="text-xs text-aivion-text">
                    сущность: {formatEntity(
                      selectedRecommendationDetail.recommendation.entity_type,
                      selectedRecommendationDetail.recommendation.entity_id,
                      selectedRecommendationDetail.recommendation.entity_sku,
                      selectedRecommendationDetail.recommendation.entity_offer_id,
                    )}
                  </p>
                  <p className="mt-2 text-xs">{selectedRecommendationDetail.recommendation.recommended_action ?? "—"}</p>
                  <p className="text-xs text-aivion-text">Ожидаемый эффект: {selectedRecommendationDetail.recommendation.expected_effect ?? "—"}</p>
                </div>
                {selectedRecommendationDetail.related_alerts.length > 0 ? (
                  <div className="rounded border p-3 text-xs">
                    <p className="mb-1 font-medium">Связанные алерты ({selectedRecommendationDetail.related_alerts.length})</p>
                    <ul className="space-y-1">
                      {selectedRecommendationDetail.related_alerts.map((a) => (
                        <li key={a.id}>#{a.id} {a.alert_type} · {a.severity}/{a.urgency} · {a.status}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {selectedRecommendationDetail.limitations?.length ? (
                  <div className="rounded border border-yellow-300 bg-yellow-50 p-3 text-xs text-yellow-900">
                    {selectedRecommendationDetail.limitations.map((l, i) => (
                      <p key={`${i}-${l}`}>{l}</p>
                    ))}
                  </div>
                ) : null}
                <InternalSupportDataNotice />
                <JsonDetailsBlock title="Метрики-поддержка (payload)" data={selectedRecommendationDetail.recommendation.supporting_metrics_payload} />
                <JsonDetailsBlock title="Ограничения (payload)" data={selectedRecommendationDetail.recommendation.constraints_payload} />
                <JsonDetailsBlock title="Сырой ответ ИИ" data={selectedRecommendationDetail.recommendation.raw_ai_response} />
              </div>
            ) : null}
          </Card>
        </section>
      ) : null}

      {!tabLoading && activeTab === "chat_logs" ? (
        <section className="space-y-3">
          <Card title="Фильтры трейсов">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-4">
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">status</span>
                <input className="w-full rounded border px-2 py-1 text-sm" value={traceStatusFilter} onChange={(e) => setTraceStatusFilter(e.target.value)} placeholder="completed / failed" />
              </label>
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">intent</span>
                <input className="w-full rounded border px-2 py-1 text-sm" value={traceIntentFilter} onChange={(e) => setTraceIntentFilter(e.target.value)} placeholder="sales" />
              </label>
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">session_id</span>
                <input className="w-full rounded border px-2 py-1 text-sm" value={traceSessionFilter} onChange={(e) => setTraceSessionFilter(e.target.value)} placeholder="123" />
              </label>
              <div className="flex items-end">
                <button
                  type="button"
                  className="w-full rounded border px-3 py-2 text-sm hover:bg-aivion-surface-soft"
                  onClick={() => setActiveTab("chat_logs")}
                >
                  Обновить
                </button>
              </div>
            </div>
          </Card>

          <Card title="Трейсы чата">
            {tracesError ? <p className="mb-2 text-sm text-red-700">{tracesError}</p> : null}
            {tracesLoading ? (
              <p className="text-sm">Загрузка трейсов...</p>
            ) : traces.length === 0 ? (
              <p className="text-sm text-aivion-muted">Трейсы чата не найдены.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="border-b text-left">
                      <th className="px-2 py-2">Трейс</th>
                      <th className="px-2 py-2">Интент / статус</th>
                      <th className="px-2 py-2">Модели</th>
                      <th className="px-2 py-2">Промпты</th>
                      <th className="px-2 py-2">Токены / стоимость</th>
                      <th className="px-2 py-2">Время</th>
                      <th className="px-2 py-2">Ошибка</th>
                      <th className="px-2 py-2">Действие</th>
                    </tr>
                  </thead>
                  <tbody>
                    {traces.map((t) => (
                      <tr key={t.id} className="border-b align-top">
                        <td className="px-2 py-2 text-xs">
                          <div>трейс #{t.id}</div>
                          <div>сессия #{t.session_id}</div>
                          <div>user_msg #{t.user_message_id ?? "—"}</div>
                          <div>assistant_msg #{t.assistant_message_id ?? "—"}</div>
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>{t.detected_intent ?? "—"}</div>
                          <Badge value={t.status} />
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>{t.planner_model}</div>
                          <div>{t.answer_model}</div>
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>{t.planner_prompt_version}</div>
                          <div>{t.answer_prompt_version}</div>
                        </td>
                        <td className="px-2 py-2">
                          <div className="flex flex-wrap gap-1">
                            <MetricChip label="вход" value={t.input_tokens} />
                            <MetricChip label="выход" value={t.output_tokens} />
                            <MetricChip label="стоим." value={t.estimated_cost} />
                          </div>
                        </td>
                        <td className="px-2 py-2 text-xs">
                          <div>{fmtDate(t.started_at)}</div>
                          <div>{fmtDate(t.finished_at)}</div>
                        </td>
                        <td className="px-2 py-2 text-xs text-red-700">{t.error_message ?? "—"}</td>
                        <td className="px-2 py-2">
                          <button
                            type="button"
                            className="rounded border px-2 py-1 text-xs hover:bg-aivion-surface-soft"
                            onClick={() => void openTraceDiagnostics(t.id)}
                          >
                            Открыть трейс
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>

          <Card title="Диагностика трейса чата">
            {selectedTraceId != null ? <p className="mb-2 text-xs text-aivion-muted">Выбран трейс: #{selectedTraceId}</p> : null}
            {selectedTraceLoading ? <p className="text-sm">Загрузка деталей трейса...</p> : null}
            {selectedTraceError ? <p className="text-sm text-red-700">{selectedTraceError}</p> : null}
            {!selectedTraceLoading && !selectedTraceError && !selectedTrace ? (
              <p className="text-sm text-aivion-muted">Выберите трейс для диагностики.</p>
            ) : null}
            {selectedTrace ? (
              <div className="space-y-3 text-sm">
                <div className="rounded border bg-aivion-surface-soft p-3">
                  <p className="font-medium">Трейс #{selectedTrace.trace.id} / сессия #{selectedTrace.trace.session_id}</p>
                  <p className="text-xs text-aivion-text">
                    intent={selectedTrace.trace.detected_intent ?? "—"} · planner={selectedTrace.trace.planner_model} · answer={selectedTrace.trace.answer_model}
                  </p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Badge value={selectedTrace.trace.status} />
                    <MetricChip label="вход. токены" value={selectedTrace.trace.input_tokens} />
                    <MetricChip label="выход. токены" value={selectedTrace.trace.output_tokens} />
                    <MetricChip label="оц. стоимость" value={selectedTrace.trace.estimated_cost} />
                  </div>
                  {(selectedTrace.trace.error_message) ? (
                    <ErrorBox message={selectedTrace.trace.error_message} />
                  ) : null}
                  <p className="mt-2 text-xs text-aivion-text">
                    started={fmtDate(selectedTrace.trace.started_at)}, finished={fmtDate(selectedTrace.trace.finished_at)}, created={fmtDate(selectedTrace.trace.created_at)}
                  </p>
                </div>
                <div className="rounded border p-3">
                  <p className="mb-2 font-medium">Сообщения</p>
                  {selectedTrace.messages.length === 0 ? (
                    <p className="text-xs text-aivion-muted">Сообщений нет.</p>
                  ) : (
                    <div className="space-y-2">
                      {selectedTrace.messages.map((m) => (
                        <div key={m.id} className={`rounded border p-2 text-xs ${m.role === "user" ? "bg-blue-50" : "bg-aivion-surface-soft"}`}>
                          <p className="font-medium">{m.role === "user" ? "Вопрос пользователя" : m.role === "assistant" ? "Ответ ассистента" : m.role}</p>
                          <p className="text-aivion-text">{m.message_type} · {fmtDate(m.created_at)}</p>
                          <p className="mt-1 whitespace-pre-wrap">{m.content}</p>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                {selectedTrace.limitations?.length ? (
                  <div className="rounded border border-yellow-300 bg-yellow-50 p-3 text-xs text-yellow-900">
                    {selectedTrace.limitations.map((l, i) => (
                      <p key={`${i}-${l}`}>{l}</p>
                    ))}
                  </div>
                ) : null}
                <InternalSupportDataNotice />
                <JsonDetailsBlock title="План инструментов" data={selectedTrace.payloads.tool_plan_payload} />
                <JsonDetailsBlock title="Проверенный план инструментов" data={selectedTrace.payloads.validated_tool_plan_payload} />
                <JsonDetailsBlock title="Результаты инструментов" data={selectedTrace.payloads.tool_results_payload} />
                <JsonDetailsBlock title="Контекст фактов" data={selectedTrace.payloads.fact_context_payload} />
                <JsonDetailsBlock title="Сырой ответ планировщика" data={selectedTrace.payloads.raw_planner_response} />
                <JsonDetailsBlock title="Сырой ответ ассистента" data={selectedTrace.payloads.raw_answer_response} />
                <JsonDetailsBlock title="Проверка ответа" data={selectedTrace.payloads.answer_validation_payload} />
              </div>
            ) : null}
          </Card>

          <Card title="Сессии чата">
            {sessionsError ? <p className="mb-2 text-sm text-red-700">{sessionsError}</p> : null}
            {sessionsLoading ? (
              <p className="text-sm">Загрузка сессий...</p>
            ) : sessions.length === 0 ? (
              <p className="text-sm text-aivion-muted">Сессии чата не найдены.</p>
            ) : (
              <div className="space-y-2">
                {sessions.map((s) => (
                  <div key={s.id} className="flex items-center justify-between rounded border p-2 text-xs">
                    <div>
                      <p className="font-medium">#{s.id} {s.title}</p>
                      <p className="text-aivion-text">{s.status} · последнее={fmtDate(s.last_message_at)}</p>
                    </div>
                    <button
                      type="button"
                      className="rounded border px-2 py-1 hover:bg-aivion-surface-soft"
                      onClick={() => void loadSessionMessages(s.id)}
                    >
                      Загрузить сообщения
                    </button>
                  </div>
                ))}
              </div>
            )}
          </Card>
          <Card title="Сообщения сессии">
            {selectedSessionId == null ? <p className="text-sm text-aivion-muted">Выберите сессию и нажмите «Загрузить сообщения».</p> : null}
            {messagesError ? <p className="mb-2 text-sm text-red-700">{messagesError}</p> : null}
            {messagesLoading ? (
              <p className="text-sm">Загрузка сообщений...</p>
            ) : messages.length === 0 ? (
              <p className="text-sm text-aivion-muted">Сообщения не загружены.</p>
            ) : (
              <div className="space-y-2">
                {messages.map((m) => (
                  <div key={m.id} className="rounded border bg-aivion-surface-soft p-2 text-xs">
                    <p className="font-medium">{m.role} · {m.message_type}</p>
                    <p className="text-aivion-text">{fmtDate(m.created_at)}</p>
                    <p className="mt-1 whitespace-pre-wrap">{m.content}</p>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </section>
      ) : null}

      {!tabLoading && activeTab === "feedback" ? (
        <section className="space-y-6">
          <div className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
            <h2 className="mb-1 text-lg font-semibold text-aivion-text">Отзывы по чату</h2>
            <p className="mb-3 text-sm text-aivion-muted">Оценки пользователей ответам ИИ в чате (вид администратора).</p>
            {chatFeedback.length === 0 ? (
              <EmptyState title="Нет отзывов по чату" message="Для этого клиента записей отзывов нет (лимит 20)." />
            ) : (
              <ul className="space-y-2 text-sm">
                {chatFeedback.map((f, i) => (
                  <li key={i} className="rounded-md border border-aivion-border bg-aivion-surface-soft/80 px-3 py-2">
                    <span className="font-medium text-aivion-text">{String((f as { rating?: string }).rating ?? "—")}</span>
                    <span className="text-aivion-text"> — {(f as { comment?: string }).comment ?? "без комментария"}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="rounded-lg border border-indigo-100 bg-indigo-50/40 p-4 shadow-sm">
            <h2 className="mb-1 text-lg font-semibold text-indigo-950">Отзывы по рекомендациям</h2>
            <p className="mb-3 text-sm text-indigo-900/80">Оценки, привязанные к строкам рекомендаций, плюс счётчики по прокси-статусам.</p>
            {!recFeedback || recFeedback.items.length === 0 ? (
              <EmptyState title="Нет отзывов по рекомендациям" message="Отзывов по рекомендациям для этого клиента нет (лимит 20)." />
            ) : (
              <>
                <ul className="space-y-2 text-sm">
                  {recFeedback.items.map((item) => (
                    <li key={item.id} className="rounded-md border border-white/80 bg-aivion-surface px-3 py-2">
                      <span className="font-medium text-aivion-text">{item.rating}</span>
                      <span className="text-aivion-text"> — {item.recommendation.title}</span>
                      <span className="text-aivion-muted"> ({item.recommendation.status})</span>
                    </li>
                  ))}
                </ul>
                <p className="mt-3 text-xs text-indigo-950/90">
                  Прокси-статус: принято={recFeedback.proxy_status_feedback.accepted_count}, отклонено=
                  {recFeedback.proxy_status_feedback.dismissed_count}, решено=
                  {recFeedback.proxy_status_feedback.resolved_count}
                </p>
              </>
            )}
          </div>
        </section>
      ) : null}

      {!tabLoading && activeTab === "billing" ? (
        <Card title="Биллинг">
          {billingMissing ? (
            <p className="text-sm text-aivion-muted">Биллинг для этого клиента не настроен.</p>
          ) : !billing ? (
            <p className="text-sm">Загрузка...</p>
          ) : (
            <div className="grid grid-cols-1 gap-1 text-sm md:grid-cols-2">
              <p>plan_code: {billing.plan_code}</p>
              <p>status: {billing.status}</p>
              <p>trial_ends_at: {fmtDate(billing.trial_ends_at)}</p>
              <p>period: {fmtDate(billing.current_period_start)} — {fmtDate(billing.current_period_end)}</p>
              <p>ai_tokens_limit_month: {billing.ai_tokens_limit_month ?? "—"}</p>
              <p>ai_tokens_used_month: {billing.ai_tokens_used_month}</p>
              <p>estimated_ai_cost_month: {billing.estimated_ai_cost_month}</p>
              <p>notes: {billing.notes ?? "—"}</p>
              <p>updated_at: {fmtDate(billing.updated_at)}</p>
            </div>
          )}
        </Card>
      ) : null}

      {!tabLoading && activeTab === "actions" ? (
        <Card title="Действия поддержки">
          <p className="text-sm text-aivion-muted">
            Рекомендуемый порядок для восстановления: подтянуть свежие данные Ozon → при необходимости сбросить зависшие курсоры → пересобрать агрегаты → заново сгенерировать ИИ.
          </p>
          <div className="mt-6 divide-y divide-gray-200">
            <div className="py-6">
              <div className="mb-3 flex flex-wrap items-baseline gap-2">
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-bold text-white">
                  1
                </span>
                <h3 className="text-base font-semibold text-aivion-text">Повторить синхронизацию</h3>
              </div>
              <p className="mb-3 text-sm text-aivion-muted">
                Ставит новую задачу синхронизации Ozon, чтобы каталог, заказы, остатки и реклама соответствовали маркетплейсу. Запускайте первым при устаревших данных или после исправления учётных данных.
              </p>
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">sync_type</span>
                <select className="rounded border px-2 py-1 text-sm" value={syncType} onChange={(e) => setSyncType(e.target.value)}>
                  <option value="initial_sync">initial_sync</option>
                </select>
              </label>
              <button
                type="button"
                className={`mt-3 ${buttonClassNames("primary")}`}
                disabled={actionLoading.rerun_sync}
                onClick={() => {
                  if (!window.confirm("Запустить новую задачу синхронизации для этого клиента?")) return;
                  const payload = { sync_type: syncType };
                  void runAction("rerun_sync", payload, () => rerunAdminSync(sellerAccountId, payload));
                }}
              >
                {actionLoading.rerun_sync ? "Выполняется…" : "Повторить синхронизацию"}
              </button>
              <ActionResultCard
                result={actionResults.rerun_sync}
                error={actionErrors.rerun_sync}
                success={actionSuccess.rerun_sync}
                requestPayload={actionRequestPayloads.rerun_sync}
              />
            </div>

            <div className="py-6">
              <div className="mb-3 flex flex-wrap items-baseline gap-2">
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-bold text-white">
                  2
                </span>
                <h3 className="text-base font-semibold text-aivion-text">Сброс курсора</h3>
              </div>
              <p className="mb-3 text-sm text-aivion-muted">
                Очищает или задаёт курсор домена, чтобы следующая синхронизация перечитала данные с выбранной позиции. Используйте при зависании импорта или контролируемом повторе (осторожно в продакшене).
              </p>
              <p className="mb-3 rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-950">
                Внимание: сброс курсора может привести к повторному импорту пересекающихся данных.
              </p>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-3">
                <label className="text-sm">
                  <span className="mb-1 block text-aivion-text">domain</span>
                  <input
                    className="w-full rounded border px-2 py-1 text-sm"
                    value={cursorDomain}
                    onChange={(e) => setCursorDomain(e.target.value)}
                    placeholder="orders"
                  />
                </label>
                <label className="text-sm">
                  <span className="mb-1 block text-aivion-text">cursor_type</span>
                  <input
                    className="w-full rounded border px-2 py-1 text-sm"
                    value={cursorType}
                    onChange={(e) => setCursorType(e.target.value)}
                    placeholder="source_cursor"
                  />
                </label>
                <label className="text-sm">
                  <span className="mb-1 block text-aivion-text">cursor_value (необяз.)</span>
                  <input
                    className="w-full rounded border px-2 py-1 text-sm"
                    value={cursorValue}
                    onChange={(e) => setCursorValue(e.target.value)}
                    placeholder="пусто = null"
                  />
                </label>
              </div>
              {(!cursorDomain.trim() || !cursorType.trim()) && actionErrors.reset_cursor ? (
                <p className="mt-2 text-sm text-red-700">{actionErrors.reset_cursor}</p>
              ) : null}
              <button
                type="button"
                className={`mt-3 ${buttonClassNames("secondary")}`}
                disabled={actionLoading.reset_cursor || !cursorDomain.trim() || !cursorType.trim()}
                onClick={() => {
                  const domain = cursorDomain.trim();
                  const cursor_type = cursorType.trim();
                  if (!domain || !cursor_type) {
                    setActionErrors((prev) => ({ ...prev, reset_cursor: "Нужны domain и cursor_type" }));
                    return;
                  }
                  if (!window.confirm("Сбросить этот курсор синхронизации?")) return;
                  const payload = {
                    domain,
                    cursor_type,
                    cursor_value: cursorValue.trim() === "" ? null : cursorValue.trim(),
                  };
                  void runAction("reset_cursor", payload, () => resetAdminCursor(sellerAccountId, payload));
                }}
              >
                {actionLoading.reset_cursor ? "Выполняется…" : "Сбросить курсор"}
              </button>
              <ActionResultCard
                result={actionResults.reset_cursor}
                error={actionErrors.reset_cursor}
                success={actionSuccess.reset_cursor}
                requestPayload={actionRequestPayloads.reset_cursor}
              />
            </div>

            <div className="py-6">
              <div className="mb-3 flex flex-wrap items-baseline gap-2">
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-bold text-white">
                  3
                </span>
                <h3 className="text-base font-semibold text-aivion-text">Пересчёт метрик</h3>
              </div>
              <p className="mb-3 text-sm text-aivion-muted">
                Пересчитывает агрегаты дашборда за выбранный период. Запускайте после успешной синхронизации, чтобы KPI и таблицы SKU совпали с импортом.
              </p>
              <p className="mb-3 rounded border border-amber-100 bg-amber-50/80 p-2 text-xs text-amber-950">
                Может завершиться ошибкой, если воркеры метрик в этой среде не настроены.
              </p>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
                <label className="text-sm">
                  <span className="mb-1 block text-aivion-text">date_from</span>
                  <input className="w-full rounded border px-2 py-1 text-sm" type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
                </label>
                <label className="text-sm">
                  <span className="mb-1 block text-aivion-text">date_to</span>
                  <input className="w-full rounded border px-2 py-1 text-sm" type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
                </label>
              </div>
              <button
                type="button"
                className={`mt-3 ${buttonClassNames("secondary")}`}
                disabled={actionLoading.rerun_metrics}
                onClick={() => {
                  if (!dateFrom || !dateTo) {
                    setActionErrors((prev) => ({ ...prev, rerun_metrics: "Нужны date_from и date_to" }));
                    return;
                  }
                  if (new Date(dateFrom).getTime() > new Date(dateTo).getTime()) {
                    setActionErrors((prev) => ({
                      ...prev,
                      rerun_metrics: "date_from должен быть не позже date_to",
                    }));
                    return;
                  }
                  if (!window.confirm("Пересчитать метрики за выбранный период?")) return;
                  const payload = { date_from: dateFrom, date_to: dateTo };
                  void runAction("rerun_metrics", payload, () => rerunAdminMetrics(sellerAccountId, payload));
                }}
              >
                {actionLoading.rerun_metrics ? "Выполняется…" : "Пересчитать метрики"}
              </button>
              <ActionResultCard
                result={actionResults.rerun_metrics}
                error={actionErrors.rerun_metrics}
                success={actionSuccess.rerun_metrics}
                requestPayload={actionRequestPayloads.rerun_metrics}
              />
            </div>

            <div className="py-6">
              <div className="mb-3 flex flex-wrap items-baseline gap-2">
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-bold text-white">
                  4
                </span>
                <h3 className="text-base font-semibold text-aivion-text">Пересобрать алерты</h3>
              </div>
              <p className="mb-3 text-sm text-aivion-muted">
                Заново строит алерты по продажам/остаткам/рекламе/ценам на дату. Запускайте перед рекомендациями, когда нужна чистая выборка для контекста ИИ.
              </p>
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">as_of_date</span>
                <input className="w-full rounded border px-2 py-1 text-sm md:w-64" type="date" value={asOfDate} onChange={(e) => setAsOfDate(e.target.value)} />
              </label>
              <button
                type="button"
                className={`mt-3 ${buttonClassNames("secondary")}`}
                disabled={actionLoading.rerun_alerts}
                onClick={() => {
                  if (!asOfDate) {
                    setActionErrors((prev) => ({ ...prev, rerun_alerts: "Нужен as_of_date" }));
                    return;
                  }
                  if (!window.confirm("Перезапустить движок алертов для этого клиента и даты?")) return;
                  const payload = { as_of_date: asOfDate };
                  void runAction("rerun_alerts", payload, () => rerunAdminAlerts(sellerAccountId, payload));
                }}
              >
                {actionLoading.rerun_alerts ? "Выполняется…" : "Пересобрать алерты"}
              </button>
              <ActionResultCard
                result={actionResults.rerun_alerts}
                error={actionErrors.rerun_alerts}
                success={actionSuccess.rerun_alerts}
                requestPayload={actionRequestPayloads.rerun_alerts}
              />
            </div>

            <div className="py-6">
              <div className="mb-3 flex flex-wrap items-baseline gap-2">
                <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-bold text-white">
                  5
                </span>
                <h3 className="text-base font-semibold text-aivion-text">Перезапуск рекомендаций ИИ</h3>
              </div>
              <p className="mb-3 text-sm text-aivion-muted">
                Вызывает движок рекомендаций ИИ на дату. Нужны адекватные метрики и алерты; расходует токены OpenAI и может повлечь затраты.
              </p>
              <p className="mb-3 rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-950">
                Вызывает OpenAI — проверьте бюджет и as_of_date перед запуском.
              </p>
              <label className="text-sm">
                <span className="mb-1 block text-aivion-text">as_of_date</span>
                <input className="w-full rounded border px-2 py-1 text-sm md:w-64" type="date" value={asOfDate} onChange={(e) => setAsOfDate(e.target.value)} />
              </label>
              <button
                type="button"
                className={`mt-3 ${buttonClassNames("primary")}`}
                disabled={actionLoading.rerun_recommendations}
                onClick={() => {
                  if (!asOfDate) {
                    setActionErrors((prev) => ({ ...prev, rerun_recommendations: "Нужен as_of_date" }));
                    return;
                  }
                  if (!window.confirm("Перезапустить рекомендации ИИ? Возможны вызовы OpenAI и расход средств.")) return;
                  const payload = { as_of_date: asOfDate };
                  void runAction("rerun_recommendations", payload, () => rerunAdminRecommendations(sellerAccountId, payload));
                }}
              >
                {actionLoading.rerun_recommendations ? "Выполняется…" : "Перезапустить рекомендации"}
              </button>
              <ActionResultCard
                result={actionResults.rerun_recommendations}
                error={actionErrors.rerun_recommendations}
                success={actionSuccess.rerun_recommendations}
                requestPayload={actionRequestPayloads.rerun_recommendations}
              />
            </div>
          </div>
        </Card>
      ) : null}
    </main>
  );
}

const ADMIN_TABS: { id: AdminTab; label: string }[] = [
  { id: "overview", label: "Обзор" },
  { id: "sync_import", label: "Синхронизация и импорт" },
  { id: "cursors", label: "Курсоры" },
  { id: "alerts", label: "Алерты" },
  { id: "recommendations", label: "Рекомендации" },
  { id: "chat_logs", label: "Чат" },
  { id: "feedback", label: "Отзывы" },
  { id: "billing", label: "Биллинг" },
  { id: "actions", label: "Действия" },
];

function AdminTabBar({
  activeTab,
  onTabChange,
}: {
  activeTab: AdminTab;
  onTabChange: (t: AdminTab) => void;
}) {
  return (
    <div className="rounded-lg border border-aivion-border bg-aivion-surface-soft/80 p-1.5 shadow-sm">
      <div className="flex flex-wrap gap-1" role="tablist" aria-label="Разделы карточки клиента">
        {ADMIN_TABS.map((t) => {
          const active = activeTab === t.id;
          return (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={active}
              className={[
                "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "border border-aivion-border bg-aivion-surface text-aivion-text shadow-sm"
                  : "border border-transparent text-aivion-muted hover:bg-aivion-surface hover:text-aivion-text",
              ].join(" ")}
              onClick={() => onTabChange(t.id)}
            >
              {t.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function ClientSummaryStrip({ detail }: { detail: AdminClientDetail }) {
  const conn = detail.connections[0];
  const sync = detail.operational_status.latest_sync_job;
  const recRun = detail.operational_status.latest_recommendation_run;
  const chatTrace = detail.operational_status.latest_chat_trace;
  const { failedJobs, sumFailedRecords } = importJobFailureStats(detail);
  const billing = detail.billing;

  return (
    <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-aivion-text">
            {detail.overview.seller_name}{" "}
            <span className="text-aivion-muted-soft">#{detail.overview.seller_account_id}</span>
          </h1>
          <div className="mt-2 flex flex-wrap gap-2">
            <Badge value={detail.overview.seller_status} />
            {conn ? <Badge value={`Seller API: ${conn.status}`} /> : <Badge value="Seller API: —" />}
            {conn ? (
              <Badge
                value={`Performance API: ${conn.performance_connection_status}${conn.performance_token_set ? " · токен есть" : " · токена нет"}`}
              />
            ) : null}
            {billing ? <Badge value={`Биллинг: ${billing.plan_code} / ${billing.status}`} /> : <Badge value="Биллинг: не задан" />}
          </div>
        </div>
      </div>
      <div className="mt-4 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <SummaryCell label="Email владельца" value={detail.overview.owner_email ?? "—"} />
        <SummaryCell
          label="Последняя синхронизация"
          value={sync ? `#${sync.id} · ${sync.status}` : "—"}
          hint={sync ? fmtDate(sync.finished_at ?? sync.started_at) : undefined}
        />
        <SummaryCell
          label="Проблемы импорта (последняя партия)"
          value={
            failedJobs > 0 || sumFailedRecords > 0
              ? `${failedJobs} сбойн. задач, ${sumFailedRecords} сбойн. записей`
              : "В последних задачах импорта сбоев нет"
          }
        />
        <SummaryCell
          label="Последний прогон рекомендаций ИИ"
          value={recRun ? `#${recRun.id} · ${recRun.status}` : "—"}
          hint={
            recRun
              ? `сген.: ${recRun.generated_recommendations_count} · вх/исх ${recRun.input_tokens}/${recRun.output_tokens}${recRun.error_message ? ` · ${recRun.error_message}` : ""}`
              : undefined
          }
        />
        <SummaryCell
          label="Последний трейс чата"
          value={chatTrace ? `#${chatTrace.id} · ${chatTrace.status}` : "—"}
          hint={
            chatTrace
              ? `${chatTrace.detected_intent ?? "намерение —"} · сессия #${chatTrace.session_id}${chatTrace.error_message ? ` · ${chatTrace.error_message}` : ""}`
              : undefined
          }
        />
        <SummaryCell
          label="Открыты алерты / рекомендации"
          value={`${detail.operational_status.open_alerts_count} / ${detail.operational_status.open_recommendations_count}`}
        />
      </div>
    </section>
  );
}

function SummaryCell({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-md border border-aivion-border bg-aivion-surface-soft/80 px-3 py-2">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-aivion-muted-soft">{label}</p>
      <p className="mt-0.5 font-medium text-aivion-text">{value}</p>
      {hint ? <p className="mt-1 text-xs text-aivion-muted">{hint}</p> : null}
    </div>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded border bg-aivion-surface p-4">
      <h2 className="mb-2 text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function CopyJsonButton({ jsonText }: { jsonText: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className={`${buttonClassNames("secondary")} text-xs`}
      onClick={() => {
        void navigator.clipboard.writeText(jsonText).then(() => {
          setDone(true);
          window.setTimeout(() => setDone(false), 2000);
        });
      }}
    >
      {done ? "Скопировано" : "Копировать JSON"}
    </button>
  );
}

function ActionResultCard({
  result,
  error,
  success,
  requestPayload,
}: {
  result: AdminActionLog | null;
  error: string | null;
  success: string | null;
  requestPayload: Record<string, unknown> | null;
}) {
  const resultJson = useMemo(() => {
    if (!result?.result_payload) return "";
    try {
      return JSON.stringify(result.result_payload, null, 2);
    } catch {
      return String(result.result_payload);
    }
  }, [result]);

  return (
    <div className="mt-4 space-y-3 rounded-lg border border-aivion-border bg-aivion-surface-soft/80 p-3 text-sm">
      {error ? <p className="text-sm text-red-700">{error}</p> : null}
      {success ? <p className="text-sm text-green-800">{success}</p> : null}
      {!result ? null : (
        <div className="space-y-2">
          <p>
            <span className="text-aivion-muted">Журнал действия</span>{" "}
            <span className="font-mono font-medium">#{result.id}</span>
          </p>
          <p>
            Статус: <b>{result.status}</b> · завершено {fmtDate(result.finished_at)}
          </p>
          {result.error_message ? <p className="text-red-800">Ошибка: {result.error_message}</p> : null}
          {requestPayload ? (
            <details className="rounded border bg-aivion-surface">
              <summary className="cursor-pointer px-2 py-2 text-xs font-medium text-aivion-text">Тело запроса</summary>
              <div className="border-t border-aivion-border p-2">
                <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded border bg-aivion-surface-soft p-2 text-xs">
                  {JSON.stringify(requestPayload, null, 2)}
                </pre>
              </div>
            </details>
          ) : null}
          <details className="rounded border bg-aivion-surface">
            <summary className="cursor-pointer px-2 py-2 text-xs font-medium text-aivion-text">Тело ответа</summary>
            <div className="border-t border-aivion-border p-2">
              {resultJson ? (
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <CopyJsonButton jsonText={resultJson} />
                </div>
              ) : null}
              <pre className="max-h-96 overflow-auto whitespace-pre-wrap break-words rounded border bg-aivion-surface-soft p-3 text-xs leading-relaxed">
                {resultJson || "{}"}
              </pre>
            </div>
          </details>
        </div>
      )}
    </div>
  );
}

function InternalSupportDataNotice() {
  return (
    <div className="rounded border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
      Внутренние данные поддержки. Не передавать наружу.
    </div>
  );
}

function JsonDetailsBlock({
  title,
  data,
}: {
  title: string;
  data: unknown;
}) {
  const rawUx = jsonTitleNeedsRawAiAuditCopy(title);
  const jsonText = !isEmptyData(data) ? JSON.stringify(data, null, 2) : "";

  return (
    <details className="rounded border bg-aivion-surface p-2">
      <summary className="cursor-pointer text-xs font-medium text-aivion-text">{title}</summary>
      <div className="mt-2 border-t border-aivion-border pt-2">
        {rawUx ? (
          <p className="mb-2 text-xs font-medium text-amber-900">Просмотр сырого ответа ИИ фиксируется в журнале аудита.</p>
        ) : null}
        {isEmptyData(data) ? (
          <p className="text-xs text-aivion-muted">Данных нет.</p>
        ) : (
          <>
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <CopyJsonButton jsonText={jsonText} />
            </div>
            <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded border bg-aivion-surface-soft p-2 text-xs">
              {jsonText}
            </pre>
          </>
        )}
      </div>
    </details>
  );
}

function MetricChip({ label, value }: { label: string; value: ReactNode }) {
  return (
    <span className="inline-flex rounded border border-aivion-border bg-aivion-surface-soft px-2 py-0.5 text-xs">
      {label}: {String(value)}
    </span>
  );
}

function ErrorBox({ message, stage }: { message: string; stage?: string }) {
  return (
    <div className="mt-2 rounded border border-red-300 bg-red-50 p-2 text-xs text-red-900">
      <p className="font-medium">Ошибка</p>
      {stage ? <p>Этап: {stage}</p> : null}
      <p>{message}</p>
    </div>
  );
}

function formatEntity(
  entityType?: string | null,
  entityId?: string | null,
  entitySku?: number | null,
  entityOfferId?: string | null,
): string {
  if (entitySku != null) return `sku:${entitySku}`;
  if (entityOfferId) return `offer:${entityOfferId}`;
  if (entityId) return `id:${entityId}`;
  return entityType ?? "—";
}

function isEmptyData(v: unknown): boolean {
  if (v == null) return true;
  if (typeof v === "string") return v.trim() === "";
  if (Array.isArray(v)) return v.length === 0;
  if (typeof v === "object") return Object.keys(v as Record<string, unknown>).length === 0;
  return false;
}

function Badge({ value }: { value: string }) {
  const v = value.toLowerCase();
  let cls = "border-aivion-border bg-aivion-surface-soft text-aivion-text";
  if (["completed", "valid", "active", "trial", "open"].includes(v)) cls = "border-green-300 bg-green-50 text-green-700";
  if (["running", "pending"].includes(v)) cls = "border-blue-300 bg-blue-50 text-blue-700";
  if (["failed", "error", "invalid", "past_due"].includes(v)) cls = "border-red-300 bg-red-50 text-red-700";
  if (["internal", "missing", "unknown", "no billing", "no connection"].includes(v)) cls = "border-yellow-300 bg-yellow-50 text-yellow-700";
  return <span className={`inline-flex rounded border px-2 py-0.5 text-xs ${cls}`}>{value}</span>;
}

function fmtDate(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}
