-- name: CreateLesson :one
INSERT INTO lessons (course_id, title, order_idx)
VALUES ($1, $2, $3) RETURNING *;

-- name: CreateSyllabusLesson :one
INSERT INTO lessons (course_id, title, order_idx, syllabus_row_index)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetLessonByID :one
SELECT * FROM lessons WHERE id = $1;

-- name: ListLessonsByCourse :many
SELECT * FROM lessons WHERE course_id = $1 AND order_idx >= 0
ORDER BY order_idx ASC;

-- name: ListAllLessonsByCourse :many
SELECT * FROM lessons WHERE course_id = $1
ORDER BY order_idx ASC;

-- name: GetMaterialsLesson :one
SELECT * FROM lessons
WHERE course_id = $1 AND order_idx = -1
ORDER BY created_at ASC
LIMIT 1;

-- name: ListLessonPlanBacklog :many
SELECT DISTINCT c.id AS course_id, c.instructor_id
FROM courses c
JOIN course_syllabus s ON s.course_id = c.id AND s.status = 'ready'
JOIN lessons l ON l.course_id = c.id AND l.order_idx >= 0
WHERE l.lesson_plan_status IN ('pending', 'generating');

-- name: UpdateLesson :one
UPDATE lessons SET title = $2, order_idx = $3, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateLessonKeywords :one
UPDATE lessons SET keywords = $2, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteLesson :exec
DELETE FROM lessons WHERE id = $1;

-- name: SetCourseLessonPlanStatus :exec
UPDATE lessons
SET lesson_plan_status = $2,
    lesson_plan_error = NULLIF($3, ''),
    updated_at = now()
WHERE course_id = $1;

-- name: UpdateLessonPlan :one
UPDATE lessons
SET lesson_plan_key = $2,
    lesson_plan_status = $3,
    lesson_plan_error = NULLIF($4, ''),
    updated_at = now()
WHERE id = $1
RETURNING *;
