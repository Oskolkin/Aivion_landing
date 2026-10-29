DROP TABLE IF EXISTS eval_baselines;
DROP TABLE IF EXISTS eval_human_reviews;
DROP TABLE IF EXISTS eval_grader_results;
DROP TABLE IF EXISTS eval_case_runs;
DROP TABLE IF EXISTS eval_runs;
DROP TABLE IF EXISTS eval_suite_cases;
DROP TABLE IF EXISTS eval_cases;
DROP TABLE IF EXISTS eval_suites;

ALTER TABLE chat_turns DROP COLUMN IF EXISTS telemetry_json;
