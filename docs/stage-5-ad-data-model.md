Stage 5 — Advertising Data Model

1\. Назначение документа



Этот документ фиксирует минимальную data model для рекламного модуля MVP.



Модель должна:



вписываться в уже существующую архитектуру проекта;

не ломать текущий pattern:

source/raw layer

normalized operational tables

daily aggregates

derived services

API

frontend pages

позволять реализовать первый advertising risk layer без избыточной сложности.



Этап 5 не должен превращаться в отдельную ad-tech платформу.

Нужен минимальный слой, который позволяет:



увидеть кампанию;

увидеть её дневные рекламные метрики;

понять, какие SKU она продвигает;

посчитать базовые рекламные сигналы;

показать Advertising screen.

2\. Контекст проекта



К моменту этапа 5 в проекте уже есть:



auth / seller account / Ozon integration

ingestion layer для:

products

orders

sales

stocks

analytics layer:

daily\_account\_metrics

daily\_sku\_metrics

dashboard services

Critical SKU

Stocks \& Replenishment

frontend operational screens



Это значит, что advertising module должен:



использовать текущий seller account context;

использовать уже существующую SKU identity;

быть совместимым с current stock / sales analytics;

не дублировать существующие business layers.

3\. Контекст Ozon



Рекламный контур Ozon живёт в отдельном Performance API, а не в обычном Seller API. Через него можно собирать статистику по рекламным каналам и работать с товарами в рекламе.



Для проекта это означает:



advertising ingestion — отдельный source domain;

advertising сущности не нужно смешивать с ingestion товаров/заказов/остатков;

связь campaign ↔ SKU должна быть явной;

advertising analytics надо строить поверх уже существующих business layers проекта.

4\. Архитектурный принцип модели



Для рекламы используется тот же pattern, что и для остальных доменов проекта:



source / ingest layer

Получение данных Ozon advertising domain

normalized operational tables

Хранение кампаний, daily metrics, campaign-to-SKU links

derived analytics services

Расчёт advertising signals и risk cards

API

Готовые DTO для frontend

frontend page

Advertising screen



На этапе 5 не требуется полноценный raw-history lake для рекламы, но модель должна быть готова к идемпотентной загрузке и повторному rebuild.



5\. Минимальные сущности advertising domain

5.1. ad\_campaigns



Таблица для хранения самих рекламных кампаний.



Назначение



Позволяет:



идентифицировать кампанию;

знать, к какому seller account она относится;

понимать тип и статус кампании;

использовать кампанию как anchor entity для ad metrics и campaign-SKU links.

Минимальные поля

id — internal id

seller\_account\_id

campaign\_external\_id — внешний campaign id из Ozon

campaign\_name

campaign\_type

placement\_type — если доступно и полезно

status

budget\_amount — если доступно

budget\_daily — если доступно

raw\_attributes — optional JSON subset для будущей совместимости

created\_at

updated\_at

Идемпотентность



Основной уникальный ключ:



(seller\_account\_id, campaign\_external\_id)

5.2. ad\_metrics\_daily



Таблица для хранения дневных рекламных метрик по кампании.



Назначение



Позволяет:



видеть spend и базовый рекламный результат по дням;

строить KPI и advertising signals;

не рассчитывать всё из raw payload на лету.

Минимальные поля

id

seller\_account\_id

campaign\_external\_id

metric\_date

impressions

clicks

spend

orders\_count — если доступно

revenue — если доступно

ctr — optional persisted field или derived later

cpc — optional persisted field или derived later

acos / roas-like fields — optional, если есть смысл хранить

raw\_attributes — optional JSON subset

created\_at

updated\_at

Идемпотентность



Основной уникальный ключ:



(seller\_account\_id, campaign\_external\_id, metric\_date)

Принцип хранения



На этапе 5 допустимы два варианта:



хранить только raw numerics и считать derived advertising ratios в analytics service;

хранить часть derived numerics в таблице, если это упрощает API.



Для MVP предпочтительнее:



хранить базовые raw numerics;

derived indicators считать в analytics service.

5.3. ad\_campaign\_skus



Таблица для связи рекламных кампаний с SKU.



Назначение



Позволяет:



понимать, какие SKU продвигает кампания;

связывать advertising domain с:

products

daily\_sku\_metrics

stock risk

Critical SKU / Replenishment logic

Минимальные поля

id

seller\_account\_id

campaign\_external\_id

ozon\_product\_id

offer\_id — если доступно

sku — если доступно

is\_active — optional

status — optional

created\_at

updated\_at

Идемпотентность



Основной уникальный ключ:



(seller\_account\_id, campaign\_external\_id, ozon\_product\_id)

6\. Почему этих трёх сущностей достаточно для MVP



Этого минимального advertising layer достаточно, чтобы:



увидеть кампанию;

увидеть campaign-level spend и базовый результат;

связать кампанию с SKU;

понять, что конкретный SKU рекламируется;

объединить:

ad spend

ad effectiveness

stock risk

weak sales dynamics



То есть этой модели уже хватает для реализации первых сигналов этапа 5:



рост расхода без заметного результата;

слабая эффективность кампании;

рекламируется товар с низким остатком;

товар тратит бюджет при слабой динамике продаж.

7\. Как advertising model связывается с существующим проектом

7.1. Связь с seller\_account\_id



Все advertising tables должны быть изолированы по seller\_account\_id, как и существующие products/orders/sales/stocks layers.



7.2. Связь с products



Основной мост:



ozon\_product\_id

при наличии:

offer\_id

sku



Это позволяет:



подтягивать metadata товара;

отображать product name;

связывать рекламу с current operational SKU model.

7.3. Связь с daily\_sku\_metrics



Через ozon\_product\_id можно:



сопоставить ad-linked SKU с revenue trend;

сопоставить ad-linked SKU с sales ops dynamics;

сопоставить ad-linked SKU с importance.

7.4. Связь с stocks\_view\_service



Через ту же SKU identity можно:



выявлять low stock / out-of-stock у рекламируемого SKU;

считать advertising + stock risk combined signal.

8\. Что не нужно моделировать на этапе 5



На этом этапе не нужно пытаться идеально покрыть все рекламные сущности Ozon.



Вне scope data model этапа 5:



сложная иерархия ad groups / ad items / placements, если она не нужна для MVP;

полная history-driven attribution model;

хранение всех raw ad event logs;

granular bid history;

creative-level entities;

multi-touch attribution;

внешние рекламные каналы.



На этапе 5 нужен именно минимальный operational ad layer.



9\. Предлагаемые индексы и ключи

ad\_campaigns

unique:

(seller\_account\_id, campaign\_external\_id)

index:

(seller\_account\_id)

(status)

ad\_metrics\_daily

unique:

(seller\_account\_id, campaign\_external\_id, metric\_date)

index:

(seller\_account\_id, metric\_date)

(campaign\_external\_id, metric\_date)

ad\_campaign\_skus

unique:

(seller\_account\_id, campaign\_external\_id, ozon\_product\_id)

index:

(seller\_account\_id, ozon\_product\_id)

(campaign\_external\_id)

10\. Принципы идемпотентности



Advertising ingestion должен следовать тем же принципам, что уже есть в проекте:



campaign upsert по external id;

daily metric upsert по (seller\_account\_id, campaign\_id, metric\_date);

campaign-SKU link upsert по (seller\_account\_id, campaign\_id, ozon\_product\_id).



Это позволит:



безопасно выполнять повторный sync;

обновлять данные без дублей;

хранить стабильный advertising domain.

