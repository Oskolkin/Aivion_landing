/** Coalesce null/undefined/non-array API values to a safe empty array. */
export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
