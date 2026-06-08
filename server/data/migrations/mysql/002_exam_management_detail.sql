CREATE TABLE IF NOT EXISTS exam_operation_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '管理端操作日志主键 ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户 ID',
    exam_id BIGINT UNSIGNED NOT NULL COMMENT '考试 ID',
    operation_type VARCHAR(64) NOT NULL COMMENT '操作类型，例如 publish_exam / send_invite',
    operation_title VARCHAR(128) NOT NULL COMMENT '操作标题，用于操作日志列表展示',
    operation_detail TEXT NOT NULL COMMENT '操作详情摘要，避免前端拼接审计文案',
    actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作人用户 ID，由后端 session 派生',
    actor_type VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '操作人主体类型，例如 tenant_user / system',
    actor_role VARCHAR(32) NOT NULL DEFAULT '' COMMENT '操作时的租户级角色快照',
    space_id BIGINT UNSIGNED COMMENT '操作关联空间，租户级操作为空',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人主体 ID，通常与 actor_id 一致',
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人主体类型，通常与 actor_type 一致',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，用于保存 operation_group_id 等元数据',
    PRIMARY KEY (id),
    KEY idx_exam_operation_logs_exam_time (tenant_id, exam_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='考试管理端操作日志表，记录发布、导入考生、导出成绩、发布成绩、设置变更和阅卷等审计事件';

SET @papermind_ddl = (
    SELECT IF(
        EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name = 'exam_attempts'
              AND index_name = 'idx_exam_attempts_exam_status_submitted'
        ),
        'SELECT 1',
        'CREATE INDEX idx_exam_attempts_exam_status_submitted ON exam_attempts (tenant_id, exam_id, status, submitted_at)'
    )
);
PREPARE papermind_stmt FROM @papermind_ddl;
EXECUTE papermind_stmt;
DEALLOCATE PREPARE papermind_stmt;

SET @papermind_ddl = (
    SELECT IF(
        EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name = 'exam_attempts'
              AND index_name = 'idx_exam_attempts_exam_user'
        ),
        'SELECT 1',
        'CREATE INDEX idx_exam_attempts_exam_user ON exam_attempts (tenant_id, exam_id, user_id)'
    )
);
PREPARE papermind_stmt FROM @papermind_ddl;
EXECUTE papermind_stmt;
DEALLOCATE PREPARE papermind_stmt;

SET @papermind_ddl = (
    SELECT IF(
        EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name = 'exam_attempts'
              AND index_name = 'idx_exam_attempts_exam_id'
        ),
        'SELECT 1',
        'CREATE INDEX idx_exam_attempts_exam_id ON exam_attempts (tenant_id, exam_id, id)'
    )
);
PREPARE papermind_stmt FROM @papermind_ddl;
EXECUTE papermind_stmt;
DEALLOCATE PREPARE papermind_stmt;

SET @papermind_ddl = (
    SELECT IF(
        EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name = 'exam_attempts'
              AND index_name = 'idx_exam_attempts_exam_score_rank'
        ),
        'SELECT 1',
        'CREATE INDEX idx_exam_attempts_exam_score_rank ON exam_attempts (tenant_id, exam_id, total_score, submitted_at, id)'
    )
);
PREPARE papermind_stmt FROM @papermind_ddl;
EXECUTE papermind_stmt;
DEALLOCATE PREPARE papermind_stmt;

SET @papermind_ddl = (
    SELECT IF(
        EXISTS (
            SELECT 1 FROM information_schema.statistics
            WHERE table_schema = DATABASE()
              AND table_name = 'exam_answers'
              AND index_name = 'idx_exam_answers_attempt_grading'
        ),
        'SELECT 1',
        'CREATE INDEX idx_exam_answers_attempt_grading ON exam_answers (tenant_id, attempt_id, grading_status)'
    )
);
PREPARE papermind_stmt FROM @papermind_ddl;
EXECUTE papermind_stmt;
DEALLOCATE PREPARE papermind_stmt;
