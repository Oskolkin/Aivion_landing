"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/ui/page-header";
import { presentationFor, formatActionParameters } from "@/lib/action-presentation-registry";
import { buildActionChatHref } from "@/lib/chat-action-handoff";
import {
  bulkCases,
  getPublicationActions,
  mutateCase,
  type ActionGroupView,
  type ActionCaseHistoryView,
} from "@/lib/publication-actions-api";

const FAMILIES = [
  { id: "", label: "Все" },
  { id: "INVENTORY", label: "Остатки" },
  { id: "PRICING", label: "Цены" },
  { id: "ADVERTISING", label: "Реклама" },
  { id: "PROMOTION", label: "Акции" },
  { id: "ASSORTMENT", label: "Ассортимент" },
  { id: "ACCOUNT_HEALTH", label: "Аккаунт" },
];

const STATUSES = [
  { id: "active", label: "Активные" },
  { id: "done", label: "Выполненные" },
  { id: "irrelevant", label: "Неактуальные" },
];

export default function ActionsScreen() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const status = searchParams.get("status") || "active";
  const family = searchParams.get("family") || "";
  const q = searchParams.get("q") || "";
  const selected = searchParams.get("action") || "";

  const [groups, setGroups] = useState<ActionGroupView[]>([]);
  const [history, setHistory] = useState<ActionCaseHistoryView[]>([]);
  const [loading, setLoading] = useState(true);
  const [mutating, setMutating] = useState(false);
  const [error, setError] = useState("");
  const [mutationError, setMutationError] = useState("");
  const [mobileDetail, setMobileDetail] = useState(Boolean(selected));

  const [nextCursor, setNextCursor] = useState("");
  const [hasMore, setHasMore] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const resp = await getPublicationActions({ status, family: family || undefined, q: q || undefined, limit: 50 });
      setGroups(resp.groups ?? []);
      setHistory(resp.history ?? []);
      setNextCursor(resp.next_cursor ?? "");
      setHasMore(Boolean(resp.has_more));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось загрузить действия");
    } finally {
      setLoading(false);
    }
  }, [status, family, q]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const setQuery = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams.toString());
    for (const [k, v] of Object.entries(patch)) {
      if (!v) next.delete(k);
      else next.set(k, v);
    }
    const qs = next.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname);
  };

  const selectedGroup = useMemo(
    () => groups.find((g) => g.action_view_id === selected) ?? null,
    [groups, selected],
  );
  const selectedHistory = useMemo(
    () => history.find((h) => h.action_view_id === selected) ?? null,
    [history, selected],
  );

  async function onMemberDone(caseId: string) {
    setMutating(true);
    setMutationError("");
    try {
      await mutateCase(caseId, "done");
      await reload();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : "Не удалось обновить действие");
    } finally {
      setMutating(false);
    }
  }
  async function onMemberDismiss(caseId: string) {
    setMutating(true);
    setMutationError("");
    try {
      await mutateCase(caseId, "dismiss");
      await reload();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : "Не удалось обновить действие");
    } finally {
      setMutating(false);
    }
  }
  async function onReopen(caseId: string) {
    setMutating(true);
    setMutationError("");
    try {
      await mutateCase(caseId, "reopen");
      await reload();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : "Не удалось обновить действие");
    } finally {
      setMutating(false);
    }
  }
  async function onBulkDone(g: ActionGroupView) {
    setMutating(true);
    setMutationError("");
    try {
      await bulkCases(g.members.filter((m) => m.active).map((m) => m.case_id), "DONE");
      await reload();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : "Не удалось выполнить групповое действие");
    } finally {
      setMutating(false);
    }
  }

  const list = (
    <div className="min-w-0" data-testid="actions-list">
      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Статус действий">
        {STATUSES.map((s) => (
          <button key={s.id} type="button" role="tab" aria-selected={status === s.id} className={`rounded-full border px-3 py-1 text-sm ${status === s.id ? "border-blue-600" : "border-slate-200"}`} onClick={() => setQuery({ status: s.id, action: null })}>
            {s.label}
          </button>
        ))}
      </div>
      <div className="mt-3 flex flex-wrap gap-2" aria-label="Семейства">
        {FAMILIES.map((f) => (
          <button key={f.id || "all"} type="button" className={`rounded-full border px-3 py-1 text-sm ${family === f.id ? "border-blue-600" : "border-slate-200"}`} onClick={() => setQuery({ family: f.id || null })}>
            {f.label}
          </button>
        ))}
      </div>
      <label className="mt-3 block text-sm">
        Поиск
        <input aria-label="Поиск действий" className="mt-1 w-full rounded-md border px-3 py-2 text-sm" value={q} onChange={(e) => setQuery({ q: e.target.value || null })} />
      </label>
      {loading ? <p className="mt-4 text-sm text-slate-500" role="status" aria-busy="true">Загрузка…</p> : null}
      {error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}
      {mutationError ? <p className="mt-4 text-sm text-red-700" role="alert" data-testid="actions-mutation-error">{mutationError}</p> : null}
      {!loading && groups.length === 0 && history.length === 0 ? (
        <p className="mt-6 text-sm text-slate-600" data-testid="actions-empty">Действий сейчас нет</p>
      ) : null}
      <ul className="mt-4 space-y-2">
        {groups.map((g) => (
          <li key={g.action_view_id}>
            <button type="button" className="w-full rounded-xl border border-slate-200 bg-white p-3 text-left" onClick={() => { setQuery({ action: g.action_view_id }); setMobileDetail(true); }}>
              <p className="font-medium">{g.display_title}</p>
              <p className="text-sm text-slate-500">{g.active_member_count} активных · {presentationFor(String(g.action_type))?.label ?? g.action_type}</p>
            </button>
          </li>
        ))}
        {history.map((h) => (
          <li key={h.action_view_id}>
            <button type="button" className="w-full rounded-xl border border-slate-200 bg-white p-3 text-left" onClick={() => { setQuery({ action: h.action_view_id }); setMobileDetail(true); }}>
              <p className="font-medium">{h.display_title}</p>
              <p className="text-sm text-slate-500">{h.target_id}</p>
            </button>
          </li>
        ))}
      </ul>
      {hasMore ? (
        <button
          type="button"
          className="mt-3 text-sm text-blue-700"
          aria-label="Показать ещё действия"
          onClick={async () => {
            const resp = await getPublicationActions({ status, family: family || undefined, q: q || undefined, limit: 50, cursor: nextCursor });
            setGroups((prev) => [...prev, ...(resp.groups ?? [])]);
            setHistory((prev) => [...prev, ...(resp.history ?? [])]);
            setNextCursor(resp.next_cursor ?? "");
            setHasMore(Boolean(resp.has_more));
          }}
        >
          Показать ещё
        </button>
      ) : null}
    </div>
  );

  const detail = (
    <div className="min-w-0 rounded-xl border border-slate-200 bg-white p-4" data-testid="actions-detail">
      <button type="button" className="mb-3 text-sm text-blue-700 xl:hidden" aria-label="Назад к действиям" onClick={() => { setMobileDetail(false); setQuery({ action: null }); }}>
        Назад к действиям
      </button>
      {selectedGroup ? (
        <>
          <h2 className="text-lg font-semibold">{selectedGroup.display_title}</h2>
          <p className="mt-1 text-sm text-slate-600">{selectedGroup.display_summary}</p>
          <p className="mt-2 text-xs text-slate-500">{presentationFor(String(selectedGroup.action_type))?.label}</p>
          <dl className="mt-3 space-y-1 text-sm">
            {formatActionParameters(String(selectedGroup.action_type), selectedGroup.members[0]?.parameters).map((row) => (
              <div key={row.label} className="flex justify-between gap-2">
                <dt className="text-slate-500">{row.label}</dt>
                <dd>{row.value}</dd>
              </div>
            ))}
          </dl>
          <ul className="mt-4 space-y-2">
            {selectedGroup.members.map((m) => (
              <li key={m.case_id} className="flex items-center justify-between gap-2 border-t py-2">
                <span className="text-sm">{m.target_kind} {m.target_id}</span>
                {m.active ? (
                  <span className="flex gap-2">
                    <button type="button" disabled={mutating} aria-label="Сделано" className="text-sm text-blue-700" onClick={() => onMemberDone(m.case_id)}>Сделано</button>
                    <button type="button" disabled={mutating} aria-label="Неактуально" className="text-sm text-slate-600" onClick={() => onMemberDismiss(m.case_id)}>Неактуально</button>
                  </span>
                ) : (
                  <span className="text-xs text-slate-400">{m.user_state}</span>
                )}
              </li>
            ))}
          </ul>
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <button type="button" disabled={mutating} className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white" onClick={() => onBulkDone(selectedGroup)}>
              Отметить группу выполненной
            </button>
            <a
              className="text-sm text-blue-700"
              href={buildActionChatHref({ actionViewId: selectedGroup.action_view_id })}
              data-testid="action-ask-ai"
              aria-label="Спросить AI"
            >
              Спросить AI
            </a>
          </div>
        </>
      ) : selectedHistory ? (
        <>
          <h2 className="text-lg font-semibold">{selectedHistory.display_title}</h2>
          <p className="mt-1 text-sm">{selectedHistory.display_summary}</p>
          {selectedHistory.user_facing_mode !== "DONE" ? (
            <button type="button" className="mt-4 text-sm text-blue-700" onClick={() => onReopen(selectedHistory.case_id)}>Вернуть в работу</button>
          ) : null}
          <p className="mt-4 text-sm">
            <a className="text-blue-700" href={buildActionChatHref({ actionViewId: selectedHistory.action_view_id })} aria-label="Спросить AI">
              Спросить AI
            </a>
          </p>
        </>
      ) : (
        <p className="text-sm text-slate-500">Выберите действие</p>
      )}
    </div>
  );

  return (
    <div className="p-5 sm:p-6" data-testid="actions-workspace" aria-busy={loading || mutating}>
      <PageHeader title="Действия" subtitle="Группы текущей публикации и состояние кейсов" />
      <div className="mt-4 hidden min-w-0 gap-4 xl:grid xl:grid-cols-2">
        {list}
        {detail}
      </div>
      <div className="mt-4 xl:hidden">{mobileDetail ? detail : list}</div>
    </div>
  );
}
