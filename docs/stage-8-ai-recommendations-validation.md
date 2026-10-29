\# Stage 8. AI Recommendations — validation scenarios



\## Назначение документа



Этот документ фиксирует сценарии проверки stage 8: \*\*AI Recommendation Engine\*\*.



Цель проверки — убедиться, что система:



\- собирает структурированный context для ChatGPT;

\- вызывает ChatGPT через OpenAI API;

\- получает структурированный JSON-ответ;

\- валидирует AI-output до сохранения;

\- сохраняет только безопасные и корректные рекомендации;

\- связывает рекомендации с alert’ами;

\- не создаёт дубли при повторной генерации;

\- показывает рекомендации через API, UI и dashboard teaser;

\- корректно обрабатывает статусы `accepted`, `dismissed`, `resolved`.



Stage 8 считается закрытым, если AI Recommendation Engine работает как управляемый backend-процесс:



```text

metrics / alerts / constraints

&#x20; -> AI context

&#x20; -> ChatGPT via OpenAI API

&#x20; -> backend validation

&#x20; -> stored recommendations

&#x20; -> Recommendations UI / Dashboard

