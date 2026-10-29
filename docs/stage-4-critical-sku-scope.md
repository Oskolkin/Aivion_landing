\# Stage 4 — Critical SKU \& Stocks \& Replenishment Scope



\## 1. Назначение документа



Этот документ фиксирует scope этапа 4 и operational semantics для двух продуктовых экранов:



\- \*\*Critical SKU\*\*

\- \*\*Stocks \& Replenishment\*\*



Цель этапа — не расширить общую аналитику как таковую, а собрать \*\*первый сильный operational surface\*\*, который показывает пользователю, какие SKU требуют внимания прямо сейчас и почему.



Документ нужен, чтобы:

\- не раздувать этап до полноценной BI/supply-chain системы;

\- зафиксировать источники данных, сигналы и ограничения;

\- отделить operational ranking layer от ingestion layer и от общей dashboard-аналитики.



\---



\## 2. Контекст Ozon и проекта



Проект строится поверх Ozon Seller API и уже реализованного ingestion-контура. Ozon позиционирует Seller API как способ автоматизировать работу с большим количеством товаров и заказов, а продуктовые и заказные данные являются естественной основой для первой операционной аналитики. :contentReference\[oaicite:0]{index=0}



Для блока остатков важно, что в Ozon обновление остатков на витрине занимает около 20 минут. Поэтому остатки в нашем проекте на этапе 4 трактуются как \*\*операционный snapshot\*\*, а не как strict realtime truth. :contentReference\[oaicite:1]{index=1}



Также Ozon отдельно выделяет сценарии, где заказ отменяется из-за отсутствия товара, что подтверждает практическую важность контроля out-of-stock риска как отдельного seller-side operational use case. :contentReference\[oaicite:2]{index=2}



\---



\## 3. Цель этапа 4



На выходе этапа 4 пользователь должен получить два отдельных operational экрана:



1\. \*\*Critical SKU\*\*  

&#x20;  Показывает SKU, которые являются проблемными по совокупности сигналов и требуют внимания в первую очередь.



2\. \*\*Stocks \& Replenishment\*\*  

&#x20;  Показывает SKU с риском дефицита и SKU, которые требуют приоритетного пополнения.



Главный результат этапа:

\- список проблемных SKU;

\- список SKU с риском out-of-stock;

\- список SKU, требующих первоочередного внимания.



\---



\## 4. Почему это operational layer, а не ещё одна аналитика



Этап 4 не про “ещё один dashboard”.  

Он про \*\*action-oriented ranking\*\*:



\- что падает;

\- что заканчивается;

\- что важно для аккаунта;

\- что надо смотреть первым.



Здесь не строится полноценная финансовая модель, не вводится прогнозирование и не делается оптимизационная supply-chain система.  

Это rule-based слой внимания поверх уже существующих аналитических агрегатов.



\---



\## 5. Какие экраны реализуются



\### 5.1. Critical SKU



Отдельная страница со списком SKU, отсортированных по `problem\_score`.



Для каждой карточки SKU должны быть видны:

\- идентификатор товара;

\- название;

\- текущая выручка / sales ops;

\- изменение продаж;

\- изменение orders / sales ops;

\- текущий остаток;

\- days of cover;

\- риск исчерпания;

\- важность для аккаунта;

\- общий problem score;

\- короткие signal badges.



\### 5.2. Stocks \& Replenishment



Отдельная страница, сфокусированная на остатках и пополнении.



Для каждой SKU должны быть видны:

\- товар;

\- текущий доступный остаток;

\- reserved / total stock;

\- days of cover;

\- риск дефицита;

\- приоритет пополнения;

\- при необходимости — warehouse detail.



\---



\## 6. Источники данных этапа 4



Этап 4 строится только на уже существующих внутренних слоях проекта.



\### Основные источники



\#### `daily\_sku\_metrics`

Используется как основной источник SKU-level operational signals:

\- revenue;

\- orders / sales ops count;

\- revenue delta;

\- orders delta;

\- stock\_available;

\- days\_of\_cover;

\- share\_of\_revenue;

\- contribution\_to\_revenue\_change.



\#### `daily\_account\_metrics`

Используется как account-level baseline:

\- account revenue;

\- account context для importance;

\- support signals для interpretation.



\#### `stocks\_view\_service`

Используется как источник current-state stock view:

\- current total stock;

\- current reserved stock;

\- current available stock;

\- warehouse-level current rows;

\- snapshot timestamp.



\---



\## 7. Operational semantics stock layer



На этапе 4 остатки трактуются как \*\*current-state stock snapshot\*\*.



