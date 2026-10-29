\# Stage 4 — Dev Validation Scenario for Critical SKU and Stocks \& Replenishment



\## 1. Назначение документа



Этот документ фиксирует, как валидировать этап 4 в dev-окружении на synthetic dataset.



Этап 4 нельзя считать проверенным только потому, что:



\- backend API отвечает,

\- frontend страницы открываются,

\- таблицы выглядят аккуратно.



Для этапа 4 важно убедиться, что synthetic dataset действительно создаёт \*\*meaningful operational cases\*\*, на которых можно проверить:



\- ranking problem SKU;

\- depletion risk;

\- replenishment priority;

\- explainable rule-based scoring;

\- корректность двух отдельных operational экранов.



\---



\## 2. Контекст и ограничения



Проект использует уже реализованный analytics layer поверх Ozon-oriented seller model:



\- `daily\_account\_metrics`

\- `daily\_sku\_metrics`

\- `stocks\_view\_service`

\- `critical\_sku\_service`

\- `replenishment\_service`



При этом реальный Ozon seller account в dev-сценарии пустой, поэтому основная validation для этапа 4 строится на \*\*synthetic seed\*\*.



Это допустимо и ожидаемо, потому что Seller API является базовым seller-side контуром для работы с товарами, заказами и остатками, а operational analytics в проекте строится на уже импортированных или синтетически подготовленных product/order/sales/stock данных. :contentReference\[oaicite:1]{index=1}



Также важно помнить, что stock layer в Ozon и в текущем проекте трактуется как \*\*current-state operational snapshot\*\*, а не как strict realtime physical truth. В официальной документации Ozon указано, что обновление остатков на витрине может занимать около 20 минут. :contentReference\[oaicite:2]{index=2}



\---



\## 3. Что именно должен покрывать synthetic dataset



Synthetic dataset для этапа 4 считается достаточным, если в нём присутствуют следующие типы SKU.



\### 3.1. Stock-driven critical cases



\#### Case A — Out of stock + high importance

SKU:

\- `stock\_available = 0`

\- `share\_of\_revenue` / importance высокая

\- должен попадать в верхнюю часть:

&#x20; - Critical SKU

&#x20; - Stocks \& Replenishment



\#### Case B — Out of stock + low importance

SKU:

\- `stock\_available = 0`

\- importance низкая

\- должен попадать в risky список, но ниже, чем аналогичный high-importance case



\#### Case C — Low stock + high importance

SKU:

\- `stock\_available` низкий

\- `days\_of\_cover <= 3` или низкий

\- importance высокая

\- должен иметь высокий replenishment priority



\#### Case D — Low stock + low importance

SKU:

\- stock низкий

\- importance низкая

\- должен быть ниже в списке, чем важные low-stock SKU



\---



\### 3.2. Demand / performance driven cases



\#### Case E — Revenue drop + stock normal

SKU:

\- stock нормальный

\- `revenue\_delta\_day` отрицательная и существенная

\- должен подниматься в Critical SKU как проблемный по performance, а не только по stock



\#### Case F — Sales ops drop + stock normal

SKU:

\- current stock достаточный

\- `orders\_delta\_day` / sales ops delta отрицательная

\- должен получать signal/badge по падению спроса



\#### Case G — Revenue drop + low cover

SKU:

\- одновременно performance problem и depletion risk

\- это один из самых ценных validation cases для `problem\_score`



\---



\### 3.3. Demand-null / ambiguous cases



\#### Case H — Stock = 0, но спрос почти нулевой

SKU:

\- `stock\_available = 0`

\- `days\_of\_cover = NULL` или near-null demand context

\- должен быть помечен как stock problem, но его importance / replenishment priority не должны искусственно раздуваться



\#### Case I — Stock > 0, но спрос почти нулевой

SKU:

\- stock есть

\- спрос почти отсутствует

\- не должен автоматически считаться urgent replenishment candidate



\---



\## 4. Минимальный обязательный validation set



Этап 4 считается покрытым, если synthetic data позволяет проверить минимум такие сценарии:



1\. SKU с `stock\_available = 0`

2\. SKU с low stock

3\. SKU с высоким revenue share

4\. SKU с отрицательной revenue динамикой

5\. SKU с отрицательной `sales ops` динамикой

6\. SKU: low stock + high importance

7\. SKU: out of stock + high importance

8\. SKU: low stock + low importance

9\. SKU: sales падают, но stock нормальный

10\. SKU: stock нулевой, но спрос почти нулевой



Если хотя бы часть этих кейсов не воспроизводится, synthetic seed нужно минимально дообогатить.



