ALTER TABLE course_syllabus
    ADD COLUMN IF NOT EXISTS subject_code TEXT,
    ADD COLUMN IF NOT EXISTS subject_name TEXT,
    ADD COLUMN IF NOT EXISTS rows JSONB NOT NULL DEFAULT '[]'::jsonb;
