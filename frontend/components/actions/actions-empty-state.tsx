import { buttonClassNames } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

type ActionsEmptyStateProps = {
  mode: "global" | "filtered";
  onShowOpen?: () => void;
  notes?: string[];
};

export function ActionsEmptyState({ mode, onShowOpen, notes }: ActionsEmptyStateProps) {
  if (mode === "filtered") {
    return (
      <EmptyState
        title="По выбранным параметрам действий нет"
        message="Измените фильтры, поиск или сортировку, чтобы увидеть другие рекомендации."
        action={
          onShowOpen ? (
            <button type="button" className={buttonClassNames("secondary")} onClick={onShowOpen}>
              Сбросить фильтры
            </button>
          ) : undefined
        }
      />
    );
  }

  return (
    <div className="space-y-3">
      <EmptyState
        title="Активных рекомендаций сейчас нет"
        message="После обновления данных Aivion покажет задачи по остаткам, SKU, рекламе и воронке."
      />
      {notes && notes.length > 0 ? (
        <div className="rounded-aivion-lg border border-[#b2ddff] bg-[#eff8ff] px-4 py-3 text-sm text-[#175cd3]">
          <p className="font-semibold">Что известно:</p>
          <ul className="mt-1 space-y-1">
            {notes.map((note) => (
              <li key={note}>- {note}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
