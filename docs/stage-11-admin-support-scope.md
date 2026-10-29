\# Stage 11. Admin / Support Tooling — Scope and Boundaries



\## Назначение документа



Этот документ фиксирует scope этапа 11: \*\*Admin / Support Tooling\*\*.



Stage 11 нужен для того, чтобы продукт стал поддерживаемым с операционной точки зрения.



До этого этапа большинство диагностических действий можно было выполнять только через:



\- прямой просмотр БД;

\- backend logs;

\- ручной запуск команд;

\- ручной анализ sync/import jobs;

\- ручной анализ AI traces;

\- ручную проверку OpenAI responses;

\- ручную проверку ошибок валидации AI output.



После stage 11 базовая диагностика и support-действия должны выполняться через внутреннюю админку.



\---



\## Главная цель stage 11



Цель stage 11 — создать внутренний support-инструмент, который помогает отвечать на вопросы:



```text

1\. У какого клиента проблема?

2\. Что сломалось: sync, import, alerts, recommendations, chat?

3\. Когда был последний успешный run?

4\. Что вернул OpenAI?

5\. Почему AI output был отклонён?

6\. Какие действия support выполнил вручную?

````



Админка должна помогать поддержке быстро понять состояние клиента и безопасно выполнить разрешённые операционные действия.



\---



\## Что такое админка в рамках MVP



Админка — это \*\*внутренний support-инструмент\*\*.



Она предназначена для:



\* диагностики проблем клиента;

\* просмотра состояния подключений;

\* просмотра sync/import jobs;

\* просмотра ошибок импорта;

\* просмотра sync cursors;

\* просмотра alert runs;

\* просмотра recommendation runs;

\* просмотра AI recommendation diagnostics;

\* просмотра AI chat traces;

\* просмотра raw AI responses;

\* просмотра validation errors;

\* просмотра user feedback по AI;

\* просмотра billing state;

\* безопасного запуска ограниченных support actions;

\* audit logging всех manual support actions.



\---



\## Чем админка не является



Админка \*\*не является пользовательским dashboard\*\*.



Пользовательский dashboard отвечает на вопрос:



```text

Что мне сегодня сделать в кабинете?

```



Админка отвечает на вопрос:



```text

Что произошло с клиентом и как support может безопасно диагностировать проблему?

```



Админка не должна заменять:



\* `/app/dashboard`;

\* `/app/recommendations`;

\* `/app/alerts`;

\* `/app/chat`;

\* `/app/pricing-constraints`;

\* `/app/critical-skus`;

\* `/app/stocks-replenishment`.



Она должна быть отдельным внутренним инструментом.



\---



\## Пользовательские и support-сценарии разделены



Обычный пользователь работает с:



\* dashboard;

\* alerts;

\* recommendations;

\* AI chat;

\* pricing constraints;

\* analytics screens.



Support/admin работает с:



\* clients list;

\* operational status;

\* sync jobs;

\* import diagnostics;

\* cursors;

\* AI logs;

\* traces;

\* raw AI responses;

\* validation results;

\* feedback;

\* billing state;

\* audit logs;

\* controlled rerun/reset actions.



Эти сценарии нельзя смешивать.



Обычный пользователь не должен иметь доступ к admin endpoints и raw diagnostic payloads.



\---



\## Основной принцип stage 11



Admin / Support Tooling должен быть:



```text

diagnostic-first

audit-logged

seller-scoped

admin-protected

safe-by-default

