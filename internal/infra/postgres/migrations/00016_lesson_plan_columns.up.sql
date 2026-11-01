ALTER TABLE lessons
    ADD COLUMN IF NOT EXISTS syllabus_row_index INT,
    ADD COLUMN IF NOT EXISTS lesson_plan_key TEXT,
    ADD COLUMN IF NOT EXISTS lesson_plan_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (lesson_plan_status IN ('pending', 'generating', 'ready', 'failed')),
    ADD COLUMN IF NOT EXISTS lesson_plan_error TEXT;
