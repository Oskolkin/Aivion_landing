/** Minimum inline width of the detail panel to use narrative + compact objects side by side. */
export const ACTION_DETAIL_TWO_COLUMN_MIN_WIDTH = 1120;

/** Minimum width for a 3-column expanded object table (no horizontal scroll). */
export const ACTION_DETAIL_EXPANDED_TABLE_MIN_WIDTH = 900;

/** Default number of objects shown in compact mode before expand. */
export const COMPACT_OBJECT_LIMIT = 4;

export function shouldUseTwoColumnDetailLayout(containerWidth: number): boolean {
  return containerWidth >= ACTION_DETAIL_TWO_COLUMN_MIN_WIDTH;
}

export function shouldUseExpandedTableLayout(containerWidth: number): boolean {
  return containerWidth >= ACTION_DETAIL_EXPANDED_TABLE_MIN_WIDTH;
}

export function compactObjectSlice<T>(items: T[], limit = COMPACT_OBJECT_LIMIT): T[] {
  return items.slice(0, limit);
}

export function hiddenObjectCount(total: number, limit = COMPACT_OBJECT_LIMIT): number {
  return Math.max(0, total - limit);
}
