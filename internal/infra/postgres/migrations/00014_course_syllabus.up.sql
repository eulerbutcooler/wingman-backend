CREATE TABLE course_syllabus (
    course_id  UUID PRIMARY KEY REFERENCES courses(id) ON DELETE CASCADE,
    file_name  TEXT NOT NULL,
    minio_key  TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    topics     JSONB NOT NULL DEFAULT '[]'::jsonb,
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_course_syllabus_status ON course_syllabus(status);
