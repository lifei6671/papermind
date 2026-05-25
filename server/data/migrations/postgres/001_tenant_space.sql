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
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE tenants IS '租户表，保存学校、企业、培训机构等租户主体';
COMMENT ON COLUMN tenants.logo_url IS '企业或机构 Logo 地址';
COMMENT ON COLUMN tenants.description IS '企业或机构描述';
COMMENT ON COLUMN tenants.tenant_code IS '租户码，用于注册归属';
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
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE spaces IS '空间表，保存班级、专业、课程、培训项目等租户内组织边界';
COMMENT ON COLUMN spaces.logo_url IS '空间 Logo 地址，可为空';
COMMENT ON COLUMN spaces.description IS '空间描述，可为空';
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
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at BIGINT NOT NULL DEFAULT 0
);
COMMENT ON TABLE space_members IS '空间成员表，保存用户在空间内的角色和启用状态';
COMMENT ON COLUMN space_members.role_in_space IS '空间内角色：space_admin / teacher / student';
COMMENT ON COLUMN space_members.status IS '空间成员状态：enabled / disabled';
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
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
COMMENT ON TABLE space_configs IS '空间配置表，保存空间级配置项';
COMMENT ON COLUMN space_configs.config_key IS '配置键，例如 default_exam_duration';
COMMENT ON COLUMN space_configs.config_value IS '配置值，按字符串保存';
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_configs_key ON space_configs (tenant_id, space_id, config_key);
