"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  getAdminActionGenerationRunDetail,
  getAdminActionGenerationRuns,
  getAdminPipelineDiagnostics,
  type AdminActionsGenerationCategoryRun,
  type AdminActionsGenerationRun,
  type AdminPipelineDiagnostics,
} from "@/lib/admin-api";
import { buildAdminRunFiltersQuery, parseAdminRunFilters } from "@/lib/admin-actions-generation-runs";

export default function AdminActionsGenerationRunsScreen() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const filters = useMemo(() => parseAdminRunFilters(Object.fromEntries(searchParams.entries())), [searchParams]);
  const runId = useMemo(() => {
    const raw = searchParams.get("run_id");
    const n = raw ? Number(raw) : NaN;
    return Number.isFinite(n) && n > 0 ? n : undefined;
  }, [searchParams]);

  const [list, setList] = useState<{ items: AdminActionsGenerationRun[]; limit: number; offset: number }>({
    items: [],
    limit: 50,
    offset: 0,
  });
  const [listError, setListError] = useState("");
  const [loading, setLoading] = useState(true);
  const [detail, setDetail] = useState<{
    run: AdminActionsGenerationRun;
    category_runs: AdminActionsGenerationCategoryRun[];
  } | null>(null);
  const [pipelineDiagnostics, setPipelineDiagnostics] = useState<AdminPipelineDiagnostics | null>(null);

  const currentFiltersQuery = buildAdminRunFiltersQuery(filters);

  const loadList = useCallback(async () => {
    setLoading(true);
    setListError("");
    try {
      const result = await getAdminActionGenerationRuns({
        seller_id: filters.seller_id,
        status: filters.status,
        final_status: filters.final_status,
        metric_date: filters.metric_date,
        limit: 50,
        offset: 0,
      });
      setList(result);
    } catch {
      setListError("Не удалось загрузить generation runs.");
      setList({ items: [], limit: 50, offset: 0 });
    } finally {
      setLoading(false);
    }
  }, [filters.final_status, filters.metric_date, filters.seller_id, filters.status]);

  useEffect(() => {
    void loadList();
  }, [loadList]);

  useEffect(() => {
    if (!runId) {
      setDetail(null);
      setPipelineDiagnostics(null);
      return;
    }
    const exists = list.items.some((row) => row.run_id === runId);
    if (!exists) {
      setDetail(null);
      return;
    }
    void getAdminActionGenerationRunDetail(runId)
      .then((value) => {
        setDetail(value);
        const pipelineRunId = value.run.ai_pipeline_run_id;
        if (pipelineRunId && pipelineRunId > 0) {
          return getAdminPipelineDiagnostics(pipelineRunId)
            .then((diag) => setPipelineDiagnostics(diag))
            .catch(() => setPipelineDiagnostics(null));
        }
        setPipelineDiagnostics(null);
      })
      .catch(() => {
        setDetail(null);
        setPipelineDiagnostics(null);
      });
  }, [list.items, runId]);

  const applyFilters = (formData: FormData) => {
    const next = new URLSearchParams();
    const seller = String(formData.get("seller_id") ?? "").trim();
    const status = String(formData.get("status") ?? "").trim();
    const finalStatus = String(formData.get("final_status") ?? "").trim();
    const metricDate = String(formData.get("metric_date") ?? "").trim();
    if (seller) next.set("seller_id", seller);
    if (status) next.set("status", status);
    if (finalStatus) next.set("final_status", finalStatus);
    if (metricDate) next.set("metric_date", metricDate);
    const encoded = next.toString();
    router.push(encoded ? `${pathname}?${encoded}` : pathname);
  };

  return (
    <div className="space-y-4 p-6">
      <h1 className="text-2xl font-semibold">Actions generation runs</h1>
      <p className="text-sm text-muted-foreground">Debug view for Actions Engine generation runs.</p>
      <form
        className="grid grid-cols-1 md:grid-cols-5 gap-3 items-end"
        onSubmit={(event) => {
          event.preventDefault();
          applyFilters(new FormData(event.currentTarget));
        }}
      >
        <label className="text-sm">
          Seller ID
          <input
            className="mt-1 w-full rounded border px-2 py-1"
            name="seller_id"
            defaultValue={filters.seller_id ?? ""}
          />
        </label>
        <label className="text-sm">
          Status
          <select className="mt-1 w-full rounded border px-2 py-1" name="status" defaultValue={filters.status ?? ""}>
            <option value="">all</option>
            <option value="running">running</option>
            <option value="completed">completed</option>
            <option value="failed">failed</option>
          </select>
        </label>
        <label className="text-sm">
          Final status
          <select
            className="mt-1 w-full rounded border px-2 py-1"
            name="final_status"
            defaultValue={filters.final_status ?? ""}
          >
            <option value="">all</option>
            <option value="completed">completed</option>
            <option value="completed_repaired">completed_repaired</option>
            <option value="completed_partial">completed_partial</option>
            <option value="empty">empty</option>
            <option value="fallback">fallback</option>
            <option value="failed">failed</option>
          </select>
        </label>
        <label className="text-sm">
          Metric date
          <input
            type="date"
            className="mt-1 w-full rounded border px-2 py-1"
            name="metric_date"
            defaultValue={filters.metric_date ?? ""}
          />
        </label>
        <div className="flex gap-2">
          <button className="rounded border px-3 py-2 text-sm" type="submit">
            Применить
          </button>
          <a className="rounded border px-3 py-2 text-sm" href="/app/admin/actions-generation-runs">
            Сбросить
          </a>
        </div>
      </form>
      {loading ? <div className="rounded border p-3 text-sm text-muted-foreground">Загрузка…</div> : null}
      {listError ? <div className="rounded border border-red-300 bg-red-50 p-3 text-sm">{listError}</div> : null}
      {!loading && list.items.length === 0 ? (
        <div className="rounded border p-3 text-sm text-muted-foreground">Generation runs не найдены.</div>
      ) : null}
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr className="border-b">
            <th className="text-left py-2">Run</th>
            <th className="text-left py-2">Seller</th>
            <th className="text-left py-2">Date</th>
            <th className="text-left py-2">Status</th>
            <th className="text-left py-2">Final</th>
            <th className="text-left py-2">Counts</th>
          </tr>
        </thead>
        <tbody>
          {list.items.map((item) => (
            <RunRow key={item.run_id} item={item} currentFiltersQuery={currentFiltersQuery} />
          ))}
        </tbody>
      </table>
      {detail ? (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold">Run detail #{detail.run.run_id}</h2>
          {pipelineDiagnostics ? (
            <div className="rounded border p-4 space-y-2 text-sm">
              <h3 className="font-medium">Pipeline trace</h3>
              <div>Pipeline run: {pipelineDiagnostics.pipeline_run_id}</div>
              <div>Data pack hash: {pipelineDiagnostics.data_pack_hash || "—"}</div>
              <div>Global context hash: {pipelineDiagnostics.global_context_hash || "—"}</div>
              <div>Dashboard run: {pipelineDiagnostics.dashboard_run_id || "—"}</div>
              <div>Actions run: {pipelineDiagnostics.actions_run_id || "—"}</div>
              <div>Top-3 source: {pipelineDiagnostics.dashboard_top_actions?.source || "—"}</div>
              <div>Top-3 action ids: {(pipelineDiagnostics.dashboard_top_actions?.action_ids ?? []).join(", ") || "—"}</div>
              <div>
                AI proxy:{" "}
                {pipelineDiagnostics.ai_proxy?.ai_proxy_configured
                  ? `${pipelineDiagnostics.ai_proxy.ai_proxy_host} (${pipelineDiagnostics.ai_proxy.effective_proxy_source})`
                  : "not configured"}
              </div>
              <div>Artifacts: {(pipelineDiagnostics.artifacts ?? []).join(", ") || "—"}</div>
            </div>
          ) : null}
          <pre className="text-xs bg-muted p-3 rounded overflow-auto">{JSON.stringify(detail.run, null, 2)}</pre>
          <h3 className="text-md font-medium">Category runs</h3>
          {detail.category_runs.map((row, idx) => (
            <CategoryRunBlock key={`${row.domain}-${idx}`} row={row} />
          ))}
        </div>
      ) : runId ? (
        <div className="rounded border p-3 text-sm text-muted-foreground">Выбранный run не найден в текущей выборке.</div>
      ) : null}
    </div>
  );
}

