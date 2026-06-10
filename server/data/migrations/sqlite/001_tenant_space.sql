-- tenants：租户表，保存学校、企业、培训机构等租户主体。
CREATE TABLE IF NOT EXISTS tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    logo_url TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    tenant_code TEXT NOT NULL,
    allow_register INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 租户码用于注册归属，软删除后允许重新生成或复用。
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_tenant_code_deleted_at ON tenants (tenant_code, deleted_at);

-- spaces：空间表，保存班级、专业、课程、培训项目等租户内组织边界。
CREATE TABLE IF NOT EXISTS spaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    logo_url TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT 'custom',
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 空间列表高频按租户过滤。
CREATE INDEX IF NOT EXISTS idx_spaces_tenant_id ON spaces (tenant_id);

-- space_members：空间成员表，保存用户在空间内的角色和启用状态。
CREATE TABLE IF NOT EXISTS space_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    space_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role_in_space TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 同一用户在同一空间只能有一条未移除成员关系。
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_members_user_deleted_at ON space_members (tenant_id, space_id, user_id, deleted_at);
-- 权限判断高频按空间和状态过滤有效成员。
CREATE INDEX IF NOT EXISTS idx_space_members_space_status ON space_members (tenant_id, space_id, status, deleted_at);

-- space_configs：空间配置表，保存空间级配置项。
CREATE TABLE IF NOT EXISTS space_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    space_id INTEGER NOT NULL,
    config_key TEXT NOT NULL,
    config_value TEXT NOT NULL DEFAULT '',
    value_type TEXT NOT NULL DEFAULT 'string',
    description TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一空间内同一个配置键只能存在一条记录。
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_configs_key ON space_configs (tenant_id, space_id, config_key);

-- platform_users：平台管理员账号表，保存平台级管理员身份。
CREATE TABLE IF NOT EXISTS platform_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    last_login_ip TEXT NOT NULL DEFAULT '',
    last_login_at INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 平台管理员登录名在未删除账号内唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_username_deleted_at ON platform_users (username, deleted_at);
-- 平台管理员手机号在未删除账号内唯一，空字符串不参与唯一性约束。
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_phone_deleted_at ON platform_users (phone, deleted_at) WHERE phone <> '';
-- 平台管理员邮箱在未删除账号内唯一，空字符串不参与唯一性约束。
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_email_deleted_at ON platform_users (email, deleted_at) WHERE email <> '';

-- platform_configs：平台配置表，保存平台级开关、默认值和注册策略。
CREATE TABLE IF NOT EXISTS platform_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT NOT NULL,
    config_value TEXT NOT NULL DEFAULT '',
    value_type TEXT NOT NULL DEFAULT 'string',
    description TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 平台配置项按配置键全平台唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_configs_config_key ON platform_configs (config_key);

-- users：全局用户账号表，保存登录账号和用户基础资料。
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    real_name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    force_password_change INTEGER NOT NULL DEFAULT 0,
    last_login_ip TEXT NOT NULL DEFAULT '',
    last_login_at INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 登录名是全局账号，未删除账号内唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (username, deleted_at);
-- 手机号在未删除账号内唯一，空字符串不参与唯一性约束。
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (phone, deleted_at) WHERE phone <> '';
-- 邮箱在未删除账号内唯一，空字符串不参与唯一性约束。
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (email, deleted_at) WHERE email <> '';

-- tenant_user_memberships：租户用户关系表，保存全局用户在租户内的固定角色和启用状态。
CREATE TABLE IF NOT EXISTS tenant_user_memberships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一用户在同一租户只能拥有一条固定角色关系。
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id);

-- questions：题目表，保存租户公共题库和空间题库中的题目。
CREATE TABLE IF NOT EXISTS questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    space_id INTEGER,
    type TEXT NOT NULL,
    difficulty TEXT NOT NULL DEFAULT 'medium',
    title TEXT NOT NULL,
    analysis TEXT NOT NULL DEFAULT '',
    standard_answer TEXT NOT NULL DEFAULT '',
    reference_answer TEXT NOT NULL DEFAULT '',
    score_default NUMERIC NOT NULL DEFAULT 0,
    quality_score INTEGER NOT NULL DEFAULT 5,
    choice_display_count INTEGER,
    shuffle_options INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 题库筛选高频按租户、空间、题型、难度、状态和软删除过滤。
