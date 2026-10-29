## Stage 5 Advertising Signals v1

Короткая фиксация explainable rules-based сигналов для MVP.

### 1) Рост расхода без заметного результата

- **Имя сигнала:** `growth_without_result`
- **Входные поля:** `ad_metrics_daily.spend`, `ad_metrics_daily.orders_count` по кампании на диапазоне дат
- **Правило:** расход вырос >= 20% (вторая половина периода vs первая), а рост заказов < 5% или заказов нет в обеих половинах
- **Explanation:** "Кампания ускорила расход, но заказов практически не прибавилось"

### 2) Слабая эффективность кампании

- **Имя сигнала:** `weak_efficiency`
- **Входные поля:** `spend_total`, `revenue_total`, `orders_total`, `ctr`, `cpc`
- **Правило:** при `spend_total > 0` выполнено хотя бы одно: `orders_total = 0`, `revenue_total < spend_total`, `ctr < 0.5%`, `cpc > 60`
- **Explanation:** "Кампания тратит бюджет с низкой конверсией/отдачей"

### 3) Alerts Engine (MVP): только campaign-level

В MVP рекламные alert’ы в Alerts Engine работают **только на уровне кампании** (`ad_weak_campaign_efficiency`), потому что Ozon Performance API даёт достоверные spend/orders/revenue на уровне кампании. SKU-level рекламные alert’ы (`ad_budget_on_low_stock_sku`, `ad_budget_on_weak_sku`, `ad_spend_without_result`) **не создаются** новыми rule runs.

- **Правило:** высокий ДРР (`spend/revenue`) при `revenue > 0`, либо расход без подтверждённой выручки/заказов при `spend >= noise_floor` (1000 ₽).
- **При `revenue <= 0`:** ДРР не рассчитывается (null в evidence), не писать «ДРР = 100%».

Риск дефицита по товару в рекламе покрывают **stock alert’ы** (`stock_low_coverage`, `stock_oos_risk`, `stock_critical_sku_low_stock`), а не отдельные SKU-level advertising alert’ы.
