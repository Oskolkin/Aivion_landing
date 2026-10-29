\# Stage 4 — Scoring Rules for Critical SKU and Stocks \& Replenishment



\## 1. Назначение документа



Этот документ фиксирует \*\*MVP-правила scoring\*\* для этапа 4.



Он нужен, чтобы:



\- сделать `problem\_score` объяснимым и воспроизводимым;

\- не превратить этап 4 в скрытую “умную модель” без прозрачных правил;

\- согласовать backend logic для:

&#x20; - \*\*Critical SKU\*\*

&#x20; - \*\*Stocks \& Replenishment\*\*

\- зафиксировать пороги, веса и fallback-правила.



На этапе 4 scoring должен быть:



\- \*\*простым\*\*

\- \*\*rule-based\*\*

\- \*\*deterministic\*\*

\- \*\*explainable\*\*

\- \*\*easy to debug\*\*



\---



\## 2. Источники данных



Scoring строится только на уже существующих внутренних слоях проекта.



\### Используемые источники



\#### `daily\_sku\_metrics`

Используется для:

\- `revenue`

\- `orders\_count` / `sales ops`

\- `revenue\_delta\_day\_to\_day`

\- `orders\_delta\_day\_to\_day`

\- `stock\_available`

\- `days\_of\_cover`

\- `share\_of\_revenue`

\- `contribution\_to\_revenue\_change`



\#### `daily\_account\_metrics`

Используется для:

\- account-level baseline

\- interpretation of SKU importance

\- overall account context



\#### `stocks\_view\_service`

Используется как current-state stock source:

\- `quantity\_total`

\- `quantity\_reserved`

\- `quantity\_available`

\- `snapshot\_at`

\- warehouse detail при необходимости



\### Operational semantics reminder



Остатки на этапе 4 трактуются как \*\*current operational snapshot\*\*, а не как history stream. Это согласовано с логикой Ozon, где обновление остатков на витрине не является мгновенным и может занимать около 20 минут. :contentReference\[oaicite:0]{index=0}



\---



\## 3. Общая логика scoring



Для каждого SKU считаются отдельные сигналы, а затем они объединяются в:



\- `out\_of\_stock\_risk`

\- `replenishment\_priority`

\- `problem\_score`



Scoring на этапе 4 строится по принципу:



1\. вычислить простые сигналы;

2\. перевести их в нормализованные risk levels / points;

3\. сложить по весам;

4\. получить итоговый score;

5\. отсортировать SKU по score.



\---



\## 4. Сигналы по SKU



\## 4.1. Sales Change Signal



Показывает ухудшение или улучшение продаж по SKU.



\### Базовая метрика

\- `revenue\_delta\_day\_to\_day`



\### Правила интерпретации



\#### Сильное падение

Если `revenue\_delta\_day\_to\_day < 0` и падение существенно относительно текущего уровня SKU, SKU получает негативный сигнал.



\#### Нейтрально

Если изменение около нуля — сигнал нейтральный.



\#### Рост

Если изменение положительное — негативного сигнала нет.



\### MVP-правило scoring

\- сильное падение: `+3`

\- умеренное падение: `+2`

\- слабое падение: `+1`

\- ноль или рост: `0`



\---



\## 4.2. Orders / Sales Ops Change Signal



Показывает изменение операционной активности SKU.



\### Базовая метрика

\- `orders\_delta\_day\_to\_day`



\### Важно

На текущем этапе это \*\*не обязательно “точные заказы” в бизнес-смысле\*\*, а текущий SKU-day operational count, согласованный с уже существующей моделью проекта.



\### MVP-правило scoring

\- сильное падение: `+2`

\- умеренное падение: `+1`

\- ноль или рост: `0`



\---



\## 4.3. Stock Signal



Показывает факт проблемного текущего остатка.



\### Базовые метрики

\- `stock\_available`

\- current stock summary из `stocks\_view\_service`



\### MVP-правила

\- `stock\_available = 0` → strongest critical signal

\- `stock\_available <= low\_stock\_threshold` → medium signal

\- иначе signal neutral



\### Базовые пороги

\- `0` → critical

\- `1–3` → low stock

\- `>3` → normal stock



\### MVP-points

\- out of stock: `+5`

\- low stock: `+3`

\- normal stock: `0`



\---



\## 4.4. Days of Cover Signal



Показывает риск скорого исчерпания остатка через отношение остатка к recent demand.



\### Базовая метрика

\- `days\_of\_cover`



\### Интерпретация

`days\_of\_cover` уже вычисляется в проекте как:



```text id="u3t9q4"

stock\_available / avg\_daily\_orders\_recent\_7\_days

