import { evidenceEntries } from "@/lib/evidence-format";

export type EvidenceListProps = {
  payload: unknown;
  emptyLabel?: string;
  className?: string;
};

/** Renders backend evidence/metrics as readable key-value rows (no raw JSON). */
export function EvidenceList({
  payload,
  emptyLabel = "Нет дополнительных данных.",
  className,
}: EvidenceListProps) {
  const entries = evidenceEntries(payload);
  if (entries.length === 0) {
    return <p className="text-sm text-aivion-muted">{emptyLabel}</p>;
  }
  return (
    <dl className={`grid gap-2 sm:grid-cols-2 ${className ?? ""}`}>
      {entries.map(({ label, value }) => (
        <div key={label} className="rounded-aivion-md border border-aivion-border bg-aivion-surface-soft px-3 py-2">
          <dt className="text-xs text-aivion-muted">{label}</dt>
          <dd className="mt-0.5 text-sm font-medium text-aivion-text">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