Это означает:



\- stock layer отражает текущее состояние данных в БД;

\- stock layer не является историей движения остатков;

\- stock layer не является strict realtime truth;

\- stock risk строится на current stock + recent demand, а не на полном потоке stock events.



Это согласовано и с текущей архитектурой проекта, и с operational semantics Ozon, где обновление остатков на витрине не является мгновенным. :contentReference\[oaicite:3]{index=3}



\---



\## 8. Какие сигналы должны появиться по каждому SKU



Для каждого SKU в этапе 4 используются следующие сигналы:



\### 8.1. Изменение продаж

Изменение выручки по SKU относительно предыдущего дня или короткого окна.



\### 8.2. Изменение orders / sales ops

Изменение операционной активности SKU.  

На текущем MVP-этапе этот показатель трактуется через уже существующую semantics проекта и не должен выдавать ложную точность.



\### 8.3. Текущий остаток

Current available stock и связанный current stock context.



\### 8.4. Риск исчерпания

Rule-based оценка вероятности близкого дефицита на базе:

\- `stock\_available`

\- `days\_of\_cover`

\- recent demand



\### 8.5. Важность для аккаунта

Насколько SKU важен для результата аккаунта:

\- доля в выручке;

\- вклад в изменение результата;

\- место среди ключевых SKU.



\### 8.6. Общий `problem\_score`

Итоговый score, который объединяет:

\- негативную динамику,

\- stock risk,

\- importance,

\- replenishment urgency.



\---



\## 9. Принцип scoring на этапе 4



Scoring на этапе 4 должен быть:



\- \*\*простым\*\*

\- \*\*rule-based\*\*

\- \*\*deterministic\*\*

\- \*\*explainable\*\*

\- \*\*debuggable на synthetic data\*\*



Это означает:



\- используются пороги и веса;

\- score легко объяснить в коде и в UI;

\- score не зависит от ML/forecasting;

\- score можно воспроизвести вручную на конкретном SKU.



\### Что это не должно быть

\- не probabilistic model;

\- не ML-ranking;

\- не hidden formula;

\- не black box.



\---



\## 10. Что backend должен отдавать



На этапе 4 backend должен отдавать \*\*готовые DTO\*\*, а не только сырые поля.



Frontend не должен:

\- сам собирать score;

\- сам объединять сигналы из разных API;

\- сам решать, какой SKU critical.



Backend должен отдавать:

\- готовую карточку Critical SKU;

\- готовую карточку Stocks \& Replenishment;

\- signal breakdown и итоговый priority/score.



\---



\## 11. Что входит в этап 4



В scope этапа 4 входят:



\- рейтинг проблемных SKU;

\- расчёт риска out-of-stock;

\- расчёт приоритета пополнения;

\- объединение сигналов по SKU в единую карточку;

\- backend services для Critical SKU и Stocks \& Replenishment;

\- API для двух отдельных экранов;

\- две отдельные frontend страницы:

&#x20; - Critical SKU

&#x20; - Stocks \& Replenishment



\---



\## 12. Что не входит в этап 4



В этап 4 \*\*не входят\*\*:



\- AI explanations;

\- forecasting;

\- lead time modelling;

\- reorder point optimisation;

\- supplier / procurement logic;

\- automatic purchase recommendations;

\- complex replenishment optimisation;

\- advanced BI/reporting;

\- heavy filtering/report builder;

\- multi-step workflow automation;

\- promo/ads scoring как часть problem score;

\- полноценная supply-chain аналитика.



\---



\## 13. Ограничения текущего этапа



Этап 4 строится с учётом текущих ограничений проекта:



1\. Реальный Ozon-кабинет пользователя пустой, поэтому основная валидация будет идти на synthetic dataset.

2\. Часть SKU-level semantics в текущем MVP уже является упрощённой и должна оставаться честно описанной.

3\. Stocks используются как current-state snapshot layer, а не как полная история stock changes.

4\. Scoring должен быть explainable и не должен притворяться “умнее”, чем позволяет текущая модель.



\---



\## 14. Результат этапа 4



После завершения этапа 4 пользователь должен получить:



\- отдельный экран \*\*Critical SKU\*\*;

\- отдельный экран \*\*Stocks \& Replenishment\*\*;

\- список SKU, требующих внимания прямо сейчас;

\- список SKU с риском дефицита;

\- список SKU, которые нужно пополнять в первую очередь.



Это должен быть первый действительно сильный operational слой проекта — не просто обзор аналитики, а список конкретных объектов внимания.

