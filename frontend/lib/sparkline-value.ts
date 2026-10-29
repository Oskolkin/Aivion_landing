/** Treat dashboard sparkline point value: missing/null stays missing; real zero stays 0. */
export function sparklineNumericValue(value: number | null | undefined): number | null {
  if (value == null || Number.isNaN(value)) {
    return null;
  }
  return value;
}
