-- exam_operation_logs：管理端考试操作日志表，记录发布、导入考生、导出成绩、发布成绩、设置变更和阅卷等审计事件。
CREATE TABLE IF NOT EXISTS exam_operation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    operation_type TEXT NOT NULL,
    operation_title TEXT NOT NULL,
    operation_detail TEXT NOT NULL DEFAULT '',
    actor_id INTEGER NOT NULL DEFAULT 0,
    actor_type TEXT NOT NULL DEFAULT 'system',
    actor_role TEXT NOT NULL DEFAULT '',
    space_id INTEGER,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);

-- 作答状态、提交时间、成绩列表和进度聚合。
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_status_submitted
    ON exam_attempts (tenant_id, exam_id, status, submitted_at);

-- 单个考生作答查询和考生管理列表关联。
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_user
    ON exam_attempts (tenant_id, exam_id, user_id);

-- 详情聚合按考试获取作答 ID 列表。
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_id
    ON exam_attempts (tenant_id, exam_id, id);

-- 成绩列表和排名稳定排序。
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_score_rank
    ON exam_attempts (tenant_id, exam_id, total_score, submitted_at, id);

-- 待阅卷数量和主观题状态聚合。
CREATE INDEX IF NOT EXISTS idx_exam_answers_attempt_grading
    ON exam_answers (tenant_id, attempt_id, grading_status);

-- 操作日志按考试时间倒序分页。
CREATE INDEX IF NOT EXISTS idx_exam_operation_logs_exam_time
    ON exam_operation_logs (tenant_id, exam_id, created_at, id);
