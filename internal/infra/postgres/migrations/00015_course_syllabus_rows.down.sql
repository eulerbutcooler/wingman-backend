ALTER TABLE course_syllabus
    DROP COLUMN IF EXISTS rows,
    DROP COLUMN IF EXISTS subject_name,
    DROP COLUMN IF EXISTS subject_code;