```



Это означает:



\* сначала диагностика, потом действие;

\* каждое manual support action логируется;

\* все данные привязаны к конкретному seller account;

\* admin endpoints защищены отдельным admin access middleware;

\* опасные действия не выполняются неявно;

\* raw payloads доступны только support/admin;

\* secrets никогда не отображаются.



\---



\## Что реализуется в stage 11



В рамках stage 11 реализуется MVP-админка, которая включает следующие блоки.



\### Clients



Админка должна показывать список клиентов / seller accounts.



Минимально:



\* seller account id;

\* seller name;

\* owner user email;

\* seller status;

\* created/updated timestamps;

\* Ozon connection status;

\* latest sync status;

\* latest import status;

\* latest alert run;

\* latest recommendation run;

\* latest chat trace;

\* open alerts count;

\* open recommendations count;

\* billing state.



Цель блока — быстро понять, у какого клиента есть проблема.



\---



\### Connection statuses



Админка должна показывать состояние подключений клиента.



Минимально:



\* Ozon connection exists / missing;

\* connection status;

\* last check time;

\* last successful check;

\* last error;

\* auth/configuration state without secrets.



Support должен видеть, подключён ли клиент и есть ли проблема с авторизацией / доступом.



\---



\### Sync jobs



Админка должна показывать список sync jobs.



Минимально:



\* sync job id;

\* seller account id;

\* job type;

\* status;

\* started\_at;

\* finished\_at;

\* error\_message;

\* created\_at;

\* updated\_at.



Цель — понять, запускалась ли синхронизация, завершилась ли она успешно и где сломалась.



\---



\### Import jobs



Админка должна показывать import jobs.



Минимально:



\* import job id;

\* sync job id;

\* domain;

\* status;

\* source cursor / period;

\* records received;

\* records imported;

\* records failed;

\* started\_at;

\* finished\_at;

\* error\_message.



Цель — понять, какой домен импорта сломался:



\* products;

\* orders;

\* stocks;

\* ads;

\* other future domains.



\---



\### Import errors



В MVP import errors могут отображаться на основе `import\_jobs.error\_message`.



Если в будущем появится per-record error table, админка должна показывать и её.



Цель — дать support возможность понять, почему импорт не завершился:



\* ошибка API;

\* ошибка parsing;

\* ошибка storage;

\* ошибка idempotency/upsert;

\* ошибка schema mismatch;

\* ошибка network/timeout/rate limit.



\---



\### Sync cursors



Админка должна показывать sync cursors.



Минимально:



\* seller account id;

\* domain;

\* cursor type;

\* cursor value;

\* updated\_at.



Цель — понять, откуда следующий sync продолжит загрузку данных.



Support должен иметь возможность reset/update cursor только через явное action с audit log.



\---



\### Metrics runs / recalculation diagnostics



Админка должна поддерживать диагностику и ручной rerun предвычисленных метрик.



В MVP это может быть реализовано как action:



```text

rerun metrics

```



Цель — пересчитать агрегаты после:



\* исправления импорта;

\* повторной синхронизации;

\* изменения расчётной логики;

\* backfill данных.



\---



\### Alerts runs



Админка должна показывать alert runs.



Минимально:



\* alert run id;

\* seller account id;

\* run type;

\* status;

\* as\_of\_date;

\* generated count;

\* resolved count;

\* error\_message;

\* started\_at;

\* finished\_at.



Support должен иметь возможность вручную запустить:



```text

rerun alerts

```



Это action должен быть audit-logged.



\---



\### Recommendation runs



Админка должна показывать AI recommendation runs.



Минимально:



\* recommendation run id;

\* seller account id;

\* run type;

\* status;

\* as\_of\_date;

\* AI model;

\* prompt version;

\* generated recommendations count;

\* accepted/rejected/failed counts, если доступны;

\* input tokens;

\* output tokens;

\* estimated cost;

\* error\_message;

\* started\_at;

\* finished\_at.



Support должен иметь возможность вручную запустить:



```text

rerun recommendations

```



Это action должен быть audit-logged, потому что может вызвать OpenAI API и создать дополнительные расходы.



\---



\### AI recommendation logs



После stage 8 AI-рекомендации являются критичным AI-слоем продукта.



В админке обязательно нужно видеть диагностику Recommendation Engine:



\* OpenAI model;

\* OpenAI request id, если доступен;

\* prompt version;

\* context payload summary;

\* raw OpenAI response;

\* parsed AI output;

\* validation result;

\* rejected recommendations;

\* accepted recommendations;

\* token usage;

\* estimated cost;

\* error code;

\* timeout/rate limit;

\* final run status.



Если исторические rejected payloads не сохранялись на ранних этапах, это нужно явно отображать как limitation:



```text

Rejected item payloads are unavailable for historical runs.

