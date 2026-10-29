/** User-facing product terms. Prefer these constants over hardcoding «ИИ» / «AI». */

export const aiTerms = {
  brandShort: "AI",
  chat: "AI-чат",
  chatCabinet: "AI-чат по кабинету",
  openChat: "Открыть AI-чат",
  diagnostics: "AI-диагностика продаж",
  conclusion: "AI-вывод",
  cabinetResume: "AI-резюме по кабинету",
  summary: "AI-сводка",
  categorySummaryHint:
    "AI-сводка по категориям, ключевые риски и действия для улучшения состояния.",
  interpretationFallback: "Показана базовая сводка по метрикам без AI-интерпретации.",
  improvementEmpty: "Подсказка появится после формирования AI-сводки.",
  scoreEmpty: "Индекс пока не рассчитан. Он появится после загрузки метрик и генерации AI-сводки.",
  recommendationEngine:
    "AI-движок рекомендаций: предпосылки, запуски генерации, проверенный результат и действия со статусом.",
  recommendationLoading: "Запуск AI-генерации рекомендаций…",
  actionSourceSummary: "AI-сводка",
} as const;
