CREATE TABLE IF NOT EXISTS tenants (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '租户主键 ID',
    name VARCHAR(128) NOT NULL COMMENT '租户名称，例如学校、企业、培训机构',
    logo_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT '企业或机构 Logo 地址',
    description TEXT NOT NULL COMMENT '企业或机构描述',
    tenant_code VARCHAR(32) NOT NULL COMMENT '租户码，用于注册归属',
    allow_register TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许该租户用户自注册',
    status VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT '租户状态：enabled / disabled',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人用户 ID',
    updated_at BIGINT NOT NULL COMMENT '更新时间，Unix 毫秒时间戳',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人用户 ID',
    version BIGINT NOT NULL DEFAULT 1 COMMENT '数据版本号，用于乐观锁',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，保存非主流程元数据',
    deleted_at BIGINT NOT NULL DEFAULT 0 COMMENT '软删除时间，0 表示未删除',
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenants_tenant_code_deleted_at (tenant_code, deleted_at)
) COMMENT='租户表，保存学校、企业、培训机构等租户主体';

CREATE TABLE IF NOT EXISTS spaces (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '空间主键 ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户 ID',
    name VARCHAR(128) NOT NULL COMMENT '空间名称，例如班级、专业、课程、培训项目',
    logo_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT '空间 Logo 地址，可为空',
    description TEXT NOT NULL COMMENT '空间描述，可为空',
    type VARCHAR(32) NOT NULL DEFAULT 'custom' COMMENT '空间类型：class / major / course / training / custom',
    status VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT '空间状态：enabled / disabled',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人用户 ID',
    updated_at BIGINT NOT NULL COMMENT '更新时间，Unix 毫秒时间戳',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人用户 ID',
    version BIGINT NOT NULL DEFAULT 1 COMMENT '数据版本号，用于乐观锁',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，保存非主流程元数据',
    deleted_at BIGINT NOT NULL DEFAULT 0 COMMENT '软删除时间，0 表示未删除',
    PRIMARY KEY (id),
    KEY idx_spaces_tenant_id (tenant_id)
) COMMENT='空间表，保存班级、专业、课程、培训项目等租户内组织边界';

CREATE TABLE IF NOT EXISTS space_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '空间成员关系主键 ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户 ID',
    space_id BIGINT UNSIGNED NOT NULL COMMENT '空间 ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '租户用户 ID',
    role_in_space VARCHAR(32) NOT NULL COMMENT '空间内角色：space_admin / teacher / student',
    status VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT '空间成员状态：enabled / disabled',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人用户 ID',
    updated_at BIGINT NOT NULL COMMENT '更新时间，Unix 毫秒时间戳',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人用户 ID',
    version BIGINT NOT NULL DEFAULT 1 COMMENT '数据版本号，用于乐观锁',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，保存非主流程元数据',
    deleted_at BIGINT NOT NULL DEFAULT 0 COMMENT '软删除时间，0 表示未移除',
    PRIMARY KEY (id),
    UNIQUE KEY uk_space_members_user_deleted_at (tenant_id, space_id, user_id, deleted_at),
    KEY idx_space_members_space_status (tenant_id, space_id, status, deleted_at)
) COMMENT='空间成员表，保存用户在空间内的角色和启用状态';

CREATE TABLE IF NOT EXISTS space_configs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '空间配置主键 ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户 ID',
    space_id BIGINT UNSIGNED NOT NULL COMMENT '空间 ID',
    config_key VARCHAR(128) NOT NULL COMMENT '配置键，例如 default_exam_duration',
    config_value TEXT NOT NULL COMMENT '配置值，按字符串保存',
    value_type VARCHAR(32) NOT NULL DEFAULT 'string' COMMENT '配置值类型：string / number / bool / json',
    description VARCHAR(512) NOT NULL DEFAULT '' COMMENT '配置说明',
    created_at BIGINT NOT NULL COMMENT '创建时间，Unix 毫秒时间戳',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人用户 ID',
    updated_at BIGINT NOT NULL COMMENT '更新时间，Unix 毫秒时间戳',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人用户 ID',
    version BIGINT NOT NULL DEFAULT 1 COMMENT '数据版本号，用于乐观锁',
    ext_json JSON NOT NULL COMMENT 'JSON 扩展字段，保存非主流程元数据',
    PRIMARY KEY (id),
    UNIQUE KEY uk_space_configs_key (tenant_id, space_id, config_key)
) COMMENT='空间配置表，保存空间级配置项';
