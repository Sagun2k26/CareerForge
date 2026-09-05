-- Schema for the AI Career Growth Platform. All migrations are idempotent.

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
    id                   UUID PRIMARY KEY,
    name                 TEXT NOT NULL UNIQUE,
    required_skills_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_jd            TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS resumes (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename    TEXT NOT NULL,
    file_url    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'uploaded',
    parsed_json JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_resumes_user ON resumes(user_id);

CREATE TABLE IF NOT EXISTS tailored_resumes (
    id              UUID PRIMARY KEY,
    resume_id       UUID NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_description TEXT NOT NULL,
    content_json    JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tailored_resume ON tailored_resumes(resume_id);

CREATE TABLE IF NOT EXISTS analyses (
    id                  UUID PRIMARY KEY,
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resume_id           UUID NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    role_id             UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    gap_score           INTEGER NOT NULL DEFAULT 0,
    matched_skills_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    missing_skills_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    role_fit_json       JSONB NOT NULL DEFAULT '{}'::jsonb,
    interview_prep_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    summary             TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_analyses_user ON analyses(user_id);
ALTER TABLE analyses ADD COLUMN IF NOT EXISTS role_fit_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE analyses ADD COLUMN IF NOT EXISTS interview_prep_json JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS roadmaps (
    id          UUID PRIMARY KEY,
    analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    items_json  JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roadmaps_analysis ON roadmaps(analysis_id);

CREATE TABLE IF NOT EXISTS progress (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    analysis_id UUID NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    item_order  INTEGER NOT NULL,
    status      TEXT NOT NULL DEFAULT 'todo',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, analysis_id, item_order)
);

CREATE TABLE IF NOT EXISTS questions (
    id           UUID PRIMARY KEY,
    role_id      UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    topic        TEXT NOT NULL DEFAULT '',
    prompt       TEXT NOT NULL,
    model_answer TEXT
);
CREATE INDEX IF NOT EXISTS idx_questions_role ON questions(role_id);

CREATE TABLE IF NOT EXISTS question_progress (
    id             UUID PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id    UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT '',
    last_practiced TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, question_id)
);

CREATE TABLE IF NOT EXISTS interview_sessions (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'active',
    transcript_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    scores_json     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_interviews_user ON interview_sessions(user_id);
