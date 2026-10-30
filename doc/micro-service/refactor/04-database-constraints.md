# PostgreSQL 数据库约束

正式部署数据库为 PostgreSQL。SQLite 仅作为旧版本数据迁移来源或开发兼容方案，不作为新模型的权威方言。

## 0. 部署约束

正式部署统一采用 Docker：

```text
Noder 应用容器
PostgreSQL 数据库容器
```

PostgreSQL 使用持久化 Volume，数据库数据不得存放在容器临时文件系统中。

部署必须通过环境变量或 Secret 注入：

```text
DATABASE_URL
数据库用户名和密码
管理端密钥
加密密钥
```

数据库迁移由应用发布流程显式执行，不在容器启动时静默修改生产结构。生产环境必须提供数据库备份和恢复方案。

## 1. User

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(128) UNIQUE,
    name VARCHAR(128) NOT NULL,
    subscription_token TEXT NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled'))
);
```

## 2. Credential

一个 Credential 只能属于一个 User。一个 Credential 可以被同一 User 的多个 AccessGrant 复用。

```sql
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    scheme VARCHAR(32) NOT NULL,
    display_name VARCHAR(128),
    uuid_value TEXT,
    username TEXT,
    password_ciphertext TEXT,
    secret_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_binding',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_credentials_id_user UNIQUE (id, user_id),
    CONSTRAINT ck_credentials_status CHECK (
        status IN ('pending_binding', 'active', 'disabled', 'revoked')
    )
);

CREATE UNIQUE INDEX uk_credentials_uuid_value
    ON credentials(uuid_value)
    WHERE uuid_value IS NOT NULL;

CREATE UNIQUE INDEX uk_credentials_default_user
    ON credentials(user_id)
    WHERE is_default = TRUE;

CREATE INDEX idx_credentials_user_id ON credentials(user_id);
```

`uuid_value` 只用于 UUID 型认证方案。Socks5 等方案使用 `username` 和 `password_ciphertext`。

## 3. Endpoint

Endpoint 是具体协议接入点，不保存用户凭证。协议差异字段保存为 JSONB，但写入必须经过协议契约校验。

```sql
CREATE TABLE endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol VARCHAR(32) NOT NULL,
    name VARCHAR(128),
    tag VARCHAR(128) NOT NULL,
    server_address TEXT NOT NULL,
    server_port INTEGER NOT NULL,
    country VARCHAR(8),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_endpoints_port CHECK (server_port BETWEEN 1 AND 65535),
    CONSTRAINT ck_endpoints_status CHECK (status IN ('active', 'disabled'))
);

CREATE INDEX idx_endpoints_protocol ON endpoints(protocol);
CREATE INDEX idx_endpoints_status ON endpoints(status);
```

## 4. AccessGrant

AccessGrant 只表示已经完成的用户资源授权。待绑定 Credential 不创建空的 AccessGrant。

```sql
CREATE TABLE access_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE RESTRICT,
    credential_id UUID NOT NULL,
    display_name VARCHAR(128),
    tag_override VARCHAR(128),
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    is_urltest BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    override_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    remark TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_user_endpoint UNIQUE (user_id, endpoint_id),
    CONSTRAINT fk_access_grant_credential
        FOREIGN KEY (credential_id, user_id)
        REFERENCES credentials (id, user_id)
        ON DELETE RESTRICT,
    CONSTRAINT ck_access_grants_status CHECK (
        status IN ('active', 'disabled', 'revoked')
    )
);

CREATE INDEX idx_access_grants_user_id ON access_grants(user_id);
CREATE INDEX idx_access_grants_endpoint_id ON access_grants(endpoint_id);
CREATE INDEX idx_access_grants_credential_id ON access_grants(credential_id);
```

## 5. AgentNode 与 Endpoint 绑定

当前手动部署时没有绑定记录。一个 AgentNode 可以控制多个 Endpoint，一个 Endpoint 可以绑定多个 AgentNode。

```sql
CREATE TABLE agent_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    control_token_ciphertext TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'offline',
    last_heartbeat_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_agent_nodes_status CHECK (
        status IN ('online', 'offline', 'disabled')
    )
);

CREATE TABLE endpoint_agent_bindings (
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE RESTRICT,
    agent_node_id UUID NOT NULL REFERENCES agent_nodes(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    last_sync_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (endpoint_id, agent_node_id)
);
```

## 6. Agent 凭证部署状态

该表是运行时投影，不改变 Credential 和 AccessGrant 的权威关系。Agent 阶段启用。

```sql
CREATE TABLE credential_deployments (
    credential_id UUID NOT NULL REFERENCES credentials(id) ON DELETE RESTRICT,
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE RESTRICT,
    agent_node_id UUID NOT NULL REFERENCES agent_nodes(id) ON DELETE RESTRICT,
    desired_state VARCHAR(32) NOT NULL,
    applied_state VARCHAR(32) NOT NULL DEFAULT 'unknown',
    last_sync_at TIMESTAMPTZ,
    last_error TEXT,
    PRIMARY KEY (credential_id, endpoint_id, agent_node_id)
);
```

## 7. 删除原则

用户、凭证、端点和授权关系不通过物理删除表达业务失效，使用状态、撤销时间和过期时间。物理删除受外键约束保护。
