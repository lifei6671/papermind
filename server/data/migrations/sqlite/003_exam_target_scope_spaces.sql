-- exam_target_scope_spaces：考试发布目标作用空间表，保存用户直投等目标的空间归属。
CREATE TABLE IF NOT EXISTS exam_target_scope_spaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    exam_target_id INTEGER NOT NULL,
    space_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);

-- 同一投放目标不能重复映射到同一空间。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_target_scope_spaces_target_space
    ON exam_target_scope_spaces (tenant_id, exam_target_id, space_id);

-- 成绩、阅卷、候选人列表按考试和空间范围过滤时使用。
CREATE INDEX IF NOT EXISTS idx_exam_target_scope_spaces_exam_space
    ON exam_target_scope_spaces (tenant_id, exam_id, space_id);

-- 既有空间目标的作用空间就是目标空间本身。
INSERT OR IGNORE INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT tenant_id, exam_id, id, target_id, created_at, created_by, created_by_type, '{}'
FROM exam_targets
WHERE target_type = 'space';

-- 既有用户直投目标从 ext_json.space_ids 回填作用空间；无 space_ids 的历史目标保留运行时回退语义。
INSERT OR IGNORE INTO exam_target_scope_spaces (
    tenant_id, exam_id, exam_target_id, space_id, created_at, created_by, created_by_type, ext_json
)
SELECT targets.tenant_id,
       targets.exam_id,
       targets.id,
       CAST(scoped_space.value AS INTEGER),
       targets.created_at,
       targets.created_by,
       targets.created_by_type,
       '{}'
FROM exam_targets AS targets,
     json_each(targets.ext_json, '$.space_ids') AS scoped_space
WHERE targets.target_type = 'user';
