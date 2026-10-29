"use client";

import Link from "next/link";
import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { buttonClassNames } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { EvidenceList } from "@/components/ui/evidence-list";
import { LoadingState } from "@/components/ui/loading-state";
import { PageHeader } from "@/components/ui/page-header";
import { getAlertsSummary, type AlertsSummaryResponse } from "@/lib/alerts-api";
import { alertGroupLabel, alertTypeLabel } from "@/lib/alert-labels";
import { recommendationTypeLabel } from "@/lib/recommendation-labels";
import {
  MVP_RECOMMENDATION_TYPES,
  acceptRecommendation,
  dismissRecommendation,
  generateRecommendations,
  getRecommendationDetail,
  getRecommendations,
  getRecommendationsSummary,
  resolveRecommendation,
  type GenerateRecommendationsResponse,
  type RecommendationDetail,
  type RecommendationItem,
  type RecommendationsSummary,
} from "@/lib/recommendations-api";

const DEFAULT_LIMIT = 50;

const PRIORITY_ORDER = ["critical", "high", "medium", "low"] as const;

type FilterState = {
  status: "" | "open" | "accepted" | "dismissed" | "resolved";
  recommendationTypeSelect: string;
  recommendationTypeText: string;
  priority_level: "" | "low" | "medium" | "high" | "critical";
  confidence_level: "" | "low" | "medium" | "high";
  horizon: "" | "short_term" | "medium_term" | "long_term";
  entity_type: "" | "account" | "sku" | "product" | "campaign" | "pricing_constraint";
  limit: number;
  offset: number;
};

type QuickFilterId = "open" | "critical" | "high" | "short_term";

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

function fmtDateShort(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString();
}

function fmtEntityRec(row: Pick<RecommendationItem, "entity_sku" | "entity_offer_id" | "entity_id" | "entity_type">): string {
  if (row.entity_sku != null && row.entity_sku !== undefined) {
    return `SKU: ${row.entity_sku}`;
  }
  if (row.entity_offer_id) return `Предложение: ${row.entity_offer_id}`;
  if (row.entity_id) return `ИД: ${row.entity_id}`;
  return translateEntityTypeRec(row.entity_type);
}

function translateEntityTypeRec(t: string): string {
  const m: Record<string, string> = {
    account: "Аккаунт",
    sku: "SKU",
    product: "Товар",
    campaign: "Кампания",
    pricing_constraint: "Устаревший тип",
  };
  return m[t] ?? t;
}

function translateRecoStatus(s: string): string {
  const m: Record<string, string> = {
    open: "открыта",
    accepted: "принята",
    dismissed: "отклонена",
    resolved: "закрыта",
  };
  return m[s] ?? s;
}

function translatePriorityLabel(level: string): string {
  const m: Record<string, string> = {
    critical: "критический",
    high: "высокий",
    medium: "средний",
    low: "низкий",
    other: "прочее",
  };
  return m[level] ?? level;
}

function translateAlertSeverityUi(s: string): string {
  const m: Record<string, string> = {
    low: "Низкая",
    medium: "Средняя",
    high: "Высокая",
    critical: "Критическая",
  };
  return m[s] ?? s;
}

function translateAlertGroupUi(g: string): string {
  return alertGroupLabel(g);
}

function effectiveRecommendationType(f: FilterState): string | undefined {
  const text = f.recommendationTypeText.trim();
  if (text) return text;
  if (f.recommendationTypeSelect) return f.recommendationTypeSelect;
  return undefined;
}

function groupRecommendationsByPriority(items: RecommendationItem[]): { level: string; rows: RecommendationItem[] }[] {
  const by = new Map<string, RecommendationItem[]>();
  for (const row of items) {
    const level = row.priority_level || "other";
    const list = by.get(level) ?? [];
    list.push(row);
    by.set(level, list);
  }
  for (const list of by.values()) {
    list.sort((a, b) => {
      const hz = horizonRank(a.horizon) - horizonRank(b.horizon);
      if (hz !== 0) return hz;
      return b.priority_score - a.priority_score;
    });
  }
  const seen = new Set<string>();
  const levels: string[] = [];
  for (const p of PRIORITY_ORDER) {
    if ((by.get(p) ?? []).length > 0) {
      levels.push(p);
      seen.add(p);
    }
  }
  for (const k of by.keys()) {
    if (!seen.has(k)) {
      levels.push(k);
      seen.add(k);
    }
  }
  return levels.map((level) => ({ level, rows: by.get(level) ?? [] }));
}

function horizonRank(h: string): number {
  if (h === "short_term") return 0;
  if (h === "medium_term") return 1;
  if (h === "long_term") return 2;
  return 3;
}

function matchesQuickFilter(id: QuickFilterId, f: FilterState): boolean {
  const recType = effectiveRecommendationType(f);
  if (recType || f.confidence_level || f.entity_type) return false;
  if (f.status !== "open") return false;
  if (id === "open") return !f.priority_level && !f.horizon;
  if (id === "critical") return f.priority_level === "critical" && !f.horizon;
  if (id === "high") return f.priority_level === "high" && !f.horizon;
  if (id === "short_term") return f.horizon === "short_term" && !f.priority_level;
  return false;
}

function quickFilterPreset(id: QuickFilterId): Partial<FilterState> {
  const base: Partial<FilterState> = {
    status: "open",
    recommendationTypeSelect: "",
    recommendationTypeText: "",
    confidence_level: "",
    entity_type: "",
    offset: 0,
  };
  if (id === "open") return { ...base, priority_level: "", horizon: "" };
  if (id === "critical") return { ...base, priority_level: "critical", horizon: "" };
  if (id === "high") return { ...base, priority_level: "high", horizon: "" };
  return { ...base, priority_level: "", horizon: "short_term" };
}

