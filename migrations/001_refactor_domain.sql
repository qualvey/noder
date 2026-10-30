-- Canonical PostgreSQL schema for the domain refactor.
-- This migration is intentionally explicit and is not run silently at startup.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
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

CREATE TABLE IF NOT EXISTS credentials (
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
    CONSTRAINT ck_credentials_status CHECK (status IN ('pending_binding', 'active', 'disabled', 'revoked'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_credentials_uuid_value ON credentials(uuid_value) WHERE uuid_value IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_credentials_default_user ON credentials(user_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS endpoints (
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

CREATE TABLE IF NOT EXISTS access_grants (
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
    CONSTRAINT uk_access_grants_user_endpoint UNIQUE (user_id, endpoint_id),
    CONSTRAINT fk_access_grants_credential FOREIGN KEY (credential_id, user_id)
        REFERENCES credentials (id, user_id) ON DELETE RESTRICT,
    CONSTRAINT ck_access_grants_status CHECK (status IN ('active', 'disabled', 'revoked'))
);

CREATE TABLE IF NOT EXISTS agent_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    control_token_ciphertext TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'offline',
    last_heartbeat_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_agent_nodes_status CHECK (status IN ('online', 'offline', 'disabled'))
);

CREATE TABLE IF NOT EXISTS endpoint_agent_bindings (
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE RESTRICT,
    agent_node_id UUID NOT NULL REFERENCES agent_nodes(id) ON DELETE RESTRICT,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    last_sync_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (endpoint_id, agent_node_id)
);

CREATE TABLE IF NOT EXISTS credential_deployments (
    credential_id UUID NOT NULL REFERENCES credentials(id) ON DELETE RESTRICT,
    endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE RESTRICT,
    agent_node_id UUID NOT NULL REFERENCES agent_nodes(id) ON DELETE RESTRICT,
    desired_state VARCHAR(32) NOT NULL,
    applied_state VARCHAR(32) NOT NULL DEFAULT 'unknown',
    last_sync_at TIMESTAMPTZ,
    last_error TEXT,
    PRIMARY KEY (credential_id, endpoint_id, agent_node_id)
);

CREATE TABLE IF NOT EXISTS legacy_migration_map (
    id BIGSERIAL PRIMARY KEY,
    legacy_user_id BIGINT NOT NULL,
    legacy_node_id BIGINT,
    legacy_endpoint_id BIGINT NOT NULL UNIQUE,
    credential_id UUID NOT NULL REFERENCES credentials(id) ON DELETE RESTRICT,
    access_grant_id UUID REFERENCES access_grants(id) ON DELETE RESTRICT,
    conflict TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
