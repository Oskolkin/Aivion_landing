import { replaceArtikulWithSKU } from "@/lib/stock-coverage-display";

export { replaceArtikulWithSKU };

const IMPROVEMENT_SUMMARY_FORBIDDEN = [
  "улучшений нет",
  "не выявлено",
  "не видно",
  "не компенсируются",
  "по сравнению с прошлой сводкой не",
  "заметных улучшений",
  "улучшений с прошлой сводки",
] as const;

export function isInappropriateImprovementSummary(text?: string | null): boolean {
  const normalized = (text ?? "").trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return IMPROVEMENT_SUMMARY_FORBIDDEN.some((phrase) => normalized.includes(phrase));
}

export function sanitizeSellerFacingText(text?: string | null): string {
  let s = replaceArtikulWithSKU((text ?? "").trim().replace(/\s+/g, " "));
  // Strip bare numeric campaign / product external IDs (9+ digits).
  s = s.replace(/\b\d{9,}\b/g, "кампания без названия");
  // Strip internal snake_case driver / factor codes from user-visible copy.
  s = s.replace(/\b[a-z]+(?:_[a-z0-9]+){1,}\b/g, (match) => {
    if (
      match.startsWith("http") ||
      match.includes("@") ||
      /^\d/.test(match) ||
      match.length < 6
    ) {
      return match;
    }
    // Keep common metric-ish tokens that are intentionally shown.
    if (
      match === "drr_pct" ||
      match === "sku_focus" ||
      match.endsWith("_pct") ||
      match.endsWith("_7d")
    ) {
      return match;
    }
    return "";
  });
  return s.replace(/\s{2,}/g, " ").replace(/\s([,.])/g, "$1").trim();
}
