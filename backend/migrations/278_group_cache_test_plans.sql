-- Scheduled group cache experiments, using the same group scheduler as Pelican.
CREATE TABLE IF NOT EXISTS group_cache_test_plans (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id VARCHAR(100) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    config JSONB NOT NULL,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    running_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_group_cache_test_due ON group_cache_test_plans (next_run_at) WHERE enabled;
CREATE TABLE IF NOT EXISTS group_cache_test_results (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL REFERENCES group_cache_test_plans(id) ON DELETE CASCADE,
    result JSONB NOT NULL,
    publication_id BIGINT,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_group_cache_test_history ON group_cache_test_results (plan_id, id DESC);
-- Costs remain available after trimming result history.
CREATE TABLE IF NOT EXISTS group_cache_test_daily_costs (
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    cost_date DATE NOT NULL,
    cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    unpriced_count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, cost_date)
);
ALTER TABLE group_cache_showcase_runs ADD COLUMN IF NOT EXISTS plan_id BIGINT REFERENCES group_cache_test_plans(id) ON DELETE CASCADE;
ALTER TABLE group_cache_showcase_runs ADD COLUMN IF NOT EXISTS source VARCHAR(16) NOT NULL DEFAULT 'billing';
ALTER TABLE group_cache_showcase_rounds ALTER COLUMN usage_log_id DROP NOT NULL;
