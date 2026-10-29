import AlertsScreen from "@/components/alerts-screen";

type AlertsPageProps = {
  searchParams?: Promise<{ focusAlertId?: string | string[] }>;
};

function parsePositiveIntParam(raw: string | string[] | undefined): number | undefined {
  if (raw === undefined) return undefined;
  const s = Array.isArray(raw) ? raw[0] : raw;
  if (s == null || typeof s !== "string") return undefined;
  const n = Number.parseInt(s.trim(), 10);
  if (!Number.isFinite(n) || n <= 0) return undefined;
  return n;
}

export default async function AlertsPage({ searchParams }: AlertsPageProps) {
  const sp = searchParams ? await searchParams : undefined;
  const initialFocusAlertId = parsePositiveIntParam(sp?.focusAlertId);

  return <AlertsScreen initialFocusAlertId={initialFocusAlertId} />;
}
