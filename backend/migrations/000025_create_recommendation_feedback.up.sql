CREATE TABLE recommendation_feedback (
    id BIGSERIAL PRIMARY KEY,
    recommendation_id BIGINT NOT NULL REFERENCES recommendations(id) ON DELETE CASCADE,
    seller_account_id BIGINT NOT NULL REFERENCES seller_accounts(id) ON DELETE CASCADE,
    rating TEXT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_recommendation_feedback_rating CHECK (rating IN ('positive', 'negative', 'neutral')),
    CONSTRAINT uq_recommendation_feedback_seller_recommendation UNIQUE (seller_account_id, recommendation_id)
);

CREATE INDEX idx_recommendation_feedback_seller_created
    ON recommendation_feedback (seller_account_id, created_at DESC);

CREATE INDEX idx_recommendation_feedback_rating
    ON recommendation_feedback (seller_account_id, rating, created_at DESC);

CREATE INDEX idx_recommendation_feedback_recommendation
    ON recommendation_feedback (recommendation_id);
