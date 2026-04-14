CREATE TABLE files (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id      UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    file_name      TEXT NOT NULL,
    minio_key      TEXT NOT NULL,
    ingest_status  TEXT NOT NULL DEFAULT 'pending'
                   CHECK (ingest_status IN ('pending', 'processing', 'ready', 'failed')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_course ON files(course_id);
CREATE INDEX idx_files_status ON files(ingest_status);