\---



\## 5. Что считается “корректным” результатом validation



Validation этапа 4 считается успешной, если на synthetic dataset подтверждается следующее.



\### Для Critical SKU



\- SKU с сильными негативными сигналами реально поднимаются вверх списка

\- `problem\_score` выглядит объяснимым

\- signals/badges соответствуют данным

\- high-importance problem SKU стоят выше low-importance problem SKU при сопоставимом risk profile



\### Для Stocks \& Replenishment



\- out-of-stock SKU получают strongest depletion risk

\- low-stock / low-cover SKU попадают в replenishment candidates

\- replenishment priority не одинаковый у всех SKU

\- importance влияет на приоритет

\- низкий stock без importance не поднимается выше действительно важных SKU



\---



\## 6. Какие источники использовать при проверке



Во время validation нужно сверять результаты с реальными backend source layers.



\### Source layers



\- `daily\_account\_metrics`

\- `daily\_sku\_metrics`

\- current stock snapshot через `stocks\_view\_service`

\- raw current stock rows из `stocks` при необходимости



\### Service / API layers



\- `critical\_sku\_service`

\- `replenishment\_service`

\- `GET /api/v1/analytics/critical-skus`

\- `GET /api/v1/analytics/stocks-replenishment`



\### Frontend layers



\- `/app/critical-skus`

\- `/app/stocks-replenishment`



\---



\## 7. Пошаговый dev-сценарий проверки



\### Шаг 1. Прогнать synthetic seed



Нужно убедиться, что текущий seller account заполнен synthetic данными.



Пример:

\- `dev-seed-stage3` для выбранного `seller\_account\_id`



\### Шаг 2. Пересчитать account metrics



Нужно обновить `daily\_account\_metrics`.



Пример:

\- `dev-rebuild-account-metrics`



\### Шаг 3. Пересчитать SKU metrics



Нужно обновить `daily\_sku\_metrics`.



Пример:

\- `dev-rebuild-sku-metrics`



\### Шаг 4. Проверить Critical SKU service/API



Проверить:

\- `dev-check-critical-skus`

\- `/api/v1/analytics/critical-skus`



Что нужно подтвердить:

\- список не пустой

\- top SKU выглядят осмысленно

\- score/signal logic совпадает с synthetic cases



\### Шаг 5. Проверить Stocks \& Replenishment service/API



Проверить:

\- `dev-check-replenishment`

\- `/api/v1/analytics/stocks-replenishment`



Что нужно подтвердить:

\- replenishment list не пустой

\- риск и priority распределяются осмысленно

\- current stock snapshot корректно участвует в ranking



\### Шаг 6. Открыть frontend страницу Critical SKU



Проверить:

\- `/app/critical-skus`



Что нужно подтвердить:

\- top 10–20 SKU реально problem-oriented

\- badges/signals читаемы

\- сортировка по умолчанию соответствует ranking logic



\### Шаг 7. Открыть frontend страницу Stocks \& Replenishment



Проверить:

\- `/app/stocks-replenishment`



Что нужно подтвердить:

\- таблица отражает current stock + depletion risk + replenishment priority

\- top строки реально выглядят как кандидаты на первоочередное внимание



\---



\## 8. Что делать, если validation не проходит



Если screens/API формально работают, но meaningful cases не проявляются, нужно:



1\. не переписывать scoring rules сразу;

2\. сначала проверить, хватает ли synthetic cases;

3\. затем минимально дообогатить seed.



Если synthetic dataset не создаёт:

\- high-importance out-of-stock,

\- low-stock important SKU,

\- negative revenue trend,

\- negative sales ops trend,



то проблема не в frontend и не обязательно в scoring — сначала нужно обогатить входной synthetic сценарий.



\---



\## 9. Практический критерий завершения этапа 4



Этап 4 можно считать верифицированным в dev-среде, если:



\- synthetic seed стабильно создаёт обязательные stage-4 cases;

\- `critical\_sku\_service` возвращает осмысленно ранжированный список;

\- `replenishment\_service` возвращает осмысленный priority list;

\- оба backend endpoint’а отдают корректные DTO;

\- обе frontend страницы открываются и показывают meaningful operational content;

\- результаты можно объяснить через зафиксированные scoring rules.



\---



\## 10. Итог



Цель этого validation сценария — убедиться, что этап 4 даёт не просто ещё две таблицы, а действительно operational layer:



\- что критично;

\- что скоро закончится;

\- что важно для аккаунта;

\- что нужно смотреть первым.



Если это подтверждается на synthetic dataset, этап 4 считается валидированным в dev-окружении.

