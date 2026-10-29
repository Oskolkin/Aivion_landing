\# Stage 11. Admin / Support Tooling — Security and Audit



\## Назначение документа



Этот документ фиксирует security- и audit-принципы для Stage 11: \*\*Admin / Support Tooling\*\*.



Админка является внутренним support-инструментом. Она предназначена для диагностики состояния клиентов, sync/import процессов, AI-рекомендаций, AI-чата, feedback и billing state.



Админка не является пользовательским dashboard и не должна быть доступна обычным seller users.



\---



\## Основной security-принцип



Admin / Support Tooling должен быть:



```text

admin-only

seller-scoped

audit-logged

safe-by-default

no-secrets

explicit-action-only



Это означает:



доступ есть только у admin/support пользователей;

данные всегда отображаются в контексте конкретного seller account;

ручные support actions логируются;

опасные действия требуют явного подтверждения;

secrets не отображаются в UI и не возвращаются из API;

админка не выполняет бизнес-действия за клиента без явной кнопки/action.

Кто имеет доступ к admin



Доступ к /api/v1/admin/\* имеют только пользователи, которые:



аутентифицированы в системе;

входят в backend allowlist ADMIN\_EMAILS.



MVP-модель доступа:



ADMIN\_EMAILS=admin@example.com,support@example.com



Проверка выполняется только на backend.



Frontend не определяет admin-доступ самостоятельно. Frontend только вызывает:



GET /api/v1/admin/me



и показывает ссылку /app/admin, если backend вернул:



{

&#x20; "is\_admin": true,

&#x20; "email": "admin@example.com"

}

Что происходит с обычным пользователем



Обычный seller user:



не видит ссылку Admin / Support на app home;

не может открыть admin endpoints;

получает 403 Forbidden при обращении к /api/v1/admin/\*;

не имеет доступа к raw AI responses;

не имеет доступа к trace payloads;

не имеет доступа к billing state других клиентов;

не имеет доступа к support actions.



Важно: скрытие ссылки во frontend — это только UX. Реальная защита находится на backend через auth middleware + admin middleware.



Что нельзя делать во frontend



Frontend не должен:



хранить admin-флаг в localStorage;

принимать is\_admin из query/body/header;

хранить или показывать ADMIN\_EMAILS;

содержать hardcoded admin emails;

выполнять admin actions без backend-проверки;

обходить /api/v1/admin/me;

показывать admin link всем пользователям.

Admin middleware



Все admin routes должны быть защищены двумя слоями:



auth middleware -> admin middleware -> admin handler



Это означает:



сначала проверяется обычная аутентификация;

затем проверяется admin-доступ;

только после этого выполняется admin handler.



Не допускается регистрация /api/v1/admin/\* endpoint без admin middleware.



Seller scope



Все admin endpoints должны работать с явным seller scope.



Для client-specific endpoints seller account передаётся в path:



/api/v1/admin/clients/{seller\_account\_id}/...



Backend обязан использовать этот seller\_account\_id как target object и фильтровать все данные по нему.



Примеры seller-scoped endpoints:



GET /api/v1/admin/clients/{seller\_account\_id}

GET /api/v1/admin/clients/{seller\_account\_id}/sync-jobs

GET /api/v1/admin/clients/{seller\_account\_id}/ai/chat-traces

GET /api/v1/admin/clients/{seller\_account\_id}/billing



Admin endpoint не должен случайно возвращать данные другого seller account.



Что support может видеть



Support/admin может видеть данные, необходимые для диагностики:



Client / account data

seller account id;

seller name;

owner email;

seller status;

created/updated timestamps;

billing state;

connection status.

Integration diagnostics

Ozon connection status;

last connection check;

last connection error;

sync jobs;

import jobs;

import errors;

sync cursors.

Alerts diagnostics

alert runs;

open alerts count;

alert status;

alert severity/urgency;

alert error messages.

Recommendations diagnostics

recommendation runs;

recommendation statuses;

AI model;

prompt version;

token usage;

estimated cost;

raw OpenAI response;

validation payload;

rejected items payload;

related recommendations;

recommendation feedback.

AI Chat diagnostics

chat sessions;

chat messages;

chat traces;

detected intent;

planner model;

answer model;

prompt versions;

tool plan;

validated tool plan;

tool results;

fact context;

raw planner response;

raw answer response;

answer validation payload;

token usage;

estimated cost;

chat feedback.

Audit data

admin action logs;

action type;

admin email;

target seller account;

request payload;

result payload;

action status;

error message;

timestamps.

Что нельзя показывать support/admin



Даже в админке нельзя показывать:



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

refresh tokens;

access tokens;

admin allowlist;

raw secrets from payloads;

private system credentials.



Если такие данные случайно попали в payload, они должны быть замаскированы до сохранения или до отображения.



Что нельзя показывать обычному пользователю



Обычному seller user нельзя показывать:



raw planner response;

raw answer response;

raw recommendation AI response;

tool plan;

validated tool plan;

tool results;

fact context;

validation payloads;

token usage;

estimated cost;

admin action logs;

billing internal notes;

support diagnostics;

raw provider errors;

admin-only feedback views.



Эти данные доступны только через admin API.



Raw AI responses



Raw AI responses являются внутренними support-данными.



Они нужны для диагностики:



невалидного JSON;

неверного planner output;

неверного tool plan;

hallucination в answer;

validation failure;

rejected recommendations;

timeout/rate limit/provider errors.



Raw AI responses доступны только через admin endpoints.



Примеры:



GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendation-runs/{run\_id}

GET /api/v1/admin/clients/{seller\_account\_id}/ai/recommendations/{id}

GET /api/v1/admin/clients/{seller\_account\_id}/ai/chat-traces/{trace\_id}

Отображение raw AI responses в UI



Raw blocks должны отображаться безопасно:



collapsed by default;

read-only;

без JSON editor;

без copy API key button;

с предупреждением:

Internal support data. Do not share externally.



UI не должен раскрывать raw payload автоматически при открытии страницы.



Защита secrets в trace и AI payloads



AI Chat traces должны сохраняться через sanitization слой.



Перед записью в trace должны маскироваться:



API keys;

bearer tokens;

authorization headers;

cookies;

passwords;

secrets;

raw payload markers;

sensitive Ozon credentials.



Пример безопасного значения:



Bearer \[REDACTED]

\[REDACTED\_OPENAI\_KEY]



Если payload содержит подозрительные поля, они не должны отображаться как есть.



OpenAI API key



OpenAI API key:



используется только backend-слоем;

не передаётся во frontend;

не сохраняется в trace;

не возвращается из admin API;

не отображается в raw AI response;

не логируется в admin action result.



Frontend не должен знать OpenAI API key.



Ozon credentials



Ozon credentials:



не должны отображаться в admin UI;

не должны возвращаться из admin API;

не должны попадать в action payloads;

не должны попадать в raw diagnostic JSON.



Admin может видеть только connection status и error summary, но не сами credentials.



Audit log



Все ручные support actions должны фиксироваться в admin\_action\_logs.



Audit log отвечает на вопрос:



Какие действия support выполнил вручную?



Audit log должен сохранять:



admin user id;

admin email;

seller account id;

action type;

target type;

target id;

request payload;

result payload;

status;

error message;

created\_at;

finished\_at.

Какие actions требуют audit log



Audit log обязателен для:



rerun sync;

reset cursor;

rerun metrics;

rerun alerts;

rerun recommendations;

update billing state;

view raw AI payload, если это будет выделено в отдельное action;

future destructive/support actions.

Lifecycle admin action



Каждый action должен проходить lifecycle:



running -> completed

running -> failed



Типовой flow:



1\. validate admin actor

2\. validate seller account target

3\. create admin\_action\_logs with status=running

4\. execute action

5\. complete action log on success

6\. fail action log on error

7\. return action summary to UI



Если action dependency не сконфигурирована, action должен:



создать audit log;

завершиться со статусом failed;

вернуть понятную ошибку.

Какие actions считаются опасными



Опасными или чувствительными считаются:



Reset cursor



Может привести к повторному импорту данных.



Риск:



следующий sync может переимпортировать данные



UI обязан показывать warning и требовать confirmation.



Rerun recommendations



Может вызвать OpenAI API.



Риск:



появится token usage и estimated cost



UI обязан показывать warning и требовать confirmation.



Rerun sync



Может создать новый sync job и нагрузку на API/worker.



UI обязан требовать confirmation.



Rerun metrics



Может пересчитать агрегаты за период.



UI обязан требовать confirmation.



Rerun alerts



Может создать новый alert run и изменить состояние alert diagnostics.



UI обязан требовать confirmation.



Update billing state



Меняет support-visible billing state клиента.



Должен быть audit-logged.



Что нельзя выполнять без confirmation



Без confirmation нельзя выполнять:



reset cursor;

rerun sync;

rerun metrics;

rerun alerts;

rerun recommendations;

update billing state;

future destructive actions.



Confirmation должен быть явным. Например:



Reset this sync cursor? The next sync may re-import data for this domain.



или:



Rerun AI recommendations? This can call OpenAI API and create additional token usage/cost.

Что админка не должна делать



Админка не должна выполнять business auto-actions за клиента.



Запрещено без отдельного будущего explicit action:



менять цену товара в Ozon;

менять рекламный бюджет;

останавливать рекламную кампанию;

создавать поставки;

изменять остатки;

принимать recommendation за клиента;

закрывать alert за клиента;

менять Ozon credentials;

автоматически исправлять данные.



Stage 11 actions являются support operational actions, а не business actions.



Разница между support action и business action



Support action:



перезапустить sync

сбросить cursor

пересчитать metrics

перезапустить alerts

перезапустить recommendations

обновить billing state



Business action:



изменить цену

остановить рекламу

создать поставку

изменить карточку товара



Stage 11 реализует только support actions.



Request payload safety



Request payload в admin\_action\_logs должен содержать только безопасные operational параметры:



sync\_type;

domain;

cursor\_type;

cursor\_value;

date\_from;

date\_to;

as\_of\_date;

billing state fields.



Request payload не должен содержать:



API keys;

tokens;

cookies;

passwords;

encrypted credentials;

raw auth headers.

Result payload safety



Result payload должен содержать только безопасный summary:



created sync job id;

alert run id;

recommendation run id;

status;

counts;

error summary;

updated billing state summary.



Result payload не должен содержать:



secrets;

raw credentials;

provider auth headers;

unmasked API keys;

session tokens.

Feedback data



Support может видеть feedback по AI:



Chat feedback

rating;

comment;

related message;

session;

trace id, если найден;

created\_at.

Recommendation feedback

rating;

comment;

related recommendation;

created\_at.



Feedback endpoints не должны возвращать raw AI payloads. Для raw diagnostics используются отдельные AI logs endpoints.



Billing state security



Billing state в Stage 11 — это MVP support-visible state.



Support может видеть:



plan\_code;

status;

trial dates;

current period;

AI token limit;

AI tokens used;

estimated AI cost;

notes;

updated\_at.



Billing state не является полноценным billing engine.



Админка не должна показывать:



payment method;

card data;

invoices;

external payment provider secrets;

billing provider tokens.

Admin frontend requirements



Admin frontend должен:



проверять /api/v1/admin/me;

скрывать admin navigation для non-admin users;

не хранить admin flag в localStorage;

не показывать raw payloads expanded by default;

использовать confirmation для risky actions;

показывать action log result после actions;

не отображать secrets;

не содержать raw JSON editor.

Error handling



Admin API должен возвращать безопасные ошибки.



Допустимо:



{

&#x20; "error": "failed to list chat traces"

}



Недопустимо:



{

&#x20; "error": "postgres://user:password@host/db..."

}



или:



{

&#x20; "error": "Authorization: Bearer ..."

}



Ошибки должны помогать support’у, но не раскрывать credentials.



Admin UI не является security boundary



Даже если frontend скрывает ссылку /app/admin, пользователь может вручную открыть URL.



Поэтому:



backend обязан защищать /api/v1/admin/\*;

frontend checks нужны только для UX;

все sensitive данные должны быть защищены backend middleware.

Validation checklist



Перед закрытием Stage 11 нужно проверить:



обычный user не видит admin link;

обычный user получает 403 на /api/v1/admin/\*;

admin видит /app/admin;

admin видит client list;

admin видит client detail;

raw AI blocks collapsed by default;

raw AI blocks помечены как internal support data;

reset cursor требует confirmation;

rerun recommendations требует confirmation;

update billing state audit-логируется;

actions пишут admin\_action\_logs;

secrets не отображаются;

seller scope соблюдается;

feedback виден support’у;

billing state виден support’у;

build/tests проходят.

Итоговое правило



Admin / Support Tooling должен помогать support’у диагностировать проблемы, но не должен становиться источником новых security-рисков.



Кратко:



Support can diagnose and safely rerun operational processes.

Support cannot see secrets.

Support cannot perform business actions without explicit future tooling.

Every manual support action is audit-logged.

