"use client";

import { useEffect, useState } from "react";
import { useDashboardData } from "@/components/dashboard/shell/dashboard-data-provider";
import { getAdCampaigns } from "@/lib/publication-dashboard-api";
import { formatMetric } from "@/lib/metric-format";

export function DashboardAdvertisingPage() {
  const { data } = useDashboardData();
  const ad = data?.advertising;
  const publicationId = data?.publication?.id;
  const [rows, setRows] = useState<Record<string, unknown>[]>([]);

  useEffect(() => {
    if (!publicationId) return;
    void getAdCampaigns({ publication_id: publicationId, limit: 50 }).then((r) => setRows(r.items));
  }, [publicationId]);

  if (!ad) return null;
  const share = ad.account_ad_spend_share;
  const y = ad.yesterday_ad_spend_share;

  return (
    <div className="space-y-5" data-testid="dashboard-advertising-page">
      <header className="rounded-[14px] border border-slate-200 bg-white p-5">
        <h1 className="text-xl font-bold">Реклама</h1>
        <p className="mt-2 text-sm text-slate-600">
          ДРР кабинета: paid advertising spend / Canonical ordered amount. Показатели кампании — attribution space Ozon, не Canonical Sales.
        </p>
      </header>
      <div className="grid gap-3 sm:grid-cols-3">
        <section className="rounded-2xl border bg-white p-4">
          <p className="text-sm text-slate-600">ДРР кабинета (7 дней)</p>
          <p className="mt-1 text-xl font-bold" data-testid="account-ad-spend-share">
            {share?.status === "READY" ? formatMetric("RATIO", share.amount) : "—"}
          </p>
        </section>
        <section className="rounded-2xl border bg-white p-4">
          <p className="text-sm text-slate-600">ДРР кабинета вчера</p>
          <p className="mt-1 text-xl font-bold">{y?.status === "READY" ? formatMetric("RATIO", y.amount) : "—"}</p>
        </section>
        <section className="rounded-2xl border bg-white p-4">
          <p className="text-sm text-slate-600">Вывод</p>
          <p className="mt-1 text-sm">{(ad.findings ?? [])[0]?.title ?? "Заключение из публикации."}</p>
        </section>
      </div>
      <section className="rounded-[14px] border bg-white p-4">
        <h2 className="font-semibold">Кампании для проверки</h2>
        <table className="mt-3 min-w-full text-sm">
          <thead>
            <tr className="text-left text-slate-500">
              <th scope="col" className="py-2">Кампания</th>
              <th scope="col">Paid spend</th>
              <th scope="col">Attributed amount</th>
              <th scope="col">Campaign DRR</th>
              <th scope="col">Budget</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={String(row.campaign_id)} className="border-t">
                <td className="py-2">{String(row.title ?? row.campaign_id)}</td>
                <td>{row.paid_spend_state === "KNOWN" ? formatMetric("MONEY", String(row.paid_spend_amount)) : "—"}</td>
                <td>{row.attributed_order_amount_state === "KNOWN" ? formatMetric("MONEY", String(row.attributed_order_amount_amount)) : "—"}</td>
                <td>{row.campaign_drr_state === "KNOWN" ? formatMetric("RATIO", String(row.campaign_drr)) : "—"}</td>
                <td>{row.budget_state === "KNOWN" ? formatMetric("MONEY", String(row.budget_amount)) : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