```



\---



\### AI chat logs



После stage 10 AI Chat стал вторым AI-интерфейсом продукта.



В админке обязательно нужно видеть chat diagnostics:



\* chat session;

\* user question;

\* detected intent;

\* planner model;

\* answer model;

\* planner prompt version;

\* answer prompt version;

\* raw planner response;

\* raw answer response;

\* proposed tool plan;

\* validated tool plan;

\* tool results;

\* fact context;

\* answer validation result;

\* final confidence level;

\* input tokens;

\* output tokens;

\* estimated cost;

\* error\_message;

\* trace status.



Цель — понять, почему чат ответил именно так или почему он не смог ответить.



\---



\### Raw AI responses



Raw AI responses доступны только admin/support.



Они нужны для диагностики:



\* OpenAI вернул невалидный JSON;

\* Planner выбрал неправильные tools;

\* Answerer сослался на недоступные данные;

\* Answerer нарушил contract;

\* output validation отклонила ответ;

\* произошёл timeout/rate limit/provider error.



Raw AI responses не должны быть доступны обычному пользователю.



\---



\### Validation errors AI output



Админка должна показывать validation errors для AI output.



Для Recommendations:



\* invalid JSON;

\* missing required fields;

\* invalid enum values;

\* SKU/product not found;

\* price below min;

\* price above max;

\* margin guardrail violation;

\* stock/ad contradiction;

\* unknown related alert;

\* missing supporting metrics.



Для Chat:



\* invalid answer JSON;

\* empty answer;

\* invalid confidence;

\* related alert id not found;

\* related recommendation id not found;

\* empty supporting facts;

\* auto-action claim;

\* direct DB/Ozon access claim;

\* ignored context limitations;

\* secret/raw marker detected.



\---



\### Feedback по ответам чата



Админка должна показывать feedback по AI Chat.



Минимально:



\* seller account;

\* session id;

\* message id;

\* rating;

\* comment;

\* user question;

\* assistant answer;

\* trace id, если можно связать;

\* created\_at.



Цель — понимать, какие ответы были полезными, а какие нет.



\---



\### Feedback по рекомендациям



Админка должна показывать feedback/status по AI Recommendations.



В MVP уже есть action-statuses рекомендаций:



\* accepted;

\* dismissed;

\* resolved.



Если будет добавлена отдельная recommendation feedback table, админка должна показывать:



\* rating;

\* comment;

\* recommendation id;

\* recommendation type;

\* created\_at.



Если отдельной feedback table ещё нет, статусы accepted/dismissed/resolved можно использовать как MVP proxy feedback.



\---



\### Billing state



Админка должна показывать billing state клиента.



В MVP это не полноценный billing engine.



Минимально:



\* plan code;

\* billing status;

\* trial dates;

\* current period;

\* AI token limit;

\* AI tokens used;

\* estimated AI cost;

\* notes;

\* updated\_at.



Цель — support должен понимать:



\* клиент на trial или active;

\* есть ли billing ограничения;

\* сколько AI usage накоплено;

\* есть ли billing-related notes.



\---



\## Admin actions в scope stage 11



Stage 11 включает ограниченный набор support actions.



\### Rerun sync



Support может вручную запустить повторную синхронизацию.



Важно:



\* action создаёт новый sync job;

\* старый failed job не должен молча перезаписываться;

\* action должен быть audit-logged;

\* результат должен быть виден в sync jobs.



\---



\### Reset cursor



Support может вручную сбросить sync cursor.



Важно:



\* действие потенциально опасное;

\* может привести к повторному импорту данных;

\* требует confirmation в UI;

\* требует audit log;

\* должен быть виден старый и новый cursor value.



\---



\### Rerun metrics



Support может вручную запустить пересчёт агрегатов.



Важно:



\* action должен быть audit-logged;

\* нужно указывать период;

\* результат должен быть виден support’у;

\* ошибки пересчёта должны сохраняться.



\---



\### Rerun alerts



Support может вручную запустить Alerts Engine.



Важно:



\* action должен быть audit-logged;

\* нужно указывать `as\_of\_date`;

\* результат должен быть виден через alert runs;

\* ошибки должны отображаться.



\---



\### Rerun recommendations



Support может вручную запустить AI Recommendation Engine.



Важно:



\* action вызывает OpenAI API;

\* может привести к дополнительной стоимости;

\* action должен быть audit-logged;

\* нужно показывать token usage и estimated cost;

\* ошибки OpenAI/validation должны отображаться.



\---



\## Какие данные можно показывать support/admin



Support/admin может видеть:



\* seller account metadata;

\* user email владельца аккаунта;

\* connection status без секретов;

\* sync jobs;

\* import jobs;

\* import error messages;

\* sync cursors;

\* alert runs;

\* alerts;

\* recommendation runs;

\* recommendations;

\* chat sessions;

\* chat messages;

\* chat traces;

\* raw planner response;

\* raw answer response;

\* raw recommendation AI response;

\* validated tool plans;

\* tool results;

\* fact context;

\* validation payloads;

\* token usage;

\* estimated cost;

\* feedback;

\* billing state;

\* admin action logs.



\---



\## Какие данные нельзя показывать support/admin



Даже в админке нельзя показывать:



\* OpenAI API key;

\* Ozon API key;

\* Ozon Client-Id, если он считается чувствительным;

\* Bearer tokens;

\* Authorization headers;

\* session cookies;

\* password hash;

\* database connection string;

\* `.env` contents;

\* raw credentials;

\* encrypted secrets;

\* refresh tokens;

\* access tokens;

\* данные другого seller account вне выбранного контекста;

\* любые secrets из trace payloads.



Если такие данные случайно попали в raw payload, они должны быть замаскированы до отображения.



\---



\## Какие данные нельзя возвращать обычному пользователю



Обычный пользователь не должен получать через публичные app endpoints:



\* raw planner response;

\* raw answer response;

\* raw recommendation AI response;

\* full tool plan;

\* full fact context;

\* trace payloads;

\* token usage;

\* estimated cost;

\* internal validation payload;

\* admin action logs;

\* billing internal notes;

\* raw provider error body.



Эти данные доступны только во внутренней админке.



\---



\## Admin endpoints security



Все endpoints вида:



```text

