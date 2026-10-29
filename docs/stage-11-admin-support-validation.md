\# Stage 11. Admin / Support Tooling — Validation Checklist



\## Назначение документа



Этот документ фиксирует проверочные сценарии для Stage 11: \*\*Admin / Support Tooling\*\*.



Цель проверки — убедиться, что админка действительно позволяет support/admin пользователю диагностировать клиентов, sync/import процессы, AI Recommendations, AI Chat, feedback и billing state без ручного обращения к БД и консоли.



\---



\## Scope проверки



Проверяются:



\- admin access model;

\- admin frontend navigation;

\- clients list;

\- client detail;

\- connection statuses;

\- sync/import diagnostics;

\- sync cursors;

\- operational actions;

\- AI recommendation diagnostics;

\- AI chat diagnostics;

\- feedback tooling;

\- billing state;

\- security/safety;

\- build/tests.



\---



\## Preconditions



Перед проверкой должны быть выполнены условия:



\- backend запущен локально;

\- frontend запущен локально;

\- PostgreSQL доступен;

\- миграции применены;

\- есть минимум один seller account;

\- есть минимум один обычный seller user;

\- есть минимум один admin/support user, email которого указан в `ADMIN\_EMAILS`;

\- у тестового seller account есть данные sync/import/metrics/alerts/recommendations/chat или подготовлены empty-state сценарии.



Пример env:



