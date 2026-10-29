/** Русские подписи для API-статусов (бейджи, фильтры) */

function normalizeStatus(raw: string): string {
  return raw.trim().toLowerCase().replace(/\s+/g, "_").replace(/-/g, "_");
}

const STATUS_LABELS_RU: Record<string, string> = {
  completed: "Завершено",
  success: "Успешно",
  succeeded: "Успешно",
  valid: "ОК",
  resolved: "Решено",
  active: "Активен",

  running: "Выполняется",
  sync_in_progress: "Синхронизация",
  pending: "Ожидание",
  sync_pending: "Ожидает синхронизации",
  checking: "Проверка",
  trial: "Пробный",
  open: "Открыт",
  medium: "Средний",

  failed: "Ошибка",
  sync_failed: "Сбой синхронизации",
  invalid: "Недействительно",
  error: "Ошибка",
  critical: "Критический",
  high: "Высокий",
  past_due: "Просрочено",

  missing: "Отсутствует",
  unknown: "Неизвестно",
  dismissed: "Отклонён",
  paused: "Приостановлен",
  cancelled: "Отменён",
  internal: "Внутренний",
  draft: "Черновик",
  not_configured: "Не настроено",
  not_connected: "Не подключено",
  low: "Низкий",

  not_started: "Не запускалось",
  not_set: "Не задано",
  set: "Задано",
  connected: "Подключено",
  disconnected: "Отключено",
  available: "Доступно",
  unavailable: "Недоступно",
  warning: "Внимание",
  done: "Готово",
  accepted: "Принято",
  rejected: "Отклонено",
  generating: "Генерация",
  manual: "Вручную",
  scheduled: "По расписанию",
};

export function statusLabelRu(status: string, fallback?: string): string {
  if (!status.trim()) {
    return fallback ?? "—";
  }
  const key = normalizeStatus(status);
  return STATUS_LABELS_RU[key] ?? fallback ?? status;
}
