import { buttonClassNames } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";

export function ActionsErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <ErrorState
      title="Не удалось загрузить действия"
      message={message || "Попробуйте обновить страницу. Если ошибка повторится, проверьте подключение Ozon."}
      action={
        <button type="button" className={buttonClassNames("secondary")} onClick={onRetry}>
          Повторить
        </button>
      }
    />
  );
}