CREATE INDEX IF NOT EXISTS idx_questions_filter ON questions (tenant_id, space_id, type, difficulty, status, deleted_at);
-- 教师维护题目列表高频按创建人过滤。
CREATE INDEX IF NOT EXISTS idx_questions_creator ON questions (tenant_id, created_by, deleted_at);

-- question_options：题目选项表，保存选择题和判断题选项。
CREATE TABLE IF NOT EXISTS question_options (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question_id INTEGER NOT NULL,
    option_key TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    content TEXT NOT NULL,
    is_correct INTEGER NOT NULL DEFAULT 0,
    is_distractor INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一题目不能出现重复的编辑展示标签。
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_key ON question_options (tenant_id, question_id, option_key);
-- 同一题目不能出现重复的选项原始排序。
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_sort_order ON question_options (tenant_id, question_id, sort_order);
-- 查询题目详情时按题目加载选项。
CREATE INDEX IF NOT EXISTS idx_question_options_question ON question_options (tenant_id, question_id);

-- tags：标签表，保存知识点、章节、技能点等题目标签。
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 同一租户内标签名称唯一，软删除后允许重建同名标签。
CREATE UNIQUE INDEX IF NOT EXISTS uk_tags_name_deleted_at ON tags (tenant_id, name, deleted_at);
-- 标签列表高频按租户和软删除过滤。
CREATE INDEX IF NOT EXISTS idx_tags_tenant_deleted_at ON tags (tenant_id, deleted_at);

-- question_tags：题目标签关系表，保存题目和标签的绑定关系。
CREATE TABLE IF NOT EXISTS question_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一题目不能重复绑定同一标签。
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_tags_tag ON question_tags (tenant_id, question_id, tag_id);
-- 按标签查题时使用该索引。
CREATE INDEX IF NOT EXISTS idx_question_tags_tag ON question_tags (tenant_id, tag_id);

-- papers：试卷表，保存公共试卷和空间内试卷。
CREATE TABLE IF NOT EXISTS papers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    space_id INTEGER,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    duration_minutes INTEGER NOT NULL DEFAULT 120,
    grade_level TEXT NOT NULL DEFAULT '',
    total_score NUMERIC NOT NULL DEFAULT 0,
    build_mode TEXT NOT NULL,
    shuffle_questions INTEGER NOT NULL DEFAULT 0,
    show_analysis INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 试卷列表高频按租户、空间、状态和软删除过滤。
CREATE INDEX IF NOT EXISTS idx_papers_filter ON papers (tenant_id, space_id, status, deleted_at);

-- paper_sections：试卷大题表，保存大题结构、题型和小计信息。
CREATE TABLE IF NOT EXISTS paper_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    paper_id INTEGER NOT NULL,
    sort_order INTEGER NOT NULL,
    name TEXT NOT NULL,
    question_type TEXT NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    total_score NUMERIC NOT NULL DEFAULT 0,
    question_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 同一试卷内大题排序稳定唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_sort_order ON paper_sections (tenant_id, paper_id, sort_order);
-- 支撑子表通过 section_id 和冗余 paper_id 建立复合外键。
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_id_paper ON paper_sections (tenant_id, id, paper_id);

-- paper_section_questions：大题题目关系表，保存手动或固化组卷后的题目。
CREATE TABLE IF NOT EXISTS paper_section_questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    section_id INTEGER NOT NULL,
    paper_id INTEGER NOT NULL,
    question_id INTEGER NOT NULL,
    sort_order INTEGER NOT NULL,
    score NUMERIC NOT NULL DEFAULT 0,
    shuffle_options INTEGER,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)
);
-- 同一题不能重复加入同一张固化试卷。
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_question ON paper_section_questions (tenant_id, paper_id, question_id);
-- 同一大题内题目排序稳定唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_sort_order ON paper_section_questions (tenant_id, section_id, sort_order);

-- paper_section_rules：大题抽题规则表，保存规则组卷条件。
CREATE TABLE IF NOT EXISTS paper_section_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    section_id INTEGER NOT NULL,
    paper_id INTEGER NOT NULL,
    sort_order INTEGER NOT NULL,
    difficulty TEXT,
    tag_filter TEXT NOT NULL DEFAULT '[]',
    question_count INTEGER NOT NULL,
    score_per_question NUMERIC NOT NULL DEFAULT 0,
    shuffle_options INTEGER,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)
);
-- 同一大题内抽题规则排序稳定唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_rules_sort_order ON paper_section_rules (tenant_id, section_id, sort_order);