function RunRow({ item, currentFiltersQuery }: { item: AdminActionsGenerationRun; currentFiltersQuery: string }) {
  return (
    <tr className="border-b">
      <td className="py-2">
        <a
          className="underline"
          href={`/app/admin/actions-generation-runs${currentFiltersQuery ? `${currentFiltersQuery}&run_id=${item.run_id}` : `?run_id=${item.run_id}`}`}
        >
          {item.run_id}
        </a>
      </td>
      <td className="py-2">{item.seller_account_id}</td>
      <td className="py-2">{item.metric_date || "-"}</td>
      <td className="py-2">{item.status}</td>
      <td className="py-2">{item.final_status || "-"}</td>
      <td className="py-2">
        {item.accepted_count}/{item.repaired_count}/{item.dropped_count}/{item.fallback_count}
      </td>
    </tr>
  );
}

function CategoryRunBlock({ row }: { row: AdminActionsGenerationCategoryRun }) {
  return (
    <div className="border rounded p-3 space-y-2">
      <div className="font-medium">
        {row.domain} - {row.status}
      </div>
      <div className="text-xs text-muted-foreground">
        accepted/repaired/dropped: {row.accepted_count}/{row.repaired_count}/{row.dropped_count}
      </div>
      <pre className="text-xs bg-muted p-2 rounded overflow-auto">{JSON.stringify(row, null, 2)}</pre>
    </div>
  );
}
