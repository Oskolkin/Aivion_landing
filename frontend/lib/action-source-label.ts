const SOURCE_LABELS: Record<string, string> = {
  recommendation: "Рекомендация",
  ai_summary: "AI-сводка",
  alert: "Алерт",
  deterministic: "Система",
};

export function formatActionSourceLabel(source?: string | null): string {
  const key = (source ?? "").trim().toLowerCase();
  if (!key) {
    return "";
  }
  return SOURCE_LABELS[key] ?? "Действие";
}
