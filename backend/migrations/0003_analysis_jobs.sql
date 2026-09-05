CREATE TABLE IF NOT EXISTS analysis_jobs (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resume_id   UUID NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    analysis_id UUID REFERENCES analyses(id) ON DELETE SET NULL,
    error       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analysis_jobs_queue
    ON analysis_jobs (status, created_at);

CREATE INDEX IF NOT EXISTS idx_analysis_jobs_user
    ON analysis_jobs (user_id, created_at DESC);