/api/v1/admin/\*

```



должны быть защищены отдельным admin access middleware.



Минимальные требования:



\* пользователь должен быть authenticated;

\* пользователь должен быть admin/support;

\* admin статус нельзя передавать из request body;

\* seller account id в admin path должен использоваться только как target object, а не как auth scope;

\* все actions должны писать audit log;

\* все raw payload responses должны проходить sanitization.



Обычный seller user должен получать:



```text

403 Forbidden

```



при попытке обратиться к admin endpoints.



\---



\## Audit log requirements



Все manual support actions должны писать audit log.



Audit log обязателен для:



\* rerun sync;

\* reset cursor;

\* rerun metrics;

\* rerun alerts;

\* rerun recommendations;

\* update billing state;

\* future destructive/support actions.



Audit log должен фиксировать:



\* admin user id;

\* admin email;

\* seller account id;

\* action type;

\* target type;

\* target id;

\* request payload;

\* result payload;

\* status;

\* error message;

\* created\_at;

\* finished\_at.



Audit log нужен для ответа на вопрос:



```text

Какие действия support выполнил вручную?

```



\---



\## Raw AI responses access



Raw AI responses можно показывать только admin/support.



Они должны быть:



\* collapsed by default в UI;

\* помечены как internal support data;

\* sanitized;

\* seller-scoped;

\* недоступны обычному пользователю.



UI должен явно показывать, что это диагностические данные:



```text

Internal support data. Do not share externally.

```



\---



\## Auto-actions через админку



Админка не должна выполнять auto-actions за клиента неявно.



Запрещено:



\* автоматически менять цены без явной кнопки/action;

\* автоматически менять рекламные бюджеты;

\* автоматически останавливать кампании;

\* автоматически создавать поставки;

\* автоматически принимать рекомендации;

\* автоматически закрывать alert’ы.



Если в будущем такие actions появятся, они должны быть:



\* отдельными explicit actions;

\* с confirmation;

\* с audit log;

\* с понятным target object;

\* с rollback/diagnostic strategy, если применимо.



В stage 11 таких auto-actions нет.



\---



\## Rerun/reset actions не являются auto-actions клиента



Важно различать:



```text

