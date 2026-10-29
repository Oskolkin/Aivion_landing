"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { AlertBanner, ErrorState } from "@/components/ui/error-state";
import { EvidenceList } from "@/components/ui/evidence-list";
import { Input } from "@/components/ui/input";
import { LoadingState } from "@/components/ui/loading-state";
import { MetricCard } from "@/components/ui/metric-card";
import { PageHeader } from "@/components/ui/page-header";
import { StatusBadge } from "@/components/ui/status-badge";
import {
  alertGroupLabel,
  SELLER_ALERT_GROUP_OPTIONS,
} from "@/lib/alert-labels";
import {
  dismissAlert,
  getAlerts,
  getAlertsSummary,
  resolveAlert,
  runAlerts,
  type AlertEntityType,
  type AlertGroup,
  type AlertItem,
  type AlertSeverity,
  type AlertsSummaryResponse,
  type AlertStatus,
} from "@/lib/alerts-api";

type FilterState = {
  status: "" | AlertStatus;
  group: "" | AlertGroup;
  severity: "" | AlertSeverity;
  entityType: "" | AlertEntityType;
  limit: number;
  offset: number;
};

function fmtDate(value: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function fmtEntity(row: AlertItem): string {
  if (row.entity_sku != null) return `SKU ${row.entity_sku}`;
  if (row.entity_offer_id) return `Предложение ${row.entity_offer_id}`;
  if (row.entity_id) return row.entity_id;
  return translateEntityType(row.entity_type);
}

function translateEntityType(t: string): string {
  const m: Record<string, string> = {
    account: "Аккаунт",
    sku: "SKU",
    product: "Товар",
    campaign: "Кампания",
    pricing_constraint: "Устаревший тип",
  };
  return m[t] ?? t;
}

function translateAlertStatus(s: string): string {
  const m: Record<string, string> = {
    open: "Открыто",
    resolved: "Закрыто",
    dismissed: "Отклонено",
  };
  return m[s] ?? s;
}

function translateSeverity(s: string): string {
  const m: Record<string, string> = {
    low: "Низкая",
    medium: "Средняя",
    high: "Высокая",
    critical: "Критическая",
  };
  return m[s] ?? s;
}

function translateAlertGroup(g: string): string {
  return alertGroupLabel(g);
}

function translateUrgency(u: string): string {
  return u.replaceAll("_", " ");
}

export default function AlertsScreen({ initialFocusAlertId }: { initialFocusAlertId?: number }) {
  const [summary, setSummary] = useState<AlertsSummaryResponse | null>(null);
  const [items, setItems] = useState<AlertItem[]>([]);
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const [focusHighlightId, setFocusHighlightId] = useState<number | null>(null);
  const [focusAlertMissing, setFocusAlertMissing] = useState(false);
  const [filters, setFilters] = useState<FilterState>({
    status: "",
    group: "",
    severity: "",
    entityType: "",
    limit: 50,
    offset: 0,
  });

  const [loadingSummary, setLoadingSummary] = useState(true);
  const [loadingList, setLoadingList] = useState(true);
  const [running, setRunning] = useState(false);
  const [actionLoadingId, setActionLoadingId] = useState<number | null>(null);
  const [runAsOfDate, setRunAsOfDate] = useState("");
  const [error, setError] = useState("");
  const [statusMessage, setStatusMessage] = useState("");

  async function reloadSummary() {
    try {
      setLoadingSummary(true);
      const data = await getAlertsSummary();
      setSummary(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить сводку оповещений");
    } finally {
      setLoadingSummary(false);
    }
  }

  async function reloadList() {
    try {
      setLoadingList(true);
      const data = await getAlerts({
        status: filters.status || undefined,
        group: filters.group || undefined,
        severity: filters.severity || undefined,
        entityType: filters.entityType || undefined,
        limit: filters.limit,
        offset: filters.offset,
      });
      setItems(data.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить оповещения");
    } finally {
      setLoadingList(false);
    }
  }

  useEffect(() => {
    setError("");
    void reloadSummary();
    void reloadList();
    // list depends on all filter values
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.status, filters.group, filters.severity, filters.entityType, filters.limit, filters.offset]);

  useEffect(() => {
    if (initialFocusAlertId == null) {
      setFocusHighlightId(null);
      setFocusAlertMissing(false);
      return;
    }
    if (loadingList) {
      return;
    }
    const hit = items.some((a) => a.id === initialFocusAlertId);
    if (hit) {
      setExpanded((prev) => ({ ...prev, [initialFocusAlertId]: true }));
      setFocusHighlightId(initialFocusAlertId);
      setFocusAlertMissing(false);
    } else {
      setFocusHighlightId(null);
      setFocusAlertMissing(true);
    }
  }, [initialFocusAlertId, items, loadingList]);

  useEffect(() => {
    if (focusHighlightId == null || loadingList || typeof document === "undefined") {
      return;
    }
    const el = document.getElementById(`alert-row-${focusHighlightId}`);
    if (el) {
      requestAnimationFrame(() => {
        el.scrollIntoView({ block: "nearest", behavior: "smooth" });
      });
    }
  }, [focusHighlightId, loadingList, items]);

  function resetFiltersForFocus() {
    setFilters({
      status: "",
      group: "",
      severity: "",
      entityType: "",
      limit: 50,
      offset: 0,
    });
  }

  async function handleRun() {
    try {
      setRunning(true);
      setError("");
      setStatusMessage("");
      await runAlerts(runAsOfDate ? { as_of_date: runAsOfDate } : undefined);
      setStatusMessage("Запуск движка оповещений завершён.");
      await Promise.all([reloadSummary(), reloadList()]);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось запустить оповещения");
    } finally {
      setRunning(false);
    }
  }

  async function handleDismiss(id: number) {
    try {
      setActionLoadingId(id);
      setError("");
      setStatusMessage("");
      await dismissAlert(id);
      setStatusMessage(`Оповещение ${id} отклонено.`);
      await Promise.all([reloadSummary(), reloadList()]);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось отклонить оповещение");
    } finally {
      setActionLoadingId(null);
    }
  }

  async function handleResolve(id: number) {
    try {
      setActionLoadingId(id);
      setError("");
      setStatusMessage("");
      await resolveAlert(id);
      setStatusMessage(`Оповещение ${id} закрыто.`);
      await Promise.all([reloadSummary(), reloadList()]);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось закрыть оповещение");
    } finally {
      setActionLoadingId(null);
    }
  }

  return (
    <main className="space-y-6 p-6">
      <PageHeader
        title="Алерты"
        subtitle="Что случилось, насколько это срочно и что делать дальше."
      >
        <div className="flex flex-wrap items-end gap-2">
          <label className="text-sm">
            <span className="mb-1 block text-aivion-text">Дата расчёта (необязательно)</span>
            <Input
              type="date"
              className="py-1.5"
              value={runAsOfDate}
              onChange={(e) => setRunAsOfDate(e.target.value)}
            />
          </label>
          <Button type="button" variant="secondary" onClick={handleRun} disabled={running}>
            {running ? "Запуск…" : "Обновить алерты"}
          </Button>
        </div>
      </PageHeader>

      {error ? <ErrorState title="Не удалось загрузить алерты" message={error} /> : null}
      {statusMessage ? <AlertBanner tone="success" message={statusMessage} /> : null}

      {initialFocusAlertId != null && focusAlertMissing && !loadingList ? (
        <AlertBanner
          tone="warning"
          title="Выбранный алерт не в текущем списке"
          message={`Алерт №${initialFocusAlertId} может быть скрыт фильтрами, на другой странице или уже закрыт.`}
          action={
            <Button type="button" variant="secondary" className="mt-2" onClick={resetFiltersForFocus}>
              Сбросить фильтры
            </Button>
          }
        />
      ) : null}

      <section className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
        {loadingSummary || !summary ? (
          <div className="col-span-full">
            <LoadingState variant="simple" message="Загрузка сводки…" />
          </div>
        ) : (
          <>
            <MetricCard title="Открытые" value={summary.open_total} />
            <MetricCard title="Критические" value={summary.critical_count} />
            <MetricCard title="Высокие" value={summary.high_count} />
            <MetricCard title="Средние" value={summary.medium_count} />
            <MetricCard title="Низкие" value={summary.low_count} />
            <MetricCard title="Продажи" value={summary.by_group.sales} />
            <MetricCard title="Склад" value={summary.by_group.stock} />
            <MetricCard title="Реклама" value={summary.by_group.advertising} />
          </>
        )}
      </section>

      <section className="rounded-aivion-lg border border-aivion-border bg-aivion-surface p-4 shadow-aivion-sm">
        <h2 className="text-base font-semibold text-aivion-text">Последнее обновление</h2>
        {loadingSummary || !summary ? (
          <LoadingState variant="simple" message="Загрузка…" className="p-2" />
        ) : !summary.latest_run ? (
          <p className="mt-2 text-sm text-aivion-muted">Обновлений пока не было.</p>
        ) : (
          <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-aivion-muted">Статус</dt>
              <dd className="font-medium text-aivion-text">
                <StatusBadge status={summary.latest_run.status} />
              </dd>
            </div>
            <div>
              <dt className="text-aivion-muted">Найдено алертов</dt>
              <dd className="font-medium text-aivion-text">{summary.latest_run.total_alerts_count}</dd>
            </div>
            <div>
              <dt className="text-aivion-muted">Начало</dt>
              <dd className="text-aivion-text">{fmtDate(summary.latest_run.started_at)}</dd>
            </div>
            <div>
              <dt className="text-aivion-muted">Завершение</dt>
              <dd className="text-aivion-text">{fmtDate(summary.latest_run.finished_at)}</dd>
            </div>
          </dl>
        )}
      </section>

      <section className="rounded border p-4">
        <h2 className="mb-3 text-lg font-semibold">Фильтры</h2>
        <div className="grid grid-cols-1 gap-2 md:grid-cols-3 xl:grid-cols-6">
          <Select
            label="Статус"
            value={filters.status}
            onChange={(v) => setFilters((s) => ({ ...s, status: v as FilterState["status"], offset: 0 }))}
            options={[
              ["", "Все"],
              ["open", "Открыто"],
              ["resolved", "Закрыто"],
              ["dismissed", "Отклонено"],
            ]}
          />
          <Select
            label="Группа"
            value={filters.group}
            onChange={(v) => setFilters((s) => ({ ...s, group: v as FilterState["group"], offset: 0 }))}
            options={SELLER_ALERT_GROUP_OPTIONS}
          />
          <Select
            label="Важность"
            value={filters.severity}
            onChange={(v) => setFilters((s) => ({ ...s, severity: v as FilterState["severity"], offset: 0 }))}
            options={[
              ["", "Все"],
              ["low", "Низкая"],
              ["medium", "Средняя"],
              ["high", "Высокая"],
              ["critical", "Критическая"],
            ]}
          />
          <Select
            label="Тип сущности"
            value={filters.entityType}
            onChange={(v) => setFilters((s) => ({ ...s, entityType: v as FilterState["entityType"], offset: 0 }))}
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
                  limit: Math.max(1, Math.min(200, Number(e.target.value) || 50)),
                  offset: 0,
                }))
              }
            />
          </label>
        </div>
      </section>

      <section className="rounded-aivion-lg border border-aivion-border bg-aivion-surface p-4 shadow-aivion-sm">
        <h2 className="mb-3 text-base font-semibold text-aivion-text">Список алертов</h2>
        {loadingList ? (
          <LoadingState variant="simple" message="Загрузка алертов…" />
        ) : items.length === 0 ? (
          <EmptyState
            title="Алертов нет"
            message="По текущим фильтрам алертов не найдено. Попробуйте сбросить фильтры или обновить данные."
          />
        ) : (
          <div className="space-y-3">
            {items.map((row) => (
              <article
                key={row.id}
                id={`alert-row-${row.id}`}
                className={`rounded-aivion-lg border border-aivion-border bg-aivion-surface-soft p-4 ${
                  focusHighlightId === row.id ? "ring-2 ring-aivion-blue/40" : ""
                }`}
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex flex-wrap gap-2">
                      <StatusBadge status={row.severity} label={translateSeverity(row.severity)} />
                      <Badge label={translateUrgency(row.urgency)} />
                      <Badge label={translateAlertGroup(row.alert_group)} />
                      <StatusBadge status={row.status} label={translateAlertStatus(row.status)} />
                    </div>
                    <h3 className="font-semibold text-aivion-text">{row.title}</h3>
                    <p className="text-sm text-aivion-muted">{row.message}</p>
                    <p className="text-xs text-aivion-muted-soft">
                      {fmtEntity(row)} · {fmtDate(row.last_seen_at)}
                    </p>
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      className="px-2 py-1 text-xs"
                      disabled={actionLoadingId === row.id}
                      onClick={() => handleDismiss(row.id)}
                    >
                      Отклонить
                    </Button>
                    <Button
                      type="button"
                      variant="secondary"
                      className="px-2 py-1 text-xs"
                      disabled={actionLoadingId === row.id}
                      onClick={() => handleResolve(row.id)}
                    >
                      Закрыть
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      className="px-2 py-1 text-xs"
                      onClick={() =>
                        setExpanded((prev) => ({
                          ...prev,
                          [row.id]: !prev[row.id],
                        }))
                      }
                    >
                      {expanded[row.id] ? "Скрыть детали" : "Подробнее"}
                    </Button>
                  </div>
                </div>
                {expanded[row.id] ? (
                  <div className="mt-4 border-t border-aivion-border pt-4">
                    <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-aivion-muted">
                      Подтверждающие данные
                    </h4>
                    <EvidenceList payload={row.evidence_payload} emptyLabel="Дополнительных данных нет." />
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        )}
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            disabled={filters.offset === 0 || loadingList}
            onClick={() =>
              setFilters((s) => ({
                ...s,
                offset: Math.max(0, s.offset - s.limit),
              }))
            }
          >
            Назад
          </Button>
          <Button
            type="button"
            variant="secondary"
            disabled={loadingList || items.length < filters.limit}
            onClick={() =>
              setFilters((s) => ({
                ...s,
                offset: s.offset + s.limit,
              }))
            }
          >
            Вперёд
          </Button>
          <span className="text-sm text-aivion-muted">
            Показано {filters.offset + 1}–{filters.offset + items.length}
          </span>
        </div>
      </section>
    </main>
  );
}

function Badge({ label }: { label: string }) {
  return (
    <span className="inline-flex rounded-full border border-aivion-border bg-aivion-surface px-2 py-0.5 text-xs text-aivion-text">
      {label}
    </span>
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
