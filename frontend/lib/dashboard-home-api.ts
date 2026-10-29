import { apiGet } from "@/lib/api";
import { normalizeDashboardHome } from "@/lib/normalize-dashboard-home";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

export async function getDashboardHome(): Promise<DashboardHomeResponse> {
  const raw = await apiGet<DashboardHomeResponse>("/api/v1/dashboard/home");
  return normalizeDashboardHome(raw);
}