support operational action

```



и



```text

client business action

```



Rerun/reset actions в stage 11 — это support operational actions.



Они могут:



\* перезапустить синхронизацию;

\* сбросить cursor;

\* пересчитать метрики;

\* перезапустить alerts;

\* перезапустить recommendations.



Они не должны:



\* менять цену в Ozon;

\* менять рекламную кампанию;

\* создавать поставку;

\* менять ассортимент;

\* выполнять бизнес-решение за клиента.



\---



\## Error handling в админке



Админка должна показывать ошибки так, чтобы support мог понять проблему.



Для sync/import:



\* HTTP/API error;

\* timeout;

\* rate limit;

\* parsing error;

\* DB/storage error;

\* validation error.



Для AI:



\* OpenAI error;

\* timeout;

\* rate limit;

\* invalid JSON;

\* invalid tool plan;

\* answer validation failed;

\* recommendation validation failed;

\* context too large;

\* missing data.



Для actions:



\* action failed;

\* action partially completed;

\* action completed.



Ошибки не должны раскрывать secrets.



\---



\## Что не входит в stage 11



Stage 11 не включает:



\* полноценный billing engine;

\* платежи;

\* invoices;

\* subscription management;

\* RBAC с множеством ролей;

\* customer-facing admin;

\* full BI-конструктор;

\* редактирование raw data;

\* редактирование AI prompts из UI;

\* replay exact OpenAI request из UI;

\* streaming logs;

\* автоматическое исправление ошибок;

\* массовые destructive actions;

\* управление Ozon ценами;

\* управление Ozon рекламой;

\* создание поставок;

\* полноценную observability platform.



\---



\## MVP boundaries



В MVP stage 11 допускается:



\* admin access через env allowlist;

\* billing state как support-visible record, а не billing engine;

\* import errors на основе `import\_jobs.error\_message`;

\* recommendation feedback через statuses accepted/dismissed/resolved, если отдельная feedback table ещё не добавлена;

\* partial AI recommendation diagnostics для historical runs, если rejected payloads раньше не сохранялись;

\* raw JSON blocks в UI collapsed by default.



\---



\## Acceptance criteria



Stage 11 можно считать закрытым, если выполнены условия.



\### Backend



\* `/api/v1/admin/\*` защищён admin middleware;

\* обычный user получает `403`;

\* admin может получить clients list;

\* admin может открыть client detail;

\* admin видит connection statuses;

\* admin видит sync jobs;

\* admin видит import jobs/errors;

\* admin видит sync cursors;

\* admin видит alert runs;

\* admin видит recommendation runs;

\* admin видит chat traces;

\* admin видит feedback;

\* admin видит billing state;

\* admin actions пишут audit log;

\* raw AI payloads sanitization сохраняется.



\### Frontend



\* есть `/app/admin`;

\* admin видит clients list;

\* admin видит client detail;

\* есть sections/tabs:



&#x20; \* overview;

&#x20; \* sync/import;

&#x20; \* cursors;

&#x20; \* alerts;

&#x20; \* recommendations;

&#x20; \* AI chat logs;

&#x20; \* feedback;

&#x20; \* billing;

&#x20; \* admin actions;

\* dangerous actions требуют confirmation;

\* raw AI payloads collapsed by default;

\* loading/error/empty states есть.



\### Safety



\* secrets не отображаются;

\* raw AI только admin/support;

\* actions audit-logged;

\* seller scope соблюдён;

\* обычный user не видит admin;

\* admin action не выполняется без явного запроса.



\---



\## Итоговая формулировка stage 11



После stage 11 поддержка клиентов и диагностика проблем должны выполняться через админку, а не вручную через БД и консоль.



Главный результат:



```text

Support can diagnose client, sync, import, AI recommendation, AI chat, feedback and billing issues from one internal tool.

```



При этом:



```text

Admin tooling is internal, protected, audit-logged and safe-by-default.

```



```

```



