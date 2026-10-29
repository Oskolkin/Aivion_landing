# Раздел «Товары» (MVP)

## Назначение

User-facing витрина SKU: фото, название, SKU/артикул, текущая цена, справочная мин. цена Ozon, метрики за последний день с данными и мини-график заказов за 7 дней.

Это **не** управление ценами и **не** repricing. Изменение цен выполняется в кабинете Ozon.

## Маршруты

| Маршрут | Поведение |
|---------|-----------|
| `/app/products` | Основной экран «Товары» |
| `/app/pricing-constraints` | Редирект на `/app/products` |
| `/app/pricing` | Редирект на `/app/products` |
| `/app/prices` | Редирект на `/app/products` |

## API

`GET /api/v1/products` (требует `connection_ready=true`)

Query: `search`, `limit` (default 50, max 200), `offset`, `sort`, `has_sales_last_day`, `stock_status`.

Источники данных:

- `products` — каталог, цены, `primary_image_url`
- `daily_sku_metrics` — последний день с данными и sparkline 7d

### Sparkline 7d и пропуски данных

Rollup `daily_sku_metrics` (`ListDailySKUSourcesBySellerAndDateRange`) создаёт строки **по дням с продажами** (из `sales`), плюс заглушки для **всех SKU только на конечную дату** диапазона пересчёта. **Не** гарантируется строка на каждый SKU на каждый календарный день.

Поэтому в sparkline:
- день **без строки** в `daily_sku_metrics` → `orders_count` / `revenue` **не возвращаются** (не путать с нулём продаж);
- день **со строкой** и `orders_count = 0` → явный ноль в JSON.
- `stock_status_bucket` — вычисляется на бэкенде из остатка и days_of_cover

## Изображения

Источник: `POST /v3/product/info/list` (поля `primary_image`, `images`).

`primary_image_url` = первый URL из `primary_image`, иначе `images[0]`, иначе `NULL`.

### Demo fallback (frontend)

Для seller-facing карточек без реального Ozon-фото:

- файлы: `frontend/public/demo/products/product-demo-1.jpg` … `product-demo-5.jpg` (URL `/demo/products/...`);
- helper: `frontend/lib/product-images.ts` (`resolveProductImageUrl`, `getDemoProductImage`);
- компонент: `frontend/components/product-image.tsx`;
- приоритет: непустой `primary_image_url` / `image_url` из API → иначе стабильное demo-фото по seed (SKU → offer_id → ozon_product_id);
- mapping детерминированный (hash), без `Math.random` на рендере;
- production-записи в БД **не** перезаписываются demo URL.

## Ценообразование (backend)

Таблицы и API `pricing-constraints` **сохранены** для внутреннего использования и будущих этапов; user-facing UI удалён из MVP.

Price/economics alerts в раздел «Товары» **не включены**.

## Навигация

В боковом меню пункт **«Товары»** заменяет **«Ценовые ограничения»**.
