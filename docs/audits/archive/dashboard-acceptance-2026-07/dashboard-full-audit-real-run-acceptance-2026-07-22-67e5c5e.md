# Dashboard real-run acceptance §9 — 2026-07-22 (SHA 67e5c5e)

## Verdict

**acceptance failed** on `67e5c5e9e3f950cba8f7e23bc0eb12508f26f147`.

Dashboard is **not** fully complete. Evidence saved; **this commit was not amended**.

Backend больше не пытается определять смысл рекламных процентов по русским словам. Он проверяет, что каждое числовое значение подтверждено валидным factual или policy claim.

---

## Fix commit

| Field | Value |
| --- | --- |
| Parent SHA | `ce0c31f354ee8d161ea433dad605b38a468743c7` (unchanged) |
| Fix SHA | `67e5c5e9e3f950cba8f7e23bc0eb12508f26f147` |
| Subject | Simplify advertising percentage grounding to structured claims |
| Schema / prompt | unchanged (`aivion_unified_analysis_v10` / `unified_analyst_v1_26`) |

### Architectural simplification

| Before | After |
| --- | --- |
| Sentence-level % assignment by Russian cues | Value match only |
| Threshold peel via `выше` / `порог` / `превышает` regexes | `threshold_claims[].value` after policy validation |
| Dual structure+claim paths duplicating % errors | Single free-text grounding pass |
| Invalid claims still “backing” text % | Only **successfully validated** claim values ground text |

Central helper: `PercentMatchesClaim` (display rounding 0/1/2 decimals).

### Removed from blocking path

* `looksLikeThresholdPercentContext` / порог phrase regexes
* `assignAdvertisingSharePercents` local-semantics mapping
* Sentence “one percent / one metric” rules
* Bare «ДРР кабинета» linguistic block
* Ambiguous unlabeled multi-percent semantic block
* Non-percent порог prose linguistic matcher

### Still blocking (structured)

Unknown/mismatched fact_id, metric_code, period, pack value, denominator confusion, unknown/mismatched `threshold_claims` policy fields, free-text % with no matching **validated** claim value.

### Fixtures

PASS/FAIL coverage for runs **193–196** shapes in `advertising_percent_grounding_test.go` (+ updated run 190/193/194/195 acceptance tests).

### Regression

* `go test ./internal/unifiedanalysis/...` — **passed**
* Frontend lint (warnings only), unit tests, production build — **passed**

---

## §9 real runs on 67e5c5e

| Run | Status | Error |
| --- | --- | --- |
| 197 | `failed` | `provider_response_incomplete` |
| 199 | `failed` | `structured_output_missing` (empty structured output) |
| 200 | `failed` | `provider_response_incomplete` |

Prompt stamped: `unified_analyst_v1_26`. Pack hash unchanged (`17e80bbe…`).

**No run reached output validation** on this SHA, so the new claim-value grounding was not exercised against a live provider payload. Failures are provider/infrastructure, not advertising percent heuristics.

Artifacts: `docs/audits/real-run-2026-07-22-67e5c5e/`.

---

## Gates not reached

Output validation (live), bundle validation, publication, pointer switch, Dashboard/Actions API dumps, repeated GET fingerprint, Playwright, screenshots, rollback, restart/reconciliation — **not executed**.

Publication pointer unchanged from prior blocked runs.

---

## Remaining limitations

* Live §9 still blocked on OpenAI provider incompleteness / empty structured output for seller 1 / `2026-07-08`.
* Retries on the same SHA did not produce a material output to validate.
* Unit/regression coverage for the new grounding model is in-repo; real-run confirmation is pending a successful provider completion.

---

## Completion checklist

| Criterion | Met? |
| --- | --- |
| Separate fix commit on `ce0c31f` | **yes** (`67e5c5e…`) |
| Schema/prompt unchanged | **yes** |
| Validator semantic % heuristics removed from blocking | **yes** |
| Full BE/FE regression | **yes** |
| Real Unified provider completed with valid output | **no** |
| Publication / Playwright / rollback | **n/a** |

**Final status: acceptance failed on SHA `67e5c5e9e3f950cba8f7e23bc0eb12508f26f147` — stop here.**
