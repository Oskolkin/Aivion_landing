CREATE TABLE IF NOT EXISTS ai_pipeline_dispatches (
    id BIGSERIAL PRIMARY KEY,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    sync_job_id BIGINT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE,
    metric_date DATE NULL,
    stage TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    task_id TEXT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    recalculation_completed_at TIMESTAMPTZ NULL,
    enqueued_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_ai_pipeline_dispatches_sync_stage UNIQUE (sync_job_id, stage)
);

CREATE INDEX IF NOT EXISTS idx_ai_pipeline_dispatches_status
    ON ai_pipeline_dispatches (status, updated_at DESC);
