import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";
import { normalizeActionItem, normalizeActionsResponse } from "@/lib/actions-normalize";
import type { ActionItem, ActionsResponse, ListActionsParams } from "@/lib/actions-types";

function queryFromParams(params: ListActionsParams): string {
  const query = new URLSearchParams();
  if (params.status) query.set("status", params.status);
  if (params.priority) query.set("priority", params.priority);
  if (params.category) query.set("category", params.category);
  if (params.metric_date) query.set("metric_date", params.metric_date);
  if (params.limit != null) query.set("limit", String(params.limit));
  if (params.offset != null) query.set("offset", String(params.offset));
  const value = query.toString();
  return value ? `?${value}` : "";
}

async function parseJSONOrThrow<T>(res: Response): Promise<T> {
  const contentType = res.headers.get("content-type") ?? "";
  const isJson = contentType.includes("application/json");
  if (!res.ok) {
    if (isJson) {
      const body = (await res.json()) as { error?: string };
      throw new Error(body.error || `Request failed with status ${res.status}`);
    }
    throw new Error(`Request failed with status ${res.status}`);
  }
  if (!isJson) throw new Error("Expected JSON response");
  return (await res.json()) as T;
}

export async function getActions(params: ListActionsParams): Promise<ActionsResponse> {
  const query = queryFromParams(params);
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions${query}`, {
    method: "GET",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  const raw = await parseJSONOrThrow<unknown>(res);
  return normalizeActionsResponse(raw);
}

export async function getActionById(actionId: number): Promise<ActionItem> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/id/${actionId}`, {
    method: "GET",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  const raw = await parseJSONOrThrow<unknown>(res);
  return normalizeActionItem(raw);
}

export async function getActionByKey(actionKey: string): Promise<ActionItem> {
  const encoded = encodeURIComponent(actionKey);
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/${encoded}`, {
    method: "GET",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  const raw = await parseJSONOrThrow<unknown>(res);
  return normalizeActionItem(raw);
}

async function mutateActionState(actionKey: string, mode: "check" | "uncheck" | "dismiss"): Promise<void> {
  const encoded = encodeURIComponent(actionKey);
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/${encoded}/${mode}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) {
    const contentType = res.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      const body = (await res.json()) as { error?: string };
      throw new Error(body.error || "Не удалось обновить статус действия");
    }
    throw new Error(`Request failed with status ${res.status}`);
  }
}

export async function checkAction(actionKey: string): Promise<void> {
  await mutateActionState(actionKey, "check");
}

export async function uncheckAction(actionKey: string): Promise<void> {
  await mutateActionState(actionKey, "uncheck");
}

export async function dismissAction(actionKey: string): Promise<void> {
  await mutateActionState(actionKey, "dismiss");
}
