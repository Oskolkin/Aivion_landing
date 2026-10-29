import RecommendationsScreen from "@/components/recommendations-screen";

type RecommendationsPageProps = {
  searchParams?: Promise<{ focusRecommendationId?: string | string[] }>;
};

function parsePositiveIntParam(raw: string | string[] | undefined): number | undefined {
  if (raw === undefined) return undefined;
  const s = Array.isArray(raw) ? raw[0] : raw;
  if (s == null || typeof s !== "string") return undefined;
  const n = Number.parseInt(s.trim(), 10);
  if (!Number.isFinite(n) || n <= 0) return undefined;
  return n;
}

export default async function RecommendationsPage({ searchParams }: RecommendationsPageProps) {
  const sp = searchParams ? await searchParams : undefined;
  const initialFocusRecommendationId = parsePositiveIntParam(sp?.focusRecommendationId);

  return <RecommendationsScreen initialFocusRecommendationId={initialFocusRecommendationId} />;
}