```text

ADMIN\_EMAILS=admin@example.com

Базовые команды проверки



Backend:



cd backend

go test ./...



Frontend:



cd frontend

npm run build

npm run lint



Если npm run lint в локальном окружении зависает или падает на уже известных legacy-ошибках, нужно зафиксировать это отдельно и дополнительно проверить изменённые файлы таргетно через ESLint / IDE diagnostics.



1\. Обычный user не видит admin

Цель



Проверить, что обычный seller user не видит entrypoint в админку и не может обращаться к admin API.



Шаги

Авторизоваться обычным seller user.

Открыть /app.

Проверить блок navigation / technical screens.

Попробовать вручную открыть /app/admin.

Попробовать вызвать:

GET /api/v1/admin/me

GET /api/v1/admin/clients

Ожидаемый результат

На /app нет ссылки Admin / Support.

/api/v1/admin/me возвращает 403 Forbidden.

/api/v1/admin/clients возвращает 403 Forbidden.

Обычный пользователь не получает clients list, raw AI payloads, billing state или admin action logs.

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

2\. Admin видит clients list

Цель



Проверить, что admin/support user видит список клиентов.



Шаги

Авторизоваться пользователем из ADMIN\_EMAILS.

Открыть /app.

Убедиться, что отображается ссылка Admin / Support.

Перейти в /app/admin.

Проверить загрузку clients list.

Проверить фильтры:

search;

seller status;

connection status;

billing status.

Ожидаемый результат

Ссылка Admin / Support видна только admin user.

/app/admin открывается.

Clients list загружается.

Для клиента отображаются:

seller name;

owner email;

seller status;

connection status;

latest sync status;

open alerts count;

open recommendations count;

latest AI status;

billing status.

Фильтры не ломают страницу.

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

3\. Admin открывает client detail

Цель



Проверить, что admin может открыть карточку клиента.



Шаги

На /app/admin выбрать клиента.

Перейти в /app/admin/clients/{seller\_account\_id}.

Проверить header и вкладки.

Ожидаемый результат



На client detail отображаются:



seller account id;

seller name;

owner email;

seller status;

connection badge;

billing badge;

tabs:

Overview;

Sync / Import;

Cursors;

Alerts;

Recommendations;

AI Chat Logs;

Feedback;

Billing;

Admin actions.

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

4\. Видны statuses подключений

Цель



Проверить, что support видит состояние подключений клиента.



Шаги

Открыть client detail.

Перейти во вкладку Overview.

Проверить блок connections.

Ожидаемый результат



Отображаются:



provider;

status;

last check;

last successful check / result, если есть;

last error, если есть;

updated\_at.



Не отображаются:



Ozon API key;

encrypted credentials;

bearer tokens;

authorization headers.

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

5\. Видны sync jobs

Цель



Проверить, что support видит историю sync jobs.



Шаги

Открыть client detail.

Перейти во вкладку Sync / Import.

Проверить таблицу sync jobs.

Проверить фильтр status, если он доступен.

Ожидаемый результат



В sync jobs отображаются:



id;

type;

status;

started\_at;

finished\_at;

error\_message;

created\_at.



Если sync jobs отсутствуют, отображается empty state:



No sync jobs found.

API endpoint

GET /api/v1/admin/clients/{seller\_account\_id}/sync-jobs

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

6\. Видны import errors

Цель



Проверить, что support видит ошибки импорта.



Шаги

Открыть client detail.

Перейти во вкладку Sync / Import.

Проверить блок import errors.

Если есть failed import job, убедиться, что error отображается.

Ожидаемый результат



В import errors отображаются:



import\_job\_id;

sync\_job\_id;

domain;

status;

error\_message;

records\_failed;

started\_at;

finished\_at.



Если ошибок нет, отображается empty state:



No import errors found.

API endpoint

GET /api/v1/admin/clients/{seller\_account\_id}/import-errors

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

7\. Видны sync cursors

Цель



Проверить, что support видит sync cursors клиента.



Шаги

Открыть client detail.

Перейти во вкладку Cursors.

Проверить таблицу cursors.

Ожидаемый результат



Отображаются:



domain;

cursor\_type;

cursor\_value;

updated\_at.



Если cursors отсутствуют, отображается empty state:



No sync cursors found.

API endpoint

GET /api/v1/admin/clients/{seller\_account\_id}/sync-cursors

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

8\. Rerun sync создаёт sync job и audit log

Цель



Проверить, что support может безопасно запустить новый sync job, а действие audit-логируется.



Шаги

Открыть client detail.

Перейти во вкладку Admin actions.

В блоке Rerun sync выбрать initial\_sync.

Нажать кнопку запуска.

Подтвердить action в confirmation.

Проверить результат на UI.

Проверить sync jobs.

Проверить admin action log в БД или через будущий admin action logs UI/API.

Ожидаемый результат

Появляется confirmation.

Во время выполнения кнопка disabled/loading.

После выполнения отображается:

action log id;

status;

result payload;

error, если action failed.

Создаётся новый sync job.

Старый failed sync job не переиспользуется.

В admin\_action\_logs появляется запись:

action\_type = rerun\_sync;

status = completed или failed;

seller\_account\_id;

admin\_email.

API endpoint

POST /api/v1/admin/clients/{seller\_account\_id}/actions/rerun-sync

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

9\. Reset cursor меняет cursor и пишет audit log

Цель



Проверить, что reset cursor работает безопасно и audit-логируется.



Шаги

Открыть client detail.

Перейти во вкладку Admin actions.

В блоке Reset cursor заполнить:

domain;

cursor\_type;

cursor\_value или оставить пустым для null.

Проверить, что UI показывает warning о возможном re-import.

Нажать reset.

Подтвердить action.

Перейти во вкладку Cursors.

Проверить изменённый cursor.

Проверить audit log.

Ожидаемый результат

Перед action есть warning:

resetting a cursor can cause the next sync to re-import data

Confirmation обязателен.

Cursor обновляется или сбрасывается.

В admin\_action\_logs появляется запись:

action\_type = reset\_cursor;

status = completed или failed;

request payload содержит domain/cursor\_type/cursor\_value;

secrets отсутствуют.

API endpoint

POST /api/v1/admin/clients/{seller\_account\_id}/actions/reset-cursor

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

10\. Rerun metrics пересчитывает aggregates

Цель



Проверить, что rerun metrics action корректно обрабатывается.



Шаги

Открыть client detail.

Перейти во вкладку Admin actions.

В блоке Rerun metrics указать:

date\_from;

date\_to.

Нажать action.

Подтвердить.

Проверить результат.

Ожидаемый результат



Если metrics rerunner подключён:



агрегаты пересчитаны за период;

action log completed;

result payload содержит summary.



Если metrics rerunner не подключён в текущем MVP:



action log создаётся;

action log получает status = failed;

UI показывает ошибку вроде:

admin action dependency is not configured

это считается допустимым MVP-поведением, если явно зафиксировано.

API endpoint

POST /api/v1/admin/clients/{seller\_account\_id}/actions/rerun-metrics

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

11\. Rerun alerts создаёт alert run

Цель



Проверить, что support может вручную перезапустить Alerts Engine.



Шаги

Открыть client detail.

Перейти во вкладку Admin actions.

В блоке Rerun alerts указать as\_of\_date.

Нажать action.

Подтвердить.

Проверить результат.

Проверить latest alert run / alert diagnostics.

Ожидаемый результат

Создаётся новый alert run или запускается manual/backfill run.

Action log получает completed или failed.

UI показывает action log id/status/result/error.

Ошибка не скрывается.

API endpoint

POST /api/v1/admin/clients/{seller\_account\_id}/actions/rerun-alerts

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

12\. Rerun recommendations создаёт recommendation run

Цель



Проверить, что support может вручную перезапустить AI Recommendation Engine.



Шаги

Открыть client detail.

Перейти во вкладку Admin actions.

В блоке Rerun recommendations указать as\_of\_date.

Проверить, что UI показывает warning про OpenAI token usage / cost.

Нажать action.

Подтвердить.

Проверить result.

Перейти во вкладку Recommendations.

Проверить, что появился recommendation run.

Ожидаемый результат

Перед action есть warning:

This can call OpenAI API and create additional token usage/cost.

Confirmation обязателен.

Создаётся recommendation run.

Action log получает completed или failed.

Если OpenAI error / rate limit / validation error — ошибка отображается, не скрывается.

В diagnostics видны model, prompt version, token usage, estimated cost, error.

API endpoint

POST /api/v1/admin/clients/{seller\_account\_id}/actions/rerun-recommendations

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

13\. Видны AI recommendation logs

Цель



Проверить, что support видит диагностику AI Recommendation Engine.



Шаги

Открыть client detail.

Перейти во вкладку Recommendations.

Проверить list recommendation runs.

Нажать View diagnostics у run.

Проверить detail.

Если есть recommendation item, открыть View recommendation raw AI.

Ожидаемый результат



В recommendation runs list видны:



run id;

run\_type;

status;

as\_of\_date;

ai\_model;

ai\_prompt\_version;

input\_tokens;

output\_tokens;

estimated\_cost;

generated count;

accepted count;

rejected count;

error\_message;

started\_at;

finished\_at.



В detail видны:



associated recommendations, если связь доступна;

diagnostics;

raw OpenAI response;

validation result;

rejected items;

limitations для historical runs, если diagnostics отсутствуют.



Raw blocks collapsed by default и помечены:



Internal support data. Do not share externally.

API endpoints

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendation-runs

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendation-runs/{run\_id}

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendations/{id}

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

14\. Видны AI chat traces

Цель



Проверить, что support видит диагностику AI Chat.



Шаги

Открыть client detail.

Перейти во вкладку AI Chat Logs.

Проверить traces list.

Использовать фильтры:

status;

intent;

session\_id.

Нажать View trace.

Проверить detail.

Ожидаемый результат



В traces list видны:



trace id;

session id;

user\_message\_id;

assistant\_message\_id;

detected\_intent;

status;

planner\_model;

answer\_model;

prompt versions;

input\_tokens;

output\_tokens;

estimated\_cost;

error\_message;

started\_at;

finished\_at.



В trace detail видны:



user question;

assistant answer;

tool plan;

validated tool plan;

tool results;

fact context;

raw planner response;

raw answer response;

answer validation payload;

limitations.



Raw blocks collapsed by default и помечены:



Internal support data. Do not share externally.

API endpoints

GET /api/v1/admin/clients/{seller\_account\_id}/ai/chat-traces

GET /api/v1/admin/clients/{seller\_account\_id}/ai/chat-traces/{trace\_id}

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

15\. Raw AI responses доступны только admin

Цель



Проверить, что raw AI responses не доступны обычным пользователям.



Шаги

Открыть raw AI admin endpoints обычным seller user.

Открыть те же endpoints admin user.

Проверить frontend обычного пользователя.

Проверить frontend admin.

Ожидаемый результат



Обычный user:



получает 403;

не видит raw AI blocks;

не видит admin page.



Admin:



видит raw AI blocks;

raw blocks collapsed by default;

raw blocks read-only;

есть warning label.

Каждый успешный просмотр recommendation run detail, raw recommendation AI или chat trace detail создаёт запись в admin_action_logs (action_type = view_raw_ai_payload, target_type/target_id, seller_account_id, admin identity); тело raw response и полные prompts/context в audit не пишутся. Если запись audit не удалась, API не возвращает raw payload (ожидаемый ответ 503 для admin handler).

Проверяемые endpoints

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendation-runs/{run\_id}

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendations/{id}

GET /api/v1/admin/clients/{seller\_account\_id}/ai/chat-traces/{trace\_id}

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

16\. Validation errors отображаются

Цель



Проверить, что AI validation errors видны support’у.



Шаги

Найти recommendation run или chat trace с validation error.

Открыть соответствующую diagnostics вкладку.

Проверить validation payload/error message.

Ожидаемый результат



Для recommendations видны:



validation\_result\_payload;

rejected\_items\_payload;

error\_stage;

error\_message.



Для chat видны:



answer\_validation\_payload;

trace status;

error\_message;

limitations.



Если historical diagnostics отсутствуют, отображается limitation.



Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

17\. Chat feedback отображается

Цель



Проверить, что support видит feedback по AI Chat.



Шаги

Оставить feedback на chat answer в пользовательском интерфейсе.

Открыть admin client detail.

Перейти во вкладку Feedback.

Проверить chat feedback.

Проверить global endpoint.

Ожидаемый результат



В chat feedback отображаются:



rating;

comment;

message;

session;

trace id, если найден;

created\_at.



Global endpoint показывает feedback по всем клиентам или с фильтром seller.



API endpoints

GET /api/v1/admin/clients/{seller\_account\_id}/feedback/chat

GET /api/v1/admin/feedback/chat

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

18\. Recommendation feedback/statuses отображаются

Цель



Проверить, что support видит feedback по AI Recommendations.



Шаги

Отправить recommendation feedback через API или UI, если UI есть.

Открыть admin client detail.

Перейти во вкладку Feedback.

Проверить recommendation feedback.

Проверить proxy counts по statuses.

Ожидаемый результат



В recommendation feedback отображаются:



rating;

comment;

recommendation title;

recommendation status;

priority/confidence;

created\_at.



Также отображается proxy status feedback:



accepted\_count;

dismissed\_count;

resolved\_count.



Есть limitation:



Historical recommendation feedback is represented by recommendation statuses accepted/dismissed/resolved.

API endpoints

POST /api/v1/recommendations/{id}/feedback

GET /api/v1/admin/clients/{seller\_account\_id}/feedback/recommendations

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

19\. Billing state отображается

Цель



Проверить, что support видит billing state клиента.



Шаги

Открыть client detail.

Перейти во вкладку Billing.

Проверить billing fields.

Проверить global billing list.

Если нужно, обновить billing state через API.

Ожидаемый результат



Отображаются:



plan\_code;

status;

trial\_ends\_at;

current\_period\_start;

current\_period\_end;

ai\_tokens\_limit\_month;

ai\_tokens\_used\_month;

estimated\_ai\_cost\_month;

notes;

updated\_at.



Если billing state отсутствует, отображается:



Billing state is not configured for this client.



При update billing state:



создаётся audit log;

response содержит action log;

обычный user не может выполнить update.

API endpoints

GET /api/v1/admin/clients/{seller\_account\_id}/billing

PUT /api/v1/admin/clients/{seller\_account\_id}/billing

GET /api/v1/admin/billing

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

20\. Secrets не отображаются

Цель



Проверить, что админка не раскрывает secrets.



Шаги

Открыть все admin sections:

Overview;

Sync / Import;

Cursors;

Recommendations;

AI Chat Logs;

Feedback;

Billing;

Admin actions.

Проверить API responses для admin endpoints.

Проверить raw AI blocks.

Проверить action payloads.

Не должны отображаться

OpenAI API key;

Ozon API key;

encrypted Ozon credentials;

Bearer tokens;

Authorization headers;

session cookies;

session tokens;

password hashes;

database connection strings;

.env contents;

admin allowlist;

raw credentials.

Ожидаемый результат

Secrets отсутствуют.

Если payload содержит чувствительное значение, оно замаскировано.

Нет кнопки copy API key.

Нет raw JSON editor.

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

21\. Build/tests проходят

Цель



Проверить, что изменения stage 11 не ломают проект.



Backend checks

cd backend

sqlc generate

go test ./...



Ожидаемый результат:



PASS

Frontend checks

cd frontend

npm run build

npm run lint



Ожидаемый результат:



npm run build проходит;

npm run lint проходит.



Если npm run lint падает на заранее известных legacy-ошибках, нужно:



зафиксировать файл и ошибку;

проверить изменённые stage 11 файлы таргетно;

не считать stage fully clean, пока общий lint не исправлен.

Stage 11 frontend files to check

frontend/lib/admin-api.ts

frontend/components/admin-screen.tsx

frontend/components/admin-client-detail-screen.tsx

frontend/components/admin-support-link.tsx

frontend/app/app/admin/page.tsx

frontend/app/app/admin/clients/\[id]/page.tsx

frontend/app/app/page.tsx

Статус

\[ ] Passed

\[ ] Failed

\[ ] Blocked

Дополнительные API smoke checks

Admin me

curl http://localhost:8081/api/v1/admin/me -b cookies\_admin.txt



Expected:



{

&#x20; "is\_admin": true,

&#x20; "email": "admin@example.com"

}

Non-admin forbidden

curl http://localhost:8081/api/v1/admin/clients -b cookies\_seller.txt



Expected:



403 Forbidden

Clients

curl http://localhost:8081/api/v1/admin/clients -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1 -b cookies\_admin.txt

Sync/import

curl http://localhost:8081/api/v1/admin/clients/1/sync-jobs -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1/import-jobs -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1/import-errors -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1/sync-cursors -b cookies\_admin.txt

Actions

curl -X POST http://localhost:8081/api/v1/admin/clients/1/actions/rerun-sync \\

&#x20; -H "Content-Type: application/json" \\

&#x20; -b cookies\_admin.txt \\

&#x20; -d '{"sync\_type":"initial\_sync"}'



curl -X POST http://localhost:8081/api/v1/admin/clients/1/actions/rerun-alerts \\

&#x20; -H "Content-Type: application/json" \\

&#x20; -b cookies\_admin.txt \\

&#x20; -d '{"as\_of\_date":"2026-04-30"}'

AI diagnostics

curl http://localhost:8081/api/v1/admin/clients/1/ai/recommendation-runs -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1/ai/chat-traces -b cookies\_admin.txt

Feedback

curl http://localhost:8081/api/v1/admin/clients/1/feedback/chat -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/feedback/chat -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/clients/1/feedback/recommendations -b cookies\_admin.txt

Billing

curl http://localhost:8081/api/v1/admin/clients/1/billing -b cookies\_admin.txt

curl http://localhost:8081/api/v1/admin/billing -b cookies\_admin.txt

Final acceptance criteria



Stage 11 можно считать закрытым, если:



обычный user не видит admin и получает 403;

admin видит clients list;

admin открывает client detail;

connection statuses видны;

sync jobs видны;

import jobs/errors видны;

sync cursors видны;

operational actions работают или корректно fail-log’ируются;

actions пишут audit log;

AI recommendation logs видны;

AI chat traces видны;

raw AI responses доступны только admin;

validation errors отображаются;

feedback отображается;

billing state отображается;

secrets не отображаются;

raw blocks collapsed by default;

dangerous actions требуют confirmation;

backend tests проходят;

frontend build проходит;

frontend lint проходит или все stage 11 файлы проверены отдельно и общий lint debt зафиксирован.

Итоговый вердикт



После прохождения validation checklist Stage 11 можно считать выполненным, если support может диагностировать клиента через админку без ручного обращения к БД/консоли и без раскрытия secrets.



Ключевой результат:



Support can diagnose clients, sync/import, AI recommendations, AI chat, feedback, billing and support actions from one protected admin interface.

