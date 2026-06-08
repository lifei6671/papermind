CREATE TABLE IF NOT EXISTS exam_target_scope_spaces (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    exam_target_id BIGINT NOT NULL,
    space_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_target_scope_spaces IS '考试发布目标作用空间表，保存用户直投等目标的空间归属';
COMMENT ON COLUMN exam_target_scope_spaces.id IS '考试发布目标作用空间主键 ID';
COMMENT ON COLUMN exam_target_scope_spaces.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_target_scope_spaces.exam_id IS '考试 ID';
COMMENT ON COLUMN exam_target_scope_spaces.exam_target_id IS '考试发布目标 ID';
COMMENT ON COLUMN exam_target_scope_spaces.space_id IS '目标作用空间 ID';
COMMENT ON COLUMN exam_target_scope_spaces.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_target_scope_spaces.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_target_scope_spaces.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_target_scope_spaces.ext_json IS 'JSON 扩展字段，保存非主流程元数据';

CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_target_scope_spaces_target_space
    ON exam_target_scope_spaces (tenant_id, exam_target_id, space_id);

CREATE INDEX IF NOT EXISTS idx_exam_target_scope_spaces_exam_space
    ON exam_target_scope_spaces (tenant_id, exam_id, space_id);

INSERT INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT tenant_id, exam_id, id, target_id, created_at, created_by, created_by_type, '{}'::jsonb
FROM exam_targets
WHERE target_type = 'space'
ON CONFLICT (tenant_id, exam_target_id, space_id) DO NOTHING;

INSERT INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT targets.tenant_id,
       targets.exam_id,
       targets.id,
       scoped_space.value::bigint,
       targets.created_at,
       targets.created_by,
       targets.created_by_type,
       '{}'::jsonb
FROM exam_targets AS targets
JOIN LATERAL jsonb_array_elements_text(targets.ext_json->'space_ids') AS scoped_space(value) ON TRUE
WHERE targets.target_type = 'user'
ON CONFLICT (tenant_id, exam_target_id, space_id) DO NOTHING;
