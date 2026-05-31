CREATE TABLE IF NOT EXISTS tenants (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    logo_url VARCHAR(512) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    tenant_code VARCHAR(32) NOT NULL,
    allow_register BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE tenants IS '租户表，保存学校、企业、培训机构等租户主体';
COMMENT ON COLUMN tenants.id IS '租户主键 ID';
COMMENT ON COLUMN tenants.name IS '租户名称，例如学校、企业、培训机构';
COMMENT ON COLUMN tenants.logo_url IS '企业或机构 Logo 地址';
COMMENT ON COLUMN tenants.description IS '企业或机构描述';
COMMENT ON COLUMN tenants.tenant_code IS '租户码，用于注册归属';
COMMENT ON COLUMN tenants.allow_register IS '是否允许该租户用户自注册';
COMMENT ON COLUMN tenants.status IS '租户状态：enabled / disabled';
COMMENT ON COLUMN tenants.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tenants.created_by IS '创建人主体 ID';
COMMENT ON COLUMN tenants.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tenants.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tenants.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN tenants.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tenants.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN tenants.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN tenants.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_tenant_code_deleted_at ON tenants (tenant_code, deleted_at);

CREATE TABLE IF NOT EXISTS spaces (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    logo_url VARCHAR(512) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(32) NOT NULL DEFAULT 'custom',
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE spaces IS '空间表，保存班级、专业、课程、培训项目等租户内组织边界';
COMMENT ON COLUMN spaces.id IS '空间主键 ID';
COMMENT ON COLUMN spaces.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN spaces.name IS '空间名称，例如班级、专业、课程、培训项目';
COMMENT ON COLUMN spaces.logo_url IS '空间 Logo 地址，可为空';
COMMENT ON COLUMN spaces.description IS '空间描述，可为空';
COMMENT ON COLUMN spaces.type IS '空间类型：class / major / course / training / custom';
COMMENT ON COLUMN spaces.status IS '空间状态：enabled / disabled';
COMMENT ON COLUMN spaces.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN spaces.created_by IS '创建人主体 ID';
COMMENT ON COLUMN spaces.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN spaces.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN spaces.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN spaces.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN spaces.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN spaces.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN spaces.deleted_at IS '软删除时间，0 表示未删除';
CREATE INDEX IF NOT EXISTS idx_spaces_tenant_id ON spaces (tenant_id);

CREATE TABLE IF NOT EXISTS space_members (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    space_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role_in_space VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE space_members IS '空间成员表，保存用户在空间内的角色和启用状态';
COMMENT ON COLUMN space_members.id IS '空间成员关系主键 ID';
COMMENT ON COLUMN space_members.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN space_members.space_id IS '空间 ID';
COMMENT ON COLUMN space_members.user_id IS '租户用户 ID';
COMMENT ON COLUMN space_members.role_in_space IS '空间内角色：space_admin / teacher / student';
COMMENT ON COLUMN space_members.status IS '空间成员状态：enabled / disabled';
COMMENT ON COLUMN space_members.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN space_members.created_by IS '创建人主体 ID';
COMMENT ON COLUMN space_members.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN space_members.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN space_members.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN space_members.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN space_members.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN space_members.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN space_members.deleted_at IS '软删除时间，0 表示未移除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_members_user_deleted_at ON space_members (tenant_id, space_id, user_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_space_members_space_status ON space_members (tenant_id, space_id, status, deleted_at);

CREATE TABLE IF NOT EXISTS space_configs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    space_id BIGINT NOT NULL,
    config_key VARCHAR(128) NOT NULL,
    config_value TEXT NOT NULL DEFAULT '',
    value_type VARCHAR(32) NOT NULL DEFAULT 'string',
    description VARCHAR(512) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE space_configs IS '空间配置表，保存空间级配置项';
COMMENT ON COLUMN space_configs.id IS '空间配置主键 ID';
COMMENT ON COLUMN space_configs.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN space_configs.space_id IS '空间 ID';
COMMENT ON COLUMN space_configs.config_key IS '配置键，例如 default_exam_duration';
COMMENT ON COLUMN space_configs.config_value IS '配置值，按字符串保存';
COMMENT ON COLUMN space_configs.value_type IS '配置值类型：string / number / bool / json';
COMMENT ON COLUMN space_configs.description IS '配置说明';
COMMENT ON COLUMN space_configs.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN space_configs.created_by IS '创建人主体 ID';
COMMENT ON COLUMN space_configs.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN space_configs.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN space_configs.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN space_configs.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN space_configs.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN space_configs.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_configs_key ON space_configs (tenant_id, space_id, config_key);

CREATE TABLE IF NOT EXISTS platform_users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    avatar_url VARCHAR(512) NOT NULL DEFAULT '',
    phone VARCHAR(32) NOT NULL DEFAULT '',
    email VARCHAR(128) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    last_login_ip VARCHAR(64) NOT NULL DEFAULT '',
    last_login_at BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE platform_users IS '平台管理员账号表，保存平台级管理员身份';
COMMENT ON COLUMN platform_users.id IS '平台管理员主键 ID';
COMMENT ON COLUMN platform_users.username IS '平台管理员登录名';
COMMENT ON COLUMN platform_users.avatar_url IS '用户头像地址';
COMMENT ON COLUMN platform_users.phone IS '手机号，可用于登录或找回账号';
COMMENT ON COLUMN platform_users.email IS '邮箱，可用于登录或通知';
COMMENT ON COLUMN platform_users.password_hash IS '密码哈希';
COMMENT ON COLUMN platform_users.last_login_ip IS '最后登录 IP';
COMMENT ON COLUMN platform_users.last_login_at IS '最后登录时间，Unix 毫秒时间戳';
COMMENT ON COLUMN platform_users.status IS '平台管理员状态：enabled / disabled';
COMMENT ON COLUMN platform_users.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN platform_users.created_by IS '创建人主体 ID';
COMMENT ON COLUMN platform_users.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN platform_users.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN platform_users.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN platform_users.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN platform_users.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN platform_users.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN platform_users.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_username_deleted_at ON platform_users (username, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_phone_deleted_at ON platform_users (phone, deleted_at) WHERE phone <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_email_deleted_at ON platform_users (email, deleted_at) WHERE email <> '';

CREATE TABLE IF NOT EXISTS platform_configs (
    id BIGSERIAL PRIMARY KEY,
    config_key VARCHAR(128) NOT NULL,
    config_value TEXT NOT NULL DEFAULT '',
    value_type VARCHAR(32) NOT NULL DEFAULT 'string',
    description VARCHAR(512) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE platform_configs IS '平台配置表，保存平台级开关、默认值和注册策略';
COMMENT ON COLUMN platform_configs.id IS '平台配置主键 ID';
COMMENT ON COLUMN platform_configs.config_key IS '配置键，例如 allow_register_default';
COMMENT ON COLUMN platform_configs.config_value IS '配置值，按字符串保存';
COMMENT ON COLUMN platform_configs.value_type IS '配置值类型：string / number / bool / json';
COMMENT ON COLUMN platform_configs.description IS '配置说明';
COMMENT ON COLUMN platform_configs.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN platform_configs.created_by IS '创建人主体 ID';
COMMENT ON COLUMN platform_configs.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN platform_configs.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN platform_configs.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN platform_configs.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN platform_configs.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN platform_configs.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_configs_config_key ON platform_configs (config_key);

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    real_name VARCHAR(128) NOT NULL DEFAULT '',
    avatar_url VARCHAR(512) NOT NULL DEFAULT '',
    phone VARCHAR(32) NOT NULL DEFAULT '',
    email VARCHAR(128) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    last_login_ip VARCHAR(64) NOT NULL DEFAULT '',
    last_login_at BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE users IS '全局用户账号表，保存登录账号和用户基础资料';
COMMENT ON COLUMN users.id IS '全局用户主键 ID';
COMMENT ON COLUMN users.username IS '全局登录名';
COMMENT ON COLUMN users.real_name IS '真实姓名，用于阅卷、成绩单和导出';
COMMENT ON COLUMN users.avatar_url IS '用户头像地址';
COMMENT ON COLUMN users.phone IS '手机号，可用于登录或通知';
COMMENT ON COLUMN users.email IS '邮箱，可用于登录或通知';
COMMENT ON COLUMN users.password_hash IS '密码哈希';
COMMENT ON COLUMN users.last_login_ip IS '最后登录 IP';
COMMENT ON COLUMN users.last_login_at IS '最后登录时间，Unix 毫秒时间戳';
COMMENT ON COLUMN users.status IS '用户状态：enabled / disabled';
COMMENT ON COLUMN users.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN users.created_by IS '创建人主体 ID';
COMMENT ON COLUMN users.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN users.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN users.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN users.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN users.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN users.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN users.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (username, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (phone, deleted_at) WHERE phone <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (email, deleted_at) WHERE email <> '';

CREATE TABLE IF NOT EXISTS tenant_user_memberships (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'enabled',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE tenant_user_memberships IS '租户用户关系表，保存全局用户在租户内的固定角色和启用状态';
COMMENT ON COLUMN tenant_user_memberships.id IS '租户用户关系主键 ID';
COMMENT ON COLUMN tenant_user_memberships.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN tenant_user_memberships.user_id IS '全局用户 ID';
COMMENT ON COLUMN tenant_user_memberships.role IS '租户固定角色：tenant_admin / teacher / student';
COMMENT ON COLUMN tenant_user_memberships.status IS '租户成员状态：enabled / disabled';
COMMENT ON COLUMN tenant_user_memberships.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tenant_user_memberships.created_by IS '创建人主体 ID';
COMMENT ON COLUMN tenant_user_memberships.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tenant_user_memberships.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tenant_user_memberships.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN tenant_user_memberships.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tenant_user_memberships.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN tenant_user_memberships.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id);
COMMENT ON INDEX uk_tenant_user_memberships_user IS '同一用户在同一租户只能拥有一条固定角色关系';

CREATE TABLE IF NOT EXISTS questions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    space_id BIGINT,
    type VARCHAR(32) NOT NULL,
    difficulty VARCHAR(32) NOT NULL DEFAULT 'medium',
    title TEXT NOT NULL,
    analysis TEXT NOT NULL DEFAULT '',
    standard_answer TEXT NOT NULL DEFAULT '',
    reference_answer TEXT NOT NULL DEFAULT '',
    score_default NUMERIC(10,2) NOT NULL DEFAULT 0,
    choice_display_count INTEGER,
    shuffle_options BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE questions IS '题目表，保存租户公共题库和空间题库中的题目';
COMMENT ON COLUMN questions.id IS '题目主键 ID';
COMMENT ON COLUMN questions.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN questions.space_id IS '所属空间 ID，NULL 表示租户公共题库';
COMMENT ON COLUMN questions.type IS '题型：single / multiple / judge / fill_blank / short_text';
COMMENT ON COLUMN questions.difficulty IS '难度：easy / medium / hard';
COMMENT ON COLUMN questions.title IS '题干内容';
COMMENT ON COLUMN questions.analysis IS '题目解析，出题人可选填';
COMMENT ON COLUMN questions.standard_answer IS '填空题标准答案或判断题标准答案';
COMMENT ON COLUMN questions.reference_answer IS '简答题参考答案';
COMMENT ON COLUMN questions.score_default IS '默认分值';
COMMENT ON COLUMN questions.choice_display_count IS '选择题展示选项数量，可为空';
COMMENT ON COLUMN questions.shuffle_options IS '题库默认选项随机设置';
COMMENT ON COLUMN questions.status IS '题目状态：draft / enabled / disabled';
COMMENT ON COLUMN questions.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN questions.created_by IS '创建人主体 ID';
COMMENT ON COLUMN questions.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN questions.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN questions.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN questions.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN questions.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN questions.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN questions.deleted_at IS '软删除时间，0 表示未删除';
CREATE INDEX IF NOT EXISTS idx_questions_filter ON questions (tenant_id, space_id, type, difficulty, status, deleted_at);
CREATE INDEX IF NOT EXISTS idx_questions_creator ON questions (tenant_id, created_by, deleted_at);

CREATE TABLE IF NOT EXISTS question_options (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    question_id BIGINT NOT NULL,
    option_key VARCHAR(32) NOT NULL,
    sort_order INTEGER NOT NULL,
    content TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    is_distractor BOOLEAN NOT NULL DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE question_options IS '题目选项表，保存选择题和判断题选项';
COMMENT ON COLUMN question_options.id IS '题目选项主键 ID';
COMMENT ON COLUMN question_options.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN question_options.question_id IS '题目 ID';
COMMENT ON COLUMN question_options.option_key IS '出题编辑时的原始展示标签，例如 A / B / C / D，不参与判分';
COMMENT ON COLUMN question_options.sort_order IS '选项原始排序';
COMMENT ON COLUMN question_options.content IS '选项内容';
COMMENT ON COLUMN question_options.is_correct IS '是否为正确答案';
COMMENT ON COLUMN question_options.is_distractor IS '是否可作为随机补位干扰项';
COMMENT ON COLUMN question_options.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN question_options.created_by IS '创建人主体 ID';
COMMENT ON COLUMN question_options.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN question_options.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN question_options.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN question_options.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN question_options.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN question_options.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_key ON question_options (tenant_id, question_id, option_key);
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_sort_order ON question_options (tenant_id, question_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_question_options_question ON question_options (tenant_id, question_id);

CREATE TABLE IF NOT EXISTS tags (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE tags IS '标签表，保存知识点、章节、技能点等题目标签';
COMMENT ON COLUMN tags.id IS '标签主键 ID';
COMMENT ON COLUMN tags.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN tags.name IS '标签名称，例如知识点、章节、技能点';
COMMENT ON COLUMN tags.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tags.created_by IS '创建人主体 ID';
COMMENT ON COLUMN tags.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tags.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN tags.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN tags.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN tags.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN tags.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN tags.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_tags_name_deleted_at ON tags (tenant_id, name, deleted_at);
CREATE INDEX IF NOT EXISTS idx_tags_tenant_deleted_at ON tags (tenant_id, deleted_at);

CREATE TABLE IF NOT EXISTS question_tags (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    question_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE question_tags IS '题目标签关系表，保存题目和标签的绑定关系';
COMMENT ON COLUMN question_tags.id IS '题目标签关系主键 ID';
COMMENT ON COLUMN question_tags.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN question_tags.question_id IS '题目 ID';
COMMENT ON COLUMN question_tags.tag_id IS '标签 ID';
COMMENT ON COLUMN question_tags.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN question_tags.created_by IS '创建人主体 ID';
COMMENT ON COLUMN question_tags.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN question_tags.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_question_tags_tag ON question_tags (tenant_id, question_id, tag_id);
CREATE INDEX IF NOT EXISTS idx_question_tags_tag ON question_tags (tenant_id, tag_id);

CREATE TABLE IF NOT EXISTS papers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    space_id BIGINT,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    total_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    build_mode VARCHAR(32) NOT NULL,
    shuffle_questions BOOLEAN NOT NULL DEFAULT FALSE,
    show_analysis BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE papers IS '试卷表，保存公共试卷和空间内试卷';
COMMENT ON COLUMN papers.id IS '试卷主键 ID';
COMMENT ON COLUMN papers.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN papers.space_id IS '所属空间 ID，NULL 表示租户公共试卷';
COMMENT ON COLUMN papers.name IS '试卷名称';
COMMENT ON COLUMN papers.description IS '试卷说明';
COMMENT ON COLUMN papers.total_score IS '试卷总分，由系统按大题题目聚合计算';
COMMENT ON COLUMN papers.build_mode IS '组卷方式：manual / rule_fixed / rule_live';
COMMENT ON COLUMN papers.shuffle_questions IS '是否对每个考生随机题目顺序';
COMMENT ON COLUMN papers.show_analysis IS '成绩可见后是否向考生展示题目解析';
COMMENT ON COLUMN papers.status IS '试卷状态：draft / enabled / disabled';
COMMENT ON COLUMN papers.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN papers.created_by IS '创建人主体 ID';
COMMENT ON COLUMN papers.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN papers.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN papers.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN papers.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN papers.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN papers.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN papers.deleted_at IS '软删除时间，0 表示未删除';
CREATE INDEX IF NOT EXISTS idx_papers_filter ON papers (tenant_id, space_id, status, deleted_at);

CREATE TABLE IF NOT EXISTS paper_sections (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    paper_id BIGINT NOT NULL,
    sort_order INTEGER NOT NULL,
    name VARCHAR(128) NOT NULL,
    question_type VARCHAR(32) NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    total_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    question_count INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE paper_sections IS '试卷大题表，保存大题结构、题型和小计信息';
COMMENT ON COLUMN paper_sections.id IS '试卷大题主键 ID';
COMMENT ON COLUMN paper_sections.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN paper_sections.paper_id IS '试卷 ID';
COMMENT ON COLUMN paper_sections.sort_order IS '大题排序';
COMMENT ON COLUMN paper_sections.name IS '大题名称，例如一、单选题';
COMMENT ON COLUMN paper_sections.question_type IS '大题题型';
COMMENT ON COLUMN paper_sections.instructions IS '大题作答说明';
COMMENT ON COLUMN paper_sections.total_score IS '大题小计分，由系统聚合计算';
COMMENT ON COLUMN paper_sections.question_count IS '大题题目数量，由系统聚合计算';
COMMENT ON COLUMN paper_sections.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_sections.created_by IS '创建人主体 ID';
COMMENT ON COLUMN paper_sections.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_sections.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_sections.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN paper_sections.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_sections.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN paper_sections.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN paper_sections.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_sort_order ON paper_sections (tenant_id, paper_id, sort_order);
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_id_paper ON paper_sections (tenant_id, id, paper_id);

CREATE TABLE IF NOT EXISTS paper_section_questions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    section_id BIGINT NOT NULL,
    paper_id BIGINT NOT NULL,
    question_id BIGINT NOT NULL,
    sort_order INTEGER NOT NULL,
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    shuffle_options BOOLEAN,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT fk_paper_section_questions_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)
);
COMMENT ON TABLE paper_section_questions IS '大题题目关系表，保存手动或固化组卷后的题目';
COMMENT ON COLUMN paper_section_questions.id IS '大题题目关系主键 ID';
COMMENT ON COLUMN paper_section_questions.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN paper_section_questions.section_id IS '大题 ID';
COMMENT ON COLUMN paper_section_questions.paper_id IS '试卷 ID，冗余保存用于减少查询 JOIN';
COMMENT ON COLUMN paper_section_questions.question_id IS '题目 ID';
COMMENT ON COLUMN paper_section_questions.sort_order IS '题目在大题中的排序';
COMMENT ON COLUMN paper_section_questions.score IS '该题在本试卷中的分值';
COMMENT ON COLUMN paper_section_questions.shuffle_options IS '手动或固化组卷下该题是否随机选项，NULL 表示回退题库默认值';
COMMENT ON COLUMN paper_section_questions.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_section_questions.created_by IS '创建人主体 ID';
COMMENT ON COLUMN paper_section_questions.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_section_questions.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_section_questions.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN paper_section_questions.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_section_questions.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN paper_section_questions.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_question ON paper_section_questions (tenant_id, paper_id, question_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_sort_order ON paper_section_questions (tenant_id, section_id, sort_order);

CREATE TABLE IF NOT EXISTS paper_section_rules (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    section_id BIGINT NOT NULL,
    paper_id BIGINT NOT NULL,
    sort_order INTEGER NOT NULL,
    difficulty VARCHAR(32),
    tag_filter TEXT NOT NULL DEFAULT '[]',
    question_count INTEGER NOT NULL,
    score_per_question NUMERIC(10,2) NOT NULL DEFAULT 0,
    shuffle_options BOOLEAN,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT fk_paper_section_rules_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)
);
COMMENT ON TABLE paper_section_rules IS '大题抽题规则表，保存规则组卷条件';
COMMENT ON COLUMN paper_section_rules.id IS '大题抽题规则主键 ID';
COMMENT ON COLUMN paper_section_rules.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN paper_section_rules.section_id IS '大题 ID';
COMMENT ON COLUMN paper_section_rules.paper_id IS '试卷 ID，冗余保存用于减少查询 JOIN';
COMMENT ON COLUMN paper_section_rules.sort_order IS '规则在大题内的排序';
COMMENT ON COLUMN paper_section_rules.difficulty IS '抽题难度条件，NULL 表示不限难度';
COMMENT ON COLUMN paper_section_rules.tag_filter IS '标签过滤条件，JSON 数组字符串';
COMMENT ON COLUMN paper_section_rules.question_count IS '该规则抽题数量';
COMMENT ON COLUMN paper_section_rules.score_per_question IS '该规则下每题分值';
COMMENT ON COLUMN paper_section_rules.shuffle_options IS '规则组卷下是否随机选项，NULL 表示回退题库默认值';
COMMENT ON COLUMN paper_section_rules.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_section_rules.created_by IS '创建人主体 ID';
COMMENT ON COLUMN paper_section_rules.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_section_rules.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN paper_section_rules.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN paper_section_rules.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN paper_section_rules.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN paper_section_rules.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_rules_sort_order ON paper_section_rules (tenant_id, section_id, sort_order);

CREATE TABLE IF NOT EXISTS exams (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    paper_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    start_time BIGINT NOT NULL,
    end_time BIGINT NOT NULL,
    duration_minutes INTEGER NOT NULL,
    max_attempts INTEGER NOT NULL DEFAULT 1,
    result_strategy VARCHAR(32) NOT NULL DEFAULT 'latest',
    publish_mode VARCHAR(32) NOT NULL DEFAULT 'manual_publish',
    score_publish_time BIGINT,
    invite_code VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE exams IS '考试表，保存试卷发布后的考试安排';
COMMENT ON COLUMN exams.id IS '考试主键 ID';
COMMENT ON COLUMN exams.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exams.paper_id IS '关联试卷 ID';
COMMENT ON COLUMN exams.name IS '考试名称';
COMMENT ON COLUMN exams.start_time IS '考试开始时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exams.end_time IS '考试结束时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exams.duration_minutes IS '单次作答时长，单位分钟';
COMMENT ON COLUMN exams.max_attempts IS '每名考生最多作答次数';
COMMENT ON COLUMN exams.result_strategy IS '多次作答成绩策略：latest / highest';
COMMENT ON COLUMN exams.publish_mode IS '成绩发布模式：immediate_score / manual_publish';
COMMENT ON COLUMN exams.score_publish_time IS '统一成绩公布时间，可为空';
COMMENT ON COLUMN exams.invite_code IS '考试邀请码';
COMMENT ON COLUMN exams.status IS '考试状态：draft / published / closed';
COMMENT ON COLUMN exams.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exams.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exams.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exams.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exams.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN exams.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exams.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN exams.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
COMMENT ON COLUMN exams.deleted_at IS '软删除时间，0 表示未删除';
CREATE UNIQUE INDEX IF NOT EXISTS uk_exams_invite_code ON exams (invite_code);

CREATE TABLE IF NOT EXISTS exam_targets (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_targets IS '考试发布范围表，保存考试面向的空间或用户';
COMMENT ON COLUMN exam_targets.id IS '考试发布范围主键 ID';
COMMENT ON COLUMN exam_targets.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_targets.exam_id IS '考试 ID';
COMMENT ON COLUMN exam_targets.target_type IS '发布目标类型：space / user';
COMMENT ON COLUMN exam_targets.target_id IS '发布目标 ID';
COMMENT ON COLUMN exam_targets.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_targets.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_targets.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_targets.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_targets_target ON exam_targets (tenant_id, exam_id, target_type, target_id);

CREATE TABLE IF NOT EXISTS exam_live_question_pools (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    section_id BIGINT NOT NULL,
    rule_id BIGINT NOT NULL,
    question_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_live_question_pools IS 'rule_live 发布态题池表，保存发布时冻结的候选题目';
COMMENT ON COLUMN exam_live_question_pools.id IS 'rule_live 发布态题池主键 ID';
COMMENT ON COLUMN exam_live_question_pools.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_live_question_pools.exam_id IS '考试 ID';
COMMENT ON COLUMN exam_live_question_pools.section_id IS '大题 ID';
COMMENT ON COLUMN exam_live_question_pools.rule_id IS '大题抽题规则 ID';
COMMENT ON COLUMN exam_live_question_pools.question_id IS '候选题目 ID';
COMMENT ON COLUMN exam_live_question_pools.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_live_question_pools.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_live_question_pools.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_live_question_pools.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_live_question_pools_question ON exam_live_question_pools (tenant_id, exam_id, section_id, rule_id, question_id);

CREATE TABLE IF NOT EXISTS exam_attempts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    attempt_no INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'in_progress',
    started_at BIGINT NOT NULL,
    submitted_at BIGINT,
    exam_token_hash VARCHAR(128) NOT NULL,
    exam_token_expires_at BIGINT NOT NULL,
    objective_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    subjective_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    total_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_attempts IS '考生作答表，保存每次考试作答过程和成绩';
COMMENT ON COLUMN exam_attempts.id IS '考生作答主键 ID';
COMMENT ON COLUMN exam_attempts.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_attempts.exam_id IS '考试 ID';
COMMENT ON COLUMN exam_attempts.user_id IS '考生用户 ID';
COMMENT ON COLUMN exam_attempts.attempt_no IS '第几次作答，从 1 开始';
COMMENT ON COLUMN exam_attempts.status IS '作答状态：in_progress / submitted / graded';
COMMENT ON COLUMN exam_attempts.started_at IS '开始作答时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_attempts.submitted_at IS '提交时间，可为空';
COMMENT ON COLUMN exam_attempts.exam_token_hash IS '考试过程 token 哈希';
COMMENT ON COLUMN exam_attempts.exam_token_expires_at IS '考试过程 token 过期时间';
COMMENT ON COLUMN exam_attempts.objective_score IS '客观题得分';
COMMENT ON COLUMN exam_attempts.subjective_score IS '主观题得分';
COMMENT ON COLUMN exam_attempts.total_score IS '总分';
COMMENT ON COLUMN exam_attempts.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_attempts.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_attempts.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_attempts.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_attempts.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN exam_attempts.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_attempts.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN exam_attempts.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_attempts_attempt_no ON exam_attempts (tenant_id, exam_id, user_id, attempt_no);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_token_status ON exam_attempts (exam_token_hash, status);

CREATE TABLE IF NOT EXISTS exam_attempt_questions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    attempt_id BIGINT NOT NULL,
    section_id BIGINT NOT NULL,
    question_id BIGINT NOT NULL,
    section_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INTEGER NOT NULL,
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    question_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    option_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    correct_answer_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_attempt_questions IS '考生题目快照表，保存考生本次作答看到的题目快照';
COMMENT ON COLUMN exam_attempt_questions.id IS '考生题目快照主键 ID';
COMMENT ON COLUMN exam_attempt_questions.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_attempt_questions.attempt_id IS '作答 ID';
COMMENT ON COLUMN exam_attempt_questions.section_id IS '原始大题 ID，仅用于溯源';
COMMENT ON COLUMN exam_attempt_questions.question_id IS '原始题目 ID';
COMMENT ON COLUMN exam_attempt_questions.section_snapshot IS '大题快照 JSON，包含大题名称和作答说明';
COMMENT ON COLUMN exam_attempt_questions.sort_order IS '该考生看到的全局题号，从 1 连续递增';
COMMENT ON COLUMN exam_attempt_questions.score IS '该题在本次作答中的分值';
COMMENT ON COLUMN exam_attempt_questions.question_snapshot IS '题干快照 JSON';
COMMENT ON COLUMN exam_attempt_questions.option_snapshot IS '选项快照 JSON';
COMMENT ON COLUMN exam_attempt_questions.correct_answer_snapshot IS '正确答案快照 JSON';
COMMENT ON COLUMN exam_attempt_questions.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_attempt_questions.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_attempt_questions.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_attempt_questions.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_attempt_questions.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN exam_attempt_questions.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_attempt_questions.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN exam_attempt_questions.ext_json IS 'JSON 扩展字段，保存非主流程元数据';

CREATE TABLE IF NOT EXISTS exam_answers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    attempt_id BIGINT NOT NULL,
    attempt_question_id BIGINT NOT NULL,
    answer_content TEXT NOT NULL DEFAULT '',
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    grading_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    graded_by BIGINT NOT NULL DEFAULT 0,
    graded_at BIGINT,
    grader_comment TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_answers IS '答案表，保存考生答案、得分和阅卷信息';
COMMENT ON COLUMN exam_answers.id IS '答案主键 ID';
COMMENT ON COLUMN exam_answers.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_answers.attempt_id IS '作答 ID';
COMMENT ON COLUMN exam_answers.attempt_question_id IS '考生题目快照 ID';
COMMENT ON COLUMN exam_answers.answer_content IS '考生答案内容';
COMMENT ON COLUMN exam_answers.score IS '该题得分';
COMMENT ON COLUMN exam_answers.grading_status IS '阅卷状态：auto / pending / graded';
COMMENT ON COLUMN exam_answers.graded_by IS '阅卷人用户 ID，0 表示未阅卷';
COMMENT ON COLUMN exam_answers.graded_at IS '阅卷时间，可为空';
COMMENT ON COLUMN exam_answers.grader_comment IS '阅卷评语';
COMMENT ON COLUMN exam_answers.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_answers.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_answers.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_answers.updated_at IS '更新时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_answers.updated_by IS '更新人主体 ID';
COMMENT ON COLUMN exam_answers.updated_by_type IS '更新人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_answers.version IS '数据版本号，用于乐观锁';
COMMENT ON COLUMN exam_answers.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_answers_attempt_question ON exam_answers (tenant_id, attempt_id, attempt_question_id);

CREATE TABLE IF NOT EXISTS exam_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    attempt_id BIGINT NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    event_time BIGINT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_by_type VARCHAR(32) NOT NULL DEFAULT 'system',
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE exam_events IS '考试事件表，追加记录切屏、自动保存、提交等考试过程事件';
COMMENT ON COLUMN exam_events.id IS '考试事件主键 ID';
COMMENT ON COLUMN exam_events.tenant_id IS '所属租户 ID';
COMMENT ON COLUMN exam_events.attempt_id IS '作答 ID';
COMMENT ON COLUMN exam_events.event_type IS '事件类型：blur / focus / auto_save / submit / auto_submit';
COMMENT ON COLUMN exam_events.event_time IS '事件发生时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_events.payload IS '事件负载 JSON';
COMMENT ON COLUMN exam_events.created_at IS '创建时间，Unix 毫秒时间戳';
COMMENT ON COLUMN exam_events.created_by IS '创建人主体 ID';
COMMENT ON COLUMN exam_events.created_by_type IS '创建人主体类型：platform_user / tenant_user / system';
COMMENT ON COLUMN exam_events.ext_json IS 'JSON 扩展字段，保存非主流程元数据';
