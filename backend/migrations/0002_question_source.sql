ALTER TABLE questions
ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'seed';

CREATE INDEX IF NOT EXISTS idx_questions_role_topic_source
ON questions (role_id, LOWER(TRIM(topic)), source);