-- exams：考试表，保存试卷发布后的考试安排。
CREATE TABLE IF NOT EXISTS exams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    paper_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER NOT NULL,
    duration_minutes INTEGER NOT NULL,
    max_attempts INTEGER NOT NULL DEFAULT 1,
    result_strategy TEXT NOT NULL DEFAULT 'latest',
    publish_mode TEXT NOT NULL DEFAULT 'manual_publish',
    score_publish_time INTEGER,
    invite_code TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}',
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 考试邀请码用于公开入口解析，必须全局唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exams_invite_code ON exams (invite_code);

-- exam_targets：考试发布范围表，保存考试面向的空间或用户。
CREATE TABLE IF NOT EXISTS exam_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一考试不能重复添加相同空间或用户目标。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_targets_target ON exam_targets (tenant_id, exam_id, target_type, target_id);

-- exam_live_question_pools：rule_live 发布态题池表，保存发布时冻结的候选题目。
CREATE TABLE IF NOT EXISTS exam_live_question_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    section_id INTEGER NOT NULL,
    rule_id INTEGER NOT NULL,
    question_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一规则候选题不能重复冻结。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_live_question_pools_question ON exam_live_question_pools (tenant_id, exam_id, section_id, rule_id, question_id);

-- exam_attempts：考生作答表，保存每次考试作答过程和成绩。
CREATE TABLE IF NOT EXISTS exam_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    attempt_no INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'in_progress',
    started_at INTEGER NOT NULL,
    submitted_at INTEGER,
    exam_token_hash TEXT NOT NULL,
    exam_token_expires_at INTEGER NOT NULL,
    objective_score NUMERIC NOT NULL DEFAULT 0,
    subjective_score NUMERIC NOT NULL DEFAULT 0,
    total_score NUMERIC NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一考生同一考试的作答次数编号唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_attempts_attempt_no ON exam_attempts (tenant_id, exam_id, user_id, attempt_no);
-- 考试过程鉴权高频按 token 哈希和作答状态查询。
CREATE INDEX IF NOT EXISTS idx_exam_attempts_token_status ON exam_attempts (exam_token_hash, status);

-- exam_attempt_questions：考生题目快照表，保存考生本次作答看到的题目快照。
CREATE TABLE IF NOT EXISTS exam_attempt_questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    attempt_id INTEGER NOT NULL,
    section_id INTEGER NOT NULL,
    question_id INTEGER NOT NULL,
    section_snapshot TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL,
    score NUMERIC NOT NULL DEFAULT 0,
    question_snapshot TEXT NOT NULL DEFAULT '{}',
    option_snapshot TEXT NOT NULL DEFAULT '[]',
    correct_answer_snapshot TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);

-- exam_answers：答案表，保存考生答案、得分和阅卷信息。
CREATE TABLE IF NOT EXISTS exam_answers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    attempt_id INTEGER NOT NULL,
    attempt_question_id INTEGER NOT NULL,
    answer_content TEXT NOT NULL DEFAULT '',
    score NUMERIC NOT NULL DEFAULT 0,
    grading_status TEXT NOT NULL DEFAULT 'pending',
    graded_by INTEGER NOT NULL DEFAULT 0,
    graded_at INTEGER,
    grader_comment TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一作答题目只能有一条答案记录。
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_answers_attempt_question ON exam_answers (tenant_id, attempt_id, attempt_question_id);

-- exam_events：考试事件表，追加记录切屏、自动保存、提交等考试过程事件。
CREATE TABLE IF NOT EXISTS exam_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    attempt_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    event_time INTEGER NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_by_type TEXT NOT NULL DEFAULT 'system',
    ext_json TEXT NOT NULL DEFAULT '{}'
);

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
    operation_group_id TEXT NOT NULL,
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

-- 操作日志按操作组聚合。
CREATE INDEX IF NOT EXISTS idx_exam_operation_logs_exam_group
    ON exam_operation_logs (tenant_id, exam_id, operation_group_id);

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