function extractValidationWarnings(detail: RecommendationDetail): string[] {
  const out: string[] = [];
  if (detail.validation_warnings?.length) {
    out.push(...detail.validation_warnings);
  }
  const sm = detail.supporting_metrics_payload;
  if (sm && typeof sm === "object") {
    for (const key of ["warnings", "validation_warnings", "validator_warnings"] as const) {
      const v = (sm as Record<string, unknown>)[key];
      if (Array.isArray(v)) {
        for (const x of v) {
          if (typeof x === "string") out.push(x);
        }
      }
    }
  }
  return [...new Set(out)];
}

function friendlyGenerateMessage(raw: string): string {
  const s = raw.toLowerCase();
  if (s.includes("503") || s.includes("502") || s.includes("openai") || s.includes("unauthorized")) {
    return "Генерация не удалась — служба ИИ может быть настроена неверно или временно недоступна.";
  }
  if (raw.length > 220) {
    return "Генерация не удалась — см. чеклист ниже и при необходимости логи сервера.";
  }
  return raw;
}

export default function RecommendationsScreen({
  initialFocusRecommendationId,
}: {
  initialFocusRecommendationId?: number;
}) {
  const [alertsSummary, setAlertsSummary] = useState<AlertsSummaryResponse | null>(null);
  const [summary, setSummary] = useState<RecommendationsSummary | null>(null);
  const [loadingPrerequisites, setLoadingPrerequisites] = useState(true);
  const [prerequisitesError, setPrerequisitesError] = useState<string | null>(null);

  const [items, setItems] = useState<RecommendationItem[]>([]);
  const [loadingList, setLoadingList] = useState(true);
  const [listError, setListError] = useState<string | null>(null);
  const [filters, setFilters] = useState<FilterState>({
    status: "",
    recommendationTypeSelect: "",
    recommendationTypeText: "",
    priority_level: "",
    confidence_level: "",
    horizon: "",
    entity_type: "",
    limit: DEFAULT_LIMIT,
    offset: 0,
  });

  const [detailId, setDetailId] = useState<number | null>(null);
  const [detail, setDetail] = useState<RecommendationDetail | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const [generateAsOf, setGenerateAsOf] = useState("");
  const [generateLoading, setGenerateLoading] = useState(false);
  const [generateMessage, setGenerateMessage] = useState<string | null>(null);
  const [generateError, setGenerateError] = useState<string | null>(null);
  const [lastGenerateResult, setLastGenerateResult] = useState<GenerateRecommendationsResponse | null>(null);

  const [actionLoadingId, setActionLoadingId] = useState<number | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const loadPrerequisites = useCallback(async () => {
    setLoadingPrerequisites(true);
    setPrerequisitesError(null);
    try {
      const [alerts, rec] = await Promise.all([getAlertsSummary(), getRecommendationsSummary()]);
      setAlertsSummary(alerts);
      setSummary(rec);
    } catch (e: unknown) {
      setAlertsSummary(null);
      setSummary(null);
      setPrerequisitesError(e instanceof Error ? e.message : "Не удалось загрузить предварительные данные");
    } finally {
      setLoadingPrerequisites(false);
    }
  }, []);

  const loadList = useCallback(async () => {
    setLoadingList(true);
    setListError(null);
    try {
      const recType = effectiveRecommendationType(filters);
      const data = await getRecommendations({
        status: filters.status || undefined,
        recommendation_type: recType,
        priority_level: filters.priority_level || undefined,
        confidence_level: filters.confidence_level || undefined,
        horizon: filters.horizon || undefined,
        entity_type: filters.entity_type || undefined,
        limit: filters.limit,
        offset: filters.offset,
      });
      setItems(data.items);
    } catch (e: unknown) {
      setItems([]);
      setListError(e instanceof Error ? e.message : "Не удалось загрузить рекомендации");
    } finally {
      setLoadingList(false);
    }
  }, [filters]);

  useEffect(() => {
    void loadPrerequisites();
  }, [loadPrerequisites]);

  useEffect(() => {
    void loadList();
  }, [loadList]);

  useEffect(() => {
    if (initialFocusRecommendationId == null || initialFocusRecommendationId <= 0) {
      return;
    }
    setDetailId(initialFocusRecommendationId);
  }, [initialFocusRecommendationId]);

  useEffect(() => {
    if (detailId == null) {
      setDetail(null);
      setDetailError(null);
      return;
    }
    let cancelled = false;
    setLoadingDetail(true);
    setDetailError(null);
    void getRecommendationDetail(detailId)
      .then((d) => {
        if (!cancelled) {
          setDetail(d);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setDetail(null);
          setDetailError(e instanceof Error ? e.message : "Не удалось загрузить детали");
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingDetail(false);
      });
    return () => {
      cancelled = true;
    };
  }, [detailId]);

  useEffect(() => {
    if (
      initialFocusRecommendationId == null ||
      detailId !== initialFocusRecommendationId ||
      loadingList ||
      typeof document === "undefined"
    ) {
      return;
    }
    if (!items.some((r) => r.id === detailId)) {
      return;
    }
    const el = document.getElementById(`recommendation-row-${detailId}`);
    if (el) {
      requestAnimationFrame(() => {
        el.scrollIntoView({ block: "nearest", behavior: "smooth" });
      });
    }
  }, [initialFocusRecommendationId, detailId, items, loadingList]);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadPrerequisites(), loadList()]);
    if (detailId != null) {
      try {
        const d = await getRecommendationDetail(detailId);
        setDetail(d);
        setDetailError(null);
      } catch (e: unknown) {
        setDetailError(e instanceof Error ? e.message : "Не удалось обновить детали");
      }
    }
  }, [detailId, loadList, loadPrerequisites]);

  async function handleGenerate() {
    setGenerateLoading(true);
    setGenerateMessage(null);
    setGenerateError(null);
    setLastGenerateResult(null);
    try {
      const payload =
        generateAsOf.trim() === ""
          ? undefined
          : { as_of_date: generateAsOf.trim() };
      const res = await generateRecommendations(payload);
      setLastGenerateResult(res);
      setGenerateMessage("Генерация завершена. Сводка ниже.");
      await refreshAll();
    } catch (e: unknown) {
      const raw = e instanceof Error ? e.message : "Генерация не удалась";
      setGenerateError(friendlyGenerateMessage(raw));
    } finally {
      setGenerateLoading(false);
    }
  }

  async function runRowAction(
    id: number,
    kind: "accept" | "dismiss" | "resolve",
  ): Promise<void> {
    setActionLoadingId(id);
    setActionMessage(null);
    setActionError(null);
    try {
      let updated: RecommendationItem;
      if (kind === "accept") updated = await acceptRecommendation(id);
      else if (kind === "dismiss") updated = await dismissRecommendation(id);
      else updated = await resolveRecommendation(id);
      setActionMessage(`Рекомендация №${id}: статус «${translateRecoStatus(updated.status)}».`);
      await Promise.all([loadPrerequisites(), loadList()]);
      if (detailId === id) {
        try {
          const d = await getRecommendationDetail(id);
          setDetail(d);
          setDetailError(null);
        } catch (e: unknown) {
          setDetailError(e instanceof Error ? e.message : "Не удалось обновить детали");
        }
      }
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : "Действие не выполнено");
    } finally {
      setActionLoadingId(null);
    }
  }

  const applyQuickFilter = useCallback((id: QuickFilterId) => {
    setFilters((s) => ({ ...s, ...quickFilterPreset(id) }));
  }, []);

  const typeSelectOptions = useMemo((): [string, string][] => {
    const rows: [string, string][] = MVP_RECOMMENDATION_TYPES.map((t) => [
      t,
      recommendationTypeLabel(t),
    ]);
    return [["", "Все"], ...rows];
  }, []);

  const groupedItems = useMemo(() => groupRecommendationsByPriority(items), [items]);

  const alertsRun = alertsSummary?.latest_run;
  const recRun = summary?.latest_run;
  const noOpenAlerts = alertsSummary != null && (alertsSummary.open_total ?? 0) === 0;
  const listIsEmpty = !loadingList && !listError && items.length === 0;
  const defaultishFilters =
    filters.offset === 0 &&
    !effectiveRecommendationType(filters) &&
    !filters.confidence_level &&
    !filters.entity_type;

  const showFocusRecommendationMissing = useMemo(() => {
    return (
      initialFocusRecommendationId != null &&
      detailId === initialFocusRecommendationId &&
      !loadingList &&
      !listError &&
      !loadingDetail &&
      detailError == null &&
      detail != null &&
      !items.some((r) => r.id === initialFocusRecommendationId)
    );
  }, [
    initialFocusRecommendationId,
    detailId,
    loadingList,
    listError,
    loadingDetail,
    detailError,
    detail,
    items,
  ]);

  function resetFocusFilters() {
    setFilters({
      status: "",
      recommendationTypeSelect: "",
      recommendationTypeText: "",
      priority_level: "",
      confidence_level: "",
      horizon: "",
      entity_type: "",
      limit: DEFAULT_LIMIT,
      offset: 0,
    });
  }

  return (
    <main className="space-y-6 p-6">
      <PageHeader
        title="Рекомендации"
        subtitle="AI-движок рекомендаций: предпосылки, запуски генерации, проверенный результат и действия со статусом."
      />

      <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
        <h2 className="text-lg font-semibold text-aivion-text">Перед генерацией</h2>
        <p className="mt-1 text-sm text-aivion-muted">
          Типичный порядок: <strong>синхронизация</strong> → <strong>метрики / дашборд</strong> →{" "}
          <Link href="/app/alerts" className="text-blue-700 underline">
            запуск оповещений
          </Link>{" "}
          → затем генерация рекомендаций здесь.
        </p>
        {prerequisitesError ? (
          <p className="mt-2 text-sm text-amber-900" role="alert">
            {prerequisitesError}
          </p>
        ) : null}
        {loadingPrerequisites ? (
          <div className="mt-4">
            <LoadingState message="Загрузка статуса оповещений и рекомендаций…" />
          </div>
        ) : (
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <div className="rounded-lg border border-aivion-border bg-aivion-surface-soft/80 p-3">
              <h3 className="text-sm font-semibold text-aivion-text">Последний запуск оповещений</h3>
              {!alertsRun ? (
                <p className="mt-2 text-sm text-aivion-muted">
                  Запусков оповещений ещё не было. Откройте «Оповещения» и сначала запустите задачу.
                </p>
              ) : (
                <ul className="mt-2 space-y-1 text-sm text-aivion-text">
                  <li>
                    <span className="text-aivion-muted">Статус:</span>{" "}
                    <RunStatusBadge status={alertsRun.status} />
                  </li>
                  <li>
                    <span className="text-aivion-muted">ИД запуска:</span> {alertsRun.id}
                  </li>
                  <li>
                    <span className="text-aivion-muted">Завершён:</span> {fmtDate(alertsRun.finished_at)}
                  </li>
                  <li>
                    <span className="text-aivion-muted">Открытых оповещений:</span> {alertsSummary?.open_total ?? "—"}
                  </li>
                  {alertsRun.error_message ? (
                    <li className="text-amber-900">
                      <span className="font-medium">Ошибка:</span> {alertsRun.error_message}
                    </li>
                  ) : null}
                </ul>
              )}
            </div>
            <div className="rounded-lg border border-aivion-border bg-aivion-surface-soft/80 p-3">
              <h3 className="text-sm font-semibold text-aivion-text">Последний запуск рекомендаций</h3>
              {!recRun ? (
                <p className="mt-2 text-sm text-aivion-muted">Запусков рекомендаций ещё не было.</p>
              ) : (
                <ul className="mt-2 space-y-1 text-sm text-aivion-text">
                  <li>
                    <span className="text-aivion-muted">Статус:</span> <RunStatusBadge status={recRun.status} />
                  </li>
                  <li>
                    <span className="text-aivion-muted">На дату:</span> {recRun.as_of_date ?? "—"}
                  </li>
                  <li>
                    <span className="text-aivion-muted">Сгенерировано (последний запуск):</span>{" "}
                    {recRun.generated_recommendations_count}
                  </li>
                  <li>
                    <span className="text-aivion-muted">Токены:</span> {recRun.input_tokens} / {recRun.output_tokens} /{" "}
                    {recRun.total_tokens}
                  </li>
                  <li>
                    <span className="text-aivion-muted">Оценка стоимости:</span>{" "}
                    {recRun.estimated_cost != null ? recRun.estimated_cost.toFixed(4) : "—"}
                  </li>
                  {recRun.error_message ? (
                    <li className="text-amber-900">
                      <span className="font-medium">Ошибка:</span> {recRun.error_message}
                    </li>
                  ) : null}
                </ul>
              )}
            </div>
          </div>
        )}
        <div className="mt-4 flex flex-wrap gap-2 text-sm">
          <Link href="/app/dashboard" className={buttonClassNames("secondary")}>
            Дашборд
          </Link>
          <Link href="/app/alerts" className={buttonClassNames("secondary")}>
            Оповещения
          </Link>
          <Link href="/app/products" className={buttonClassNames("secondary")}>
            Товары
          </Link>
        </div>
      </section>

      {!loadingPrerequisites && summary ? (
        <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
          <h2 className="text-lg font-semibold text-aivion-text">Сводные счётчики</h2>
          <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
            <SummaryCard label="Открытые" value={summary.open_total} />
            <SummaryCard label="Критические" value={summary.by_priority.critical} />
            <SummaryCard label="Высокие" value={summary.by_priority.high} />
            <SummaryCard label="Средние" value={summary.by_priority.medium} />
            <SummaryCard label="Низкие" value={summary.by_priority.low} />
            <SummaryCard label="Уверенность: высокая" value={summary.by_confidence.high} />
            <SummaryCard label="Уверенность: средняя" value={summary.by_confidence.medium} />
            <SummaryCard label="Уверенность: низкая" value={summary.by_confidence.low} />
          </div>
        </section>
      ) : null}

      <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
        <h2 className="text-lg font-semibold text-aivion-text">Сгенерировать рекомендации</h2>
        <p className="mt-1 text-sm text-aivion-muted">
          Использует текущий контекст (алерты, метрики, цены). Необязательная дата отчёта фиксирует расчётный день.
        </p>
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <label className="text-sm">
            <span className="mb-1 block text-aivion-text">Дата отчёта</span>
            <input
              className="rounded-lg border border-aivion-border px-2 py-2"
              type="date"
              value={generateAsOf}
              onChange={(e) => setGenerateAsOf(e.target.value)}
              disabled={generateLoading}
            />
          </label>
          <button
            type="button"
            disabled={generateLoading}
            className={buttonClassNames("primary")}
            onClick={() => void handleGenerate()}
          >
            {generateLoading ? "Генерация…" : "Сгенерировать рекомендации"}
          </button>
        </div>
        {generateLoading ? (
          <div className="mt-4">
            <LoadingState message="Запуск AI-генерации рекомендаций…" />
          </div>
        ) : null}
        {generateMessage ? <p className="mt-3 text-sm text-green-800">{generateMessage}</p> : null}
        {generateError ? (
          <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-950" role="alert">
            <p className="font-medium">{generateError}</p>
            <p className="mt-2 font-medium text-amber-900">Чеклист</p>
            <ul className="mt-1 list-disc space-y-1 pl-5 text-amber-950">
              <li>Настроен ли ключ OpenAI API для развёртывания?</li>
              <li>Есть ли оповещения на выбранную дату (сначала запустите оповещения)?</li>
              <li>Заданы ли ограничения цен там, где они нужны для ценовых рекомендаций (необязательно)?</li>
              <li>Превышен ли бюджет контекста / токенов? Попробуйте другую дату или меньше открытых оповещений.</li>
              <li>Валидация отклоняет все элементы? Проверьте причины в логах сервера.</li>
            </ul>
          </div>
        ) : null}
        {lastGenerateResult ? (
          <div className="mt-4 rounded-lg border border-aivion-border bg-aivion-surface-soft p-4">
            <h3 className="text-sm font-semibold text-aivion-text">Результат последнего запуска</h3>
            <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
              <div>
                <dt className="text-aivion-muted">ИД запуска</dt>
                <dd className="font-mono font-medium text-aivion-text">{lastGenerateResult.run_id}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Дата отчёта</dt>
                <dd className="font-medium text-aivion-text">{fmtDateShort(lastGenerateResult.as_of_date)}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Сгенерировано</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.generated_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Валидные</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.valid_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Отклонено</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.rejected_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Сохранено (upsert)</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.upserted_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Связанные оповещения</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.linked_alerts_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Предупреждения (элементы)</dt>
                <dd className="font-medium text-aivion-text">{lastGenerateResult.warnings_total}</dd>
              </div>
              <div>
                <dt className="text-aivion-muted">Токены (вход / выход / всего)</dt>
                <dd className="font-mono text-aivion-text">
                  {lastGenerateResult.input_tokens} / {lastGenerateResult.output_tokens} /{" "}
                  {lastGenerateResult.total_tokens}
                </dd>
              </div>
              {lastGenerateResult.estimated_cost != null ? (
                <div>
                  <dt className="text-aivion-muted">Оценка стоимости</dt>
                  <dd className="font-medium text-aivion-text">{lastGenerateResult.estimated_cost}</dd>
                </div>
              ) : null}
            </dl>
          </div>
        ) : null}
      </section>

      <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
        <h2 className="mb-2 text-lg font-semibold text-aivion-text">Быстрые фильтры</h2>
        <p className="mb-3 text-sm text-aivion-muted">Сузить список (статус = открытые + один параметр).</p>
        <div className="flex flex-wrap gap-2">
          {(["open", "critical", "high", "short_term"] as const).map((id) => (
            <button
              key={id}
              type="button"
              className={
                matchesQuickFilter(id, filters)
                  ? `${buttonClassNames("primary")} ring-2 ring-blue-300`
                  : buttonClassNames("secondary")
              }
              onClick={() => applyQuickFilter(id)}
            >
              {id === "open"
                ? "Открытые"
                : id === "critical"
                  ? "Критические"
                  : id === "high"
                    ? "Высокие"
                    : "Краткосрочные"}
            </button>
          ))}
        </div>

        <details className="mt-4 rounded-lg border border-aivion-border bg-aivion-surface-soft/60 p-3">
          <summary className="cursor-pointer text-sm font-medium text-aivion-text">Расширенные фильтры</summary>
          <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-3 xl:grid-cols-6">
            <Select
              label="Статус"
              value={filters.status}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  status: v as FilterState["status"],
                  offset: 0,
                }))
              }
              options={[
                ["", "Все"],
                ["open", "Открыта"],
                ["accepted", "Принята"],
                ["dismissed", "Отклонена"],
                ["resolved", "Закрыта"],
              ]}
            />
            <Select
              label="Тип (из списка)"
              value={filters.recommendationTypeSelect}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  recommendationTypeSelect: v,
                  offset: 0,
                }))
              }
              options={typeSelectOptions}
            />
            <label className="text-sm md:col-span-2">
              <span className="mb-1 block text-aivion-text">Тип (свободный ввод)</span>
              <input
                className="w-full rounded border px-2 py-1"
                type="text"
                placeholder="напр. replenish_sku"
                value={filters.recommendationTypeText}
                onChange={(e) =>
                  setFilters((s) => ({
                    ...s,
                    recommendationTypeText: e.target.value,
                    offset: 0,
                  }))
                }
              />
            </label>
            <Select
              label="Приоритет"
              value={filters.priority_level}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  priority_level: v as FilterState["priority_level"],
                  offset: 0,
                }))
              }
              options={[
                ["", "Все"],
                ["low", "Низкий"],
                ["medium", "Средний"],
                ["high", "Высокий"],
                ["critical", "Критический"],
              ]}
            />
            <Select
              label="Уверенность"
              value={filters.confidence_level}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  confidence_level: v as FilterState["confidence_level"],
                  offset: 0,
                }))
              }
              options={[
                ["", "Все"],
                ["low", "Низкая"],
                ["medium", "Средняя"],
                ["high", "Высокая"],
              ]}
            />
            <Select
              label="Горизонт"
              value={filters.horizon}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  horizon: v as FilterState["horizon"],
                  offset: 0,
                }))
              }
              options={[
                ["", "Все"],
                ["short_term", "Краткосрочный"],
                ["medium_term", "Среднесрочный"],
                ["long_term", "Долгосрочный"],
              ]}
            />
            <Select
              label="Тип сущности"
              value={filters.entity_type}
              onChange={(v) =>
                setFilters((s) => ({
                  ...s,
                  entity_type: v as FilterState["entity_type"],
                  offset: 0,
                }))
              }
              options={[
                ["", "Все"],
                ["account", "Аккаунт"],
                ["sku", "SKU"],
                ["product", "Товар"],
                ["campaign", "Кампания"],
              ]}
            />
            <label className="text-sm">
              <span className="mb-1 block text-aivion-text">Лимит</span>
              <input
                className="w-full rounded border px-2 py-1"
                type="number"
                min={1}
                max={200}
                value={filters.limit}
                onChange={(e) =>
                  setFilters((s) => ({
                    ...s,
                    limit: Math.max(1, Math.min(200, Number(e.target.value) || DEFAULT_LIMIT)),
                    offset: 0,
                  }))
                }
              />
            </label>
          </div>
        </details>
      </section>

      {listError ? (
        <p className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">{listError}</p>
      ) : null}
      {actionError ? <p className="text-sm text-red-700">{actionError}</p> : null}
      {actionMessage ? <p className="text-sm text-green-800">{actionMessage}</p> : null}

      {showFocusRecommendationMissing ? (
        <div
          className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950"
          role="status"
        >
          <p className="font-medium">Выбранная рекомендация не в текущем списке</p>
          <p className="mt-1 text-amber-900">
            Рекомендация №{initialFocusRecommendationId} открыта в панели деталей, но отсутствует в отфильтрованном
            списке. Сбросьте фильтры, чтобы попытаться найти её здесь.
          </p>
          <button type="button" className={`${buttonClassNames("secondary")} mt-3`} onClick={resetFocusFilters}>
            Сбросить фильтры и пагинацию
          </button>
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
          <h2 className="mb-3 text-lg font-semibold text-aivion-text">Рекомендации</h2>
          {loadingList ? (
            <LoadingState message="Загрузка рекомендаций…" />
          ) : listIsEmpty ? (
            <EmptyState
              title="Нет рекомендаций"
              message={
                defaultishFilters
                  ? "Ничего не подходит под фильтры или генерация ещё не создала записи."
                  : "Нет строк для этих фильтров. Ослабьте фильтры или сбросьте быстрые фильтры."
              }
              action={
                <div className="flex flex-col items-center gap-2 sm:flex-row">
                  <button type="button" className={buttonClassNames("primary")} onClick={() => void handleGenerate()}>
                    Сгенерировать рекомендации
                  </button>
                  {noOpenAlerts ? (
                    <Link href="/app/alerts" className={buttonClassNames("secondary")}>
                      Сначала запустите оповещения
                    </Link>
                  ) : null}
                </div>
              }
            />
          ) : (
            <div className="space-y-6">
              {groupedItems.map(({ level, rows }) => (
                <Fragment key={level}>
                  <h3 className="border-b pb-1 text-sm font-semibold uppercase tracking-wide text-aivion-text">
                    Приоритет: {translatePriorityLabel(level)}{" "}
                    <span className="font-normal normal-case text-aivion-muted-soft">({rows.length})</span>
                  </h3>
                  <ul className="space-y-3">
                    {rows.map((row) => (
                      <li
                        id={`recommendation-row-${row.id}`}
                        key={row.id}
                        className={`rounded-lg border p-3 transition-colors ${
                          detailId === row.id ? "border-blue-400 bg-blue-50/50" : "border-aivion-border bg-aivion-surface-soft/40"
                        }`}
                      >
                        <button
                          type="button"
                          className="w-full text-left"
                          onClick={() => setDetailId(row.id)}
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <PriorityBadge level={row.priority_level} />
                            <HorizonBadge horizon={row.horizon} />
                            <ConfidenceBadge level={row.confidence_level} />
                            <UrgencyBadge urgency={row.urgency} />
                            <StatusBadge status={row.status} />
                            <span className="text-xs text-aivion-muted-soft">#{row.id}</span>
                          </div>
                          <p className="mt-2 font-medium text-aivion-text">{row.title}</p>
                          <p className="mt-1 line-clamp-2 text-sm text-aivion-text">{row.what_happened}</p>
                          <p className="mt-1 text-xs text-aivion-muted">{fmtEntityRec(row)}</p>
                          <p className="mt-1 text-xs text-aivion-muted-soft">Обновлено {fmtDate(row.last_seen_at)}</p>
                        </button>
                        <div className="mt-3 flex flex-wrap gap-2 border-t border-aivion-border pt-3">
                          <button
                            type="button"
                            className={buttonClassNames("secondary")}
                            onClick={() => setDetailId(row.id)}
                          >
                            Детали
                          </button>
                          {row.status === "open" ? (
                            <>
                              <button
                                type="button"
                                disabled={actionLoadingId === row.id}
                                className={buttonClassNames("primary")}
                                onClick={() => void runRowAction(row.id, "accept")}
                              >
                                {actionLoadingId === row.id ? "…" : "Принять"}
                              </button>
                              <button
                                type="button"
                                disabled={actionLoadingId === row.id}
                                className={buttonClassNames("secondary")}
                                onClick={() => void runRowAction(row.id, "dismiss")}
                              >
                                Отклонить
                              </button>
                              <button
                                type="button"
                                disabled={actionLoadingId === row.id}
                                className={buttonClassNames("secondary")}
                                onClick={() => void runRowAction(row.id, "resolve")}
                              >
                                Закрыть
                              </button>
                            </>
                          ) : (
                            <span className="self-center text-xs text-aivion-muted-soft">
                              Статус: {translateRecoStatus(row.status)} — действия недоступны
                            </span>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                </Fragment>
              ))}
            </div>
          )}
          <div className="mt-4 flex items-center gap-2">
            <button
              type="button"
              disabled={filters.offset === 0 || loadingList}
              className={buttonClassNames("secondary")}
              onClick={() =>
                setFilters((s) => ({
                  ...s,
                  offset: Math.max(0, s.offset - s.limit),
                }))
              }
            >
              Назад
            </button>
            <button
              type="button"
              disabled={loadingList || items.length < filters.limit}
              className={buttonClassNames("secondary")}
              onClick={() =>
                setFilters((s) => ({
                  ...s,
                  offset: s.offset + s.limit,
                }))
              }
            >
              Вперёд
            </button>
            <span className="text-sm text-aivion-muted">
              смещение {filters.offset}, лимит {filters.limit}
            </span>
          </div>
        </section>

        <section className="rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm">
          <h2 className="mb-3 text-lg font-semibold text-aivion-text">Детали</h2>
          {detailId == null ? (
            <EmptyState
              title="Строка не выбрана"
              message="Выберите рекомендацию из списка, чтобы просмотреть поля, метрики и связанные оповещения."
            />
          ) : loadingDetail ? (
            <LoadingState message="Загрузка деталей рекомендации…" />
          ) : detailError ? (
            <p className="text-sm text-red-700">{detailError}</p>
          ) : !detail ? (
            <p className="text-sm text-aivion-muted">Нет данных.</p>
          ) : (
            <DetailPanel
              detail={detail}
              actionLoadingId={actionLoadingId}
              onAccept={() => void runRowAction(detail.id, "accept")}
              onDismiss={() => void runRowAction(detail.id, "dismiss")}
              onResolve={() => void runRowAction(detail.id, "resolve")}
            />
          )}
        </section>
      </div>

      {!loadingPrerequisites && alertsSummary && alertsSummary.open_total === 0 ? (
        <section className="rounded-lg border border-dashed border-amber-200 bg-amber-50/60 p-4 text-sm text-amber-950">
          <p className="font-medium">Нет открытых оповещений</p>
          <p className="mt-1">
            Рекомендации строятся на основе контекста оповещений.{" "}
            <Link href="/app/alerts" className="font-medium text-blue-800 underline">
              Запустите оповещения
            </Link>{" "}
            для выбранной даты, прежде чем ожидать содержательный ответ ИИ.
          </p>
        </section>
      ) : null}
    </main>
  );
}

function DetailPanel({
  detail,
  actionLoadingId,
  onAccept,
  onDismiss,
  onResolve,
}: {
  detail: RecommendationDetail;
  actionLoadingId: number | null;
  onAccept: () => void;
  onDismiss: () => void;
  onResolve: () => void;
}) {
  const validationWarnings = extractValidationWarnings(detail);

  return (
    <div className="space-y-4 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <PriorityBadge level={detail.priority_level} />
        <HorizonBadge horizon={detail.horizon} />
        <ConfidenceBadge level={detail.confidence_level} />
        <UrgencyBadge urgency={detail.urgency} />
        <StatusBadge status={detail.status} />
        <span className="text-xs text-aivion-muted-soft">идентификатор {detail.id}</span>
      </div>

      {validationWarnings.length > 0 ? (
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-3">
          <h3 className="font-semibold text-amber-950">Предупреждения валидации</h3>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-amber-950">
            {validationWarnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </div>
      ) : null}

      {detail.status === "open" ? (
        <div className="rounded-lg border border-aivion-border bg-aivion-surface-soft p-3">
          <p className="mb-2 text-xs text-aivion-muted">
            Действия в этом разделе только меняют статус рекомендации.
          </p>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              disabled={actionLoadingId === detail.id}
              className={buttonClassNames("primary")}
              onClick={onAccept}
            >
              {actionLoadingId === detail.id ? "Подождите…" : "Принять"}
            </button>
            <button
              type="button"
              disabled={actionLoadingId === detail.id}
              className={buttonClassNames("secondary")}
              onClick={onDismiss}
            >
              Отклонить
            </button>
            <button
              type="button"
              disabled={actionLoadingId === detail.id}
              className={buttonClassNames("secondary")}
              onClick={onResolve}
            >
              Закрыть
            </button>
          </div>
        </div>
      ) : null}

      <section>
        <h3 className="font-semibold text-aivion-text">Что произошло</h3>
        <p className="mt-1 whitespace-pre-wrap text-aivion-text">{detail.what_happened}</p>
      </section>
      <section>
        <h3 className="font-semibold text-aivion-text">Почему это важно</h3>
        <p className="mt-1 whitespace-pre-wrap text-aivion-text">{detail.why_it_matters}</p>
      </section>
      <section>
        <h3 className="font-semibold text-aivion-text">Рекомендуемое действие</h3>
        <p className="mt-1 whitespace-pre-wrap text-aivion-text">{detail.recommended_action}</p>
      </section>
      <section>
        <h3 className="font-semibold text-aivion-text">Ожидаемый эффект</h3>
        <p className="mt-1 whitespace-pre-wrap text-aivion-text">{detail.expected_effect ?? "—"}</p>
      </section>
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wide text-aivion-muted">Приоритет</h3>
          <p className="text-aivion-text">{detail.priority_score.toFixed(1)}</p>
        </div>
      <section>
        <h3 className="font-semibold text-aivion-text">Метки времени</h3>
        <ul className="mt-1 list-inside list-disc text-aivion-text">
          <li>впервые замечено: {fmtDate(detail.first_seen_at)}</li>
          <li>последний раз: {fmtDate(detail.last_seen_at)}</li>
          <li>принято: {fmtDate(detail.accepted_at)}</li>
          <li>отклонено: {fmtDate(detail.dismissed_at)}</li>
          <li>закрыто: {fmtDate(detail.resolved_at)}</li>
        </ul>
      </section>
      <section>
        <h3 className="font-semibold text-aivion-text">Поддерживающие метрики</h3>
        <EvidenceList
          payload={detail.supporting_metrics_payload}
          emptyLabel="Нет поддерживающих метрик."
          className="mt-2"
        />
      </section>
      <section>
        <h3 className="font-semibold text-aivion-text">Проверенные ограничения</h3>
        <ConstraintHints payload={detail.constraints_payload} />
        <EvidenceList
          payload={detail.constraints_payload}
          emptyLabel="Нет данных об ограничениях."
          className="mt-2"
        />
      </section>
      <section>
        <h3 className="mb-2 font-semibold text-aivion-text">Связанные оповещения</h3>
        <p className="mb-2 text-xs">
          <Link href="/app/alerts" className="text-blue-700 underline">
            Открыть экран оповещений
          </Link>
        </p>
        {!detail.related_alerts || detail.related_alerts.length === 0 ? (
          <p className="text-aivion-muted">Нет связанных оповещений.</p>
        ) : (
          <ul className="space-y-3">
            {detail.related_alerts.map((a) => (
              <li key={a.id} className="rounded-lg border border-aivion-border bg-aivion-surface-soft p-3">
                <div className="flex flex-wrap gap-2 text-xs">
                  <Badge label={translateAlertSeverityUi(a.severity)} />
                  <Badge label={a.urgency.replaceAll("_", " ")} />
                  <span className="text-aivion-text">{translateAlertGroupUi(a.alert_group)}</span>
                  <span className="text-aivion-text">{alertTypeLabel(a.alert_type)}</span>
                </div>
                <p className="mt-1 font-medium">{a.title}</p>
                <p className="text-aivion-text">{a.message}</p>
                <p className="mt-1 text-xs text-aivion-muted">сущность: {fmtEntityRec(a)}</p>
                <p className="mt-1 text-xs text-aivion-muted">
                  Статус: {translateRecoStatus(a.status)} · {fmtDate(a.last_seen_at)}
                </p>
                <Link
                  href={`/app/alerts?focusAlertId=${encodeURIComponent(String(a.id))}`}
                  className="mt-2 inline-block text-xs font-medium text-aivion-blue hover:underline"
                >
                  Открыть в алертах
                </Link>
                <details className="mt-2">
                  <summary className="cursor-pointer text-xs font-medium text-aivion-blue">
                    Подтверждающие данные
                  </summary>
                  <EvidenceList
                    payload={a.evidence_payload}
                    emptyLabel="Нет дополнительных данных."
                    className="mt-2"
                  />
                </details>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function translateRunStatus(status: string): string {
  const s = status.toLowerCase();
  const m: Record<string, string> = {
    completed: "завершено",
    success: "успех",
    succeeded: "успешно",
    failed: "ошибка",
    error: "ошибка",
    running: "выполняется",
    pending: "в очереди",
  };
  return m[s] ?? status;
}

function RunStatusBadge({ status }: { status: string }) {
  const s = status.toLowerCase();
  const tone =
    s === "completed" || s === "success" || s === "succeeded"
      ? "border-emerald-300 bg-emerald-50 text-emerald-900"
      : s === "failed" || s === "error"
        ? "border-red-300 bg-red-50 text-red-900"
        : "border-amber-300 bg-amber-50 text-amber-900";
  return (
    <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-medium ${tone}`}>
      {translateRunStatus(status)}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const s = status.toLowerCase();
  const tone =
    s === "open"
      ? "border-emerald-300 bg-emerald-50 text-emerald-900"
      : s === "accepted"
        ? "border-blue-300 bg-blue-50 text-blue-900"
        : s === "dismissed"
          ? "border-aivion-border bg-aivion-surface-soft text-aivion-text"
          : s === "resolved"
            ? "border-violet-300 bg-violet-50 text-violet-900"
            : "border-aivion-border bg-aivion-surface text-aivion-text";
  const labels: Record<string, string> = {
    open: "Открыта",
    accepted: "Принята",
    dismissed: "Отклонена",
    resolved: "Закрыта",
  };
  return (
    <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-medium ${tone}`}>
      {labels[s] ?? status}
    </span>
  );
}

function PriorityBadge({ level }: { level: string }) {
  const tone =
    level === "critical"
      ? "border-red-400 bg-red-50 text-red-900"
      : level === "high"
        ? "border-orange-400 bg-orange-50 text-orange-900"
        : level === "medium"
          ? "border-amber-300 bg-amber-50 text-amber-900"
          : "border-slate-300 bg-slate-50 text-slate-800";
  const ru =
    level === "critical"
      ? "Критический"
      : level === "high"
        ? "Высокий"
        : level === "medium"
          ? "Средний"
          : level === "low"
            ? "Низкий"
            : level;
  return (
    <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-medium ${tone}`}>{ru}</span>
  );
}

function HorizonBadge({ horizon }: { horizon: string }) {
  const ru =
    horizon === "short_term"
      ? "Краткосрочный"
      : horizon === "medium_term"
        ? "Среднесрочный"
        : horizon === "long_term"
          ? "Долгосрочный"
          : horizon.replaceAll("_", " ");
  return (
    <span className="inline-flex rounded-full border border-cyan-200 bg-cyan-50 px-2 py-0.5 text-xs font-medium text-cyan-900">
      {ru}
    </span>
  );
}

function ConfidenceBadge({ level }: { level: string }) {
  const lv =
    level === "low" ? "низкая" : level === "medium" ? "средняя" : level === "high" ? "высокая" : level;
  return (
    <span className="inline-flex rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-900">
      уверенность: {lv}
    </span>
  );
}

function UrgencyBadge({ urgency }: { urgency: string }) {
  return (
    <span className="inline-flex rounded-full border border-aivion-border bg-aivion-surface px-2 py-0.5 text-xs font-medium text-aivion-text">
      срочность: {urgency.replaceAll("_", " ")}
    </span>
  );
}

function ConstraintHints({ payload }: { payload: Record<string, unknown> }) {
  const keys = payload && typeof payload === "object" ? Object.keys(payload) : [];
  if (keys.length === 0) return null;
  const has = (sub: string) => keys.some((k) => k.toLowerCase().includes(sub));
  const bits: string[] = [];
  if (has("stock")) bits.push("есть поля складского риска");
  if (has("ad") || has("campaign")) bits.push("есть поля рекламы");
  if (bits.length === 0) return null;
  return <p className="mb-1 text-xs text-aivion-muted">{bits.join(" · ")}</p>;
}

function SummaryCard({ label, value }: { label: string; value: number }) {
  return (
    <article className="rounded-lg border border-aivion-border bg-aivion-surface-soft/80 p-3">
      <p className="text-xs text-aivion-muted">{label}</p>
      <p className="text-xl font-semibold text-aivion-text">{value}</p>
    </article>
  );
}

function Select({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: [string, string][];
}) {
  return (
    <label className="text-sm">
      <span className="mb-1 block text-aivion-text">{label}</span>
      <select className="w-full rounded border px-2 py-1" value={value} onChange={(e) => onChange(e.target.value)}>
        {options.map(([v, text]) => (
          <option key={v || "all"} value={v}>
            {text}
          </option>
        ))}
      </select>
    </label>
  );
}

function Badge({ label }: { label: string }) {
  return <span className="inline-flex rounded border px-2 py-0.5 text-xs">{label}</span>;
}
