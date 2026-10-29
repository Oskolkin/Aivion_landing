# Unified Analysis provider root cause — 2026-07-22

Parent commit under investigation: `67e5c5e9e3f950cba8f7e23bc0eb12508f26f147`.

Real-run acceptance on that SHA failed before live output validation:

| Run | Error code | Error message (DB) | Provider response ID |
| --- | --- | --- | --- |
| 197 | `provider_response_incomplete` | `provider response incomplete` | `resp_09d00f9cf3c0adf7006a608f07884c81a1ac712819d739d9d3` |
| 199 | `structured_output_missing` | `structured output text is empty` | `resp_041f9eb82c822b14006a608f82f554819280407a67f815c516` |
| 200 | `provider_response_incomplete` | `provider response incomplete` | `resp_0e88f437ef5734ac006a608fd33d60819eb51f03176e284600` |

Prompt stamped: `unified_analyst_v1_26`. Schema target: `aivion_unified_analysis_v10`.

## Evidence refresh attempt

Tried to re-fetch the three OpenAI Responses via `backend/debug/_fetch_provider_resp` in this environment.

**Result:** `OPENAI_API_KEY` was not available in the agent environment (no `.env`, no process env). Live retrieve was **not** executed. Incomplete reasons and usage for these response IDs are therefore **not newly proven** from OpenAI.

Do not invent `incomplete_details.reason` for runs 197/200.

---

## Proven

These facts are established from repository code at `67e5c5e` and the DB/acceptance rows above:

1. Runs 197 and 200 failed with `provider_response_incomplete` and a generic message that does **not** include `incomplete_details.reason`.
2. Run 199 failed with `structured_output_missing` / empty structured output text — meaning the provider stage was treated as completed enough to reach Stage 4 extraction, then failed there.
3. `SaveProviderResponse` / `SaveProviderUsage` are only called from `completeSuccessfulPoll`. Terminal `incomplete` / `failed` / validation-fail paths call `markFailed` without persisting raw response, usage, or incomplete reason.
4. `markFailed` does not set `provider_status` (or tokens / CI counts), so terminal provider status can remain stale (e.g. `in_progress`).
5. Analysis `validateProviderCompletion` requires Code Interpreter (+ container match, refusal check) but **does not** require assistant `output_text` / structured output. Tool-only completed responses advance to `output_validating`.
6. `StartBackgroundResponse` discards OpenAI `x-request-id` returned by `doJSON`.
7. `provider_attempt_count` increments on `SaveResponseStarted`, but there is no automatic second provider attempt inside one logical run.
8. Analysis default `UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS` is **16000** (legacy single env). Docs already disagree with each other (provider doc vs analyst doc).
9. `HandlePoll` enqueues cleanup for any terminal poll result (`Completed` / `Failed` / `Cancelled`), including incomplete and tool-only failures.
10. Advertising validator / claim grounding were **not** reached on these runs; failures are provider orchestration, not narrative grounding.

---

## Likely

These are strongly indicated by code + prior run history, but not re-proven from live Response payloads for 197/199/200:

1. Runs 197/200 incomplete reason is probably `max_output_tokens` (historical incomplete runs and the 16000 analysis budget; prior outputs already approached that cap).
2. Run 199 is probably a completed Code-Interpreter-only response (assistant message count 0 / empty `output_text`) — the same class as runs 116/118 under earlier `tool_choice` settings, still possible with `tool_choice=auto`.
3. Without a bounded in-run retry, a single incomplete or tool-only terminal permanently fails the logical run even when a second attempt with the same pack/container would succeed.
4. Missing attempt artifacts make Inspector/acceptance unable to distinguish “OpenAI incomplete” from “local poll timeout” or to show usage at the incomplete terminal.

---

## Unknown because evidence was not persisted

For runs 197 / 199 / 200 specifically, the following were **not** recoverable from Aivion artifacts/DB at acceptance time:

* `incomplete_details.reason`
* Provider `error.code` / `error.message` (if any)
* `usage.input_tokens` / `output_tokens` / `total_tokens` on the failed terminal
* Assistant message count and `output_text` byte length
* `response.output` item type multiset
* Code Interpreter call count on the failed terminal (DB may retain only partial provider columns)
* OpenAI `x-request-id` for the creating POST and for retrieves
* Effective request summary fields: `max_output_tokens`, `tool_choice`, schema/prompt hashes as stamped on that attempt
* Whether the model stopped after CI without emitting structured output (199) vs truncated mid-generation (197/200)

Live OpenAI retrieve of the three `resp_*` IDs was not possible in this session without an API key; treat any later retrieve as optional corroboration, not a substitute for built-in persistence.

---

## Fix direction (this commit)

Stabilize provider orchestration without changing schema `aivion_unified_analysis_v10` or prompt `unified_analyst_v1_26`:

* Persist every terminal provider response (including incomplete) before mutating run status.
* Persist terminal provider status + reason-bearing error message + usage.
* Capture `x-request-id`.
* Expand request summary with effective runtime parameters.
* Raise analysis max output to **32768** (GPT-4.1 cap); keep smoke separate.
* Bounded in-run retry (`UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS=2`) for `incomplete/max_output_tokens` and completed tool-only / empty structured output.
* Gate structured-output presence at provider completion (retry or fail) before `output_validating`.
* CAS-safe retry; cleanup only after logical terminal (success, exhausted retry, non-retryable error, cancel).
