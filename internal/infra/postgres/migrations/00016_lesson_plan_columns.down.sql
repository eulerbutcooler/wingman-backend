ALTER TABLE lessons
    DROP COLUMN IF EXISTS lesson_plan_error,
    DROP COLUMN IF EXISTS lesson_plan_status,
    DROP COLUMN IF EXISTS lesson_plan_key,
    DROP COLUMN IF EXISTS syllabus_row_index;
