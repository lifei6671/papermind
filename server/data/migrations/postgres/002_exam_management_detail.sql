CREATE TABLE IF NOT EXISTS exam_operation_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    operation_type VARCHAR(64) NOT NULL,
    operation_title VARCHAR(128) NOT NULL,
    operation_detail TEXT NOT NULL DEFAULT '',
    actor_id BIGINT NOT NULL DEFAULT 0,
    actor_type VARCHAR(32) NOT NULL DEFAULT 'system',
    actor_role VARCHAR(32) NOT NULL DEFAULT '',
    space_id BIGINT,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_operation_logs IS '考试管理端操作日志表，记录发布、导入考生、导出成绩、发布成绩、设置变更和阅卷等审计事件';
COMMENT ON COLUMN exam_operation_logs.id IS '管理端操作日志主键 ID';
COMMENT ON COLUMN exam_operation_logs.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_operation_logs.exam_id IS '考试 ID';
COMMENT ON COLUMN exam_operation_logs.operation_type IS '操作类型，例如 publish_exam / send_invite';
COMMENT ON COLUMN exam_operation_logs.operation_title IS '操作标题，用于操作日志列表展示';
COMMENT ON COLUMN exam_operation_logs.operation_detail IS '操作详情摘要，避免前端拼接审计文案';
COMMENT ON COLUMN exam_operation_logs.actor_id IS '操作人用户 ID，由后端 session 派生';
COMMENT ON COLUMN exam_operation_logs.actor_type IS '操作人主体类型，例如 tenant_user / system';
COMMENT ON COLUMN exam_operation_logs.actor_role IS '操作时的租户级角色快照';
COMMENT ON COLUMN exam_operation_logs.space_id IS '操作关联空间，租户级操作为空';
COMMENT ON COLUMN exam_operation_logs.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_operation_logs.created_by IS '创建人主体 ID，通常与 actor_id 一致';
COMMENT ON COLUMN exam_operation_logs.created_by_type IS '创建人主体类型，通常与 actor_type 一致';
COMMENT ON COLUMN exam_operation_logs.ext_json IS 'JSON 扩展字段，用于保存 operation_group_id 等元数据';

CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_status_submitted
    ON exam_attempts (tenant_id, exam_id, status, submitted_at);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_user
    ON exam_attempts (tenant_id, exam_id, user_id);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_id
    ON exam_attempts (tenant_id, exam_id, id);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam_score_rank
    ON exam_attempts (tenant_id, exam_id, total_score, submitted_at, id);
CREATE INDEX IF NOT EXISTS idx_exam_answers_attempt_grading
    ON exam_answers (tenant_id, attempt_id, grading_status);
CREATE INDEX IF NOT EXISTS idx_exam_operation_logs_exam_time
    ON exam_operation_logs (tenant_id, exam_id, created_at, id);
