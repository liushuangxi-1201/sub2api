-- Explicitly published cache probes. Snapshots survive usage-log cleanup.
CREATE TABLE IF NOT EXISTS group_cache_showcase_runs (
    id BIGSERIAL PRIMARY KEY,
    run_id VARCHAR(80) NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model VARCHAR(200) NOT NULL,
    protocol VARCHAR(16) NOT NULL CHECK (protocol IN ('responses', 'chat', 'messages')),
    tested_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (created_by, run_id)
);
CREATE INDEX IF NOT EXISTS idx_group_cache_showcase_group_time
    ON group_cache_showcase_runs (group_id, tested_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS group_cache_showcase_rounds (
    run_id BIGINT NOT NULL REFERENCES group_cache_showcase_runs(id) ON DELETE CASCADE,
    round_index SMALLINT NOT NULL CHECK (round_index BETWEEN 1 AND 6),
    -- Deliberately no FK: clearing usage history must not erase published evidence.
    usage_log_id BIGINT NOT NULL UNIQUE,
    input_tokens BIGINT NOT NULL CHECK (input_tokens > 0),
    cache_read_tokens BIGINT NOT NULL CHECK (cache_read_tokens >= 0 AND cache_read_tokens <= input_tokens),
    first_token_ms BIGINT CHECK (first_token_ms >= 0),
    measured_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, round_index)
);
