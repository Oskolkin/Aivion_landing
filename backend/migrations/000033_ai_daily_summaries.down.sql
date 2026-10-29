ALTER TABLE ai_daily_summary_runs DROP CONSTRAINT IF EXISTS fk_ai_daily_summary_runs_account_summary;

DROP TABLE IF EXISTS ai_daily_summaries;
DROP TABLE IF EXISTS ai_daily_summary_runs;

DROP TABLE IF EXISTS description_categories;
