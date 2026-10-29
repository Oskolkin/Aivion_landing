import Link from "next/link";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { StatusBadge } from "@/components/ui/status-badge";
import { buttonClassNames } from "@/components/ui/button";
import { resolveServerApiBaseUrl } from "@/lib/env/api-base-url";
import { buildServerBackendHeaders } from "@/lib/server-request-headers";
import { nav } from "@/lib/ui-copy";
import { statusLabelRu } from "@/lib/status-labels";

type AccountResponse = {
  id: number;
  name: string;
  status: string;
};

export default async function AccountPage() {
  const headers = await buildServerBackendHeaders();

  const res = await fetch(`${resolveServerApiBaseUrl()}/api/v1/account`, {
    headers,
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error("Не удалось загрузить аккаунт");
  }

  const account = (await res.json()) as AccountResponse;

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-6">
      <PageHeader
        title="Аккаунт продавца"
        subtitle="Основные данные кабинета и статус подключения к Ozon."
      />

      <Card>
        <CardHeader>
          <CardTitle>{account.name || "Без названия"}</CardTitle>
          <CardDescription>Текущий продавец в Aivion</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <dl className="grid gap-4 sm:grid-cols-2">
            <div>
              <dt className="text-sm text-aivion-muted">Статус</dt>
              <dd className="mt-1">
                <StatusBadge status={account.status} label={statusLabelRu(account.status)} />
              </dd>
            </div>
            <div>
              <dt className="text-sm text-aivion-muted">Идентификатор</dt>
              <dd className="mt-1 font-mono text-xs text-aivion-muted-soft">{account.id}</dd>
            </div>
          </dl>
          <div className="flex flex-wrap gap-2 border-t border-aivion-border pt-4">
            <Link
              href="/app/integrations/ozon"
              className={buttonClassNames("secondary")}
            >
              {nav.ozonConnection}
            </Link>
          </div>
        </CardContent>
      </Card>
    </main>
  );
}
