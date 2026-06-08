CREATE TABLE IF NOT EXISTS exam_target_scope_spaces (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '考试发布目标作用空间主键 ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户 ID',
    exam_id BIGINT UNSIGNED NOT NULL COMMENT '考试 ID',
    exam_target_id BIGINT UNSIGNED NOT NULL COMMENT '考试发布目标 ID',
    space_id BIGINT UNSIGNED NOT NULL COMMENT '目标作用空间 ID',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人主体 ID',
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人主体类型：platform_user / tenant_user / system',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，保存非主流程元数据',
    PRIMARY KEY (id),
    UNIQUE KEY uk_exam_target_scope_spaces_target_space (tenant_id, exam_target_id, space_id),
    KEY idx_exam_target_scope_spaces_exam_space (tenant_id, exam_id, space_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='考试发布目标作用空间表，保存用户直投等目标的空间归属';

INSERT IGNORE INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT tenant_id, exam_id, id, target_id, created_at, created_by, created_by_type, JSON_OBJECT()
FROM exam_targets
WHERE target_type = 'space';

INSERT IGNORE INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT targets.tenant_id,
       targets.exam_id,
       targets.id,
       scoped_space.space_id,
       targets.created_at,
       targets.created_by,
       targets.created_by_type,
       JSON_OBJECT()
FROM exam_targets AS targets
JOIN JSON_TABLE(
    targets.ext_json,
    '$.space_ids[*]' COLUMNS (space_id BIGINT UNSIGNED PATH '$')
) AS scoped_space ON TRUE
WHERE targets.target_type = 'user';
