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
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
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
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
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
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
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
    updated_at INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    ext_json TEXT NOT NULL DEFAULT '{}'
);
-- 同一空间内同一个配置键只能存在一条记录。
CREATE UNIQUE INDEX IF NOT EXISTS uk_space_configs_key ON space_configs (tenant_id, space_id, config_key);
