import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";
import type { CanonicalActionType } from "@/lib/action-presentation-registry";

export type ActionMember = {
  case_id: string;
  member_order: number;
  action_type: string;
  target_kind: string;
  target_id: string;
  user_state: string;
  system_state: string;
  active: boolean;
  parameters?: Record<string, unknown>;
};

export type ActionGroupView = {
  action_view_id: string;
  display_order: number;
  display_title: string;
  display_summary: string;
  action_type: CanonicalActionType | string;
  family: string;
  presentation_template_key: string;
  active_member_count: number;
  members: ActionMember[];
};

export type ActionCaseHistoryView = {
  action_view_id: string;
  case_id: string;
  display_title: string;
  display_summary: string;
  action_type: string;
  family: string;
  user_facing_mode: string;
  target_kind: string;
  target_id: string;
  parameters?: Record<string, unknown>;
};

export type ActionsListResponse = {
  publication_id?: string;
  state: string;
  groups?: ActionGroupView[];
  history?: ActionCaseHistoryView[];
  has_more?: boolean;
  next_cursor?: string;
};

async function parseJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(`Request failed with status ${res.status}`);
  }
  return (await res.json()) as T;
}

export async function getPublicationActions(params: {
  status?: string;
  family?: string;
  q?: string;
  limit?: number;
  cursor?: string;
}): Promise<ActionsListResponse> {
  const q = new URLSearchParams();
  if (params.status) q.set("status", params.status);
  if (params.family) q.set("family", params.family);
  if (params.q) q.set("q", params.q);
  if (params.limit) q.set("limit", String(params.limit));
  if (params.cursor) q.set("cursor", params.cursor);
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions?${q.toString()}`, {
    credentials: "include",
    cache: "no-store",
  });
  return parseJSON<ActionsListResponse>(res);
}

export async function getActionView(id: string) {
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/${encodeURIComponent(id)}`, {
    credentials: "include",
    cache: "no-store",
  });
  return parseJSON<{ kind: string; group?: ActionGroupView; history?: ActionCaseHistoryView }>(res);
}

export async function mutateCase(caseId: string, mode: "done" | "dismiss" | "reopen"): Promise<void> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/cases/${encodeURIComponent(caseId)}/${mode}`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) throw new Error("Не удалось обновить действие");
}

export async function bulkCases(caseIds: string[], eventType: string): Promise<void> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/actions/bulk`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ case_ids: caseIds, event_type: eventType }),
  });
  if (!res.ok) throw new Error("Не удалось выполнить групповое действие");
}
