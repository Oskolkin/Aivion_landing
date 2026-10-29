\# Stage 7. Alerts Engine — validation scenarios



\## Назначение документа



Этот документ фиксирует сценарии ручной и технической проверки stage 7: \*\*Alerts Engine\*\*.



Цель проверки — убедиться, что система:



\- запускает Alerts Engine для seller account;

\- создаёт alert’ы по продажам, остаткам и рекламе (price/economics **отключены** для MVP);

\- для SKU sales-drop активен **`sku_orders_drop`** (не `sku_revenue_drop`);

\- сохраняет evidence payload;

\- не создаёт дубли при повторном запуске;

\- корректно обрабатывает `dismiss` и `resolve`;

\- отображает alert’ы через API, Alerts screen и dashboard teaser.



Документ предназначен для dev / QA-проверки MVP-реализации stage 7.



\---



\## Предусловия



Перед проверкой stage 7 должны быть выполнены следующие условия:



\- backend запущен;

\- frontend запущен;

\- применены все миграции;

\- пользователь авторизован;

\- у пользователя есть seller account;

\- в системе есть данные по товарам, заказам, остаткам, рекламе и pricing constraints;

\- stage 3–6 уже выполнены;

\- endpoint `POST /api/v1/alerts/run` доступен;

\- экран `/app/alerts` доступен;

\- dashboard содержит alerts teaser.



\---



\## Основные API для проверки



\### Запуск Alerts Engine



```http

POST /api/v1/alerts/run

```

## Stock coverage alerts (FBO)

- `stock_low_coverage`: `7 < days_of_cover <= 14`, `current_stock > 0`, severity/urgency medium, reason `supply_planning_window`.
- `stock_oos_risk`: `0 < days_of_cover <= 7`, `current_stock > 0`; `3 < days <= 7` → high; `0 < days <= 3` → critical/immediate.
- `current_stock <= 0` **не создаёт** stock alert сам по себе (товар мог быть выведен из ассортимента).
- `stock_critical_sku_low_stock`: только key SKU (7d relative context) + `0 < days_of_cover <= 14`, без алерта на нулевой остаток.

## Advertising alerts (MVP)

- Активно генерируется только `ad_weak_campaign_efficiency` на уровне **кампании**.
- Покрывает: высокий ДРР (`spend/revenue`) и расход без подтверждённой выручки/заказов (`spend >= 1000`).
- При `revenue <= 0` в evidence `drr`/`drr_percent` = null (не «ДРР = 100%»).
- SKU-level типы `ad_spend_without_result`, `ad_budget_on_weak_sku`, `ad_budget_on_low_stock_sku` новыми runs не создаются.

## FBO surplus storage risk

- `fbo_surplus_storage_risk` не рассчитывает точную стоимость платного хранения. Сигнал о риске избыточного FBO-хранения на основе Ozon `/v1/analytics/stocks`: `turnover_grade`, `idc`, `excess_stock_count`, `available_stock_count`.
- Создаётся при `available_stock_count > 0` и хотя бы одном из: `turnover_grade = SURPLUS`, `excess_stock_count > 0`, `idc >= 120`.
- При `available_stock_count = 0` alert не создаётся.
- Данные сохраняются в `stock_analytics_snapshots` при stocks sync (после warehouse stocks).

## Price / economics (disabled for MVP)

- `RunPriceEconomicsAlerts` / `RunForAccount` **не генерируют** новые price/margin/pricing_constraint alerts.
- Legacy types в БД могут читаться admin/support; seller UI показывает их как **«Устаревший тип»** без активных фильтров.

## SKU orders drop

- Тип: `sku_orders_drop`
- Сравнение: заказы за день vs предыдущий день (`orders_count`, `comparison_orders`, `orders_delta_percent` в evidence).
- Порог: −40% при минимум 5 заказов в базовом дне.
- Падение только выручки без падения заказов **не** создаёт `sku_orders_drop`.

