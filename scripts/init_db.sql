-- 用于创建基础表结构和测试数据

-- Required for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(255),
    avatar TEXT,
    provider VARCHAR(50) DEFAULT 'email',
    tier VARCHAR(20) DEFAULT 'free',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建快照表
CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    tab_groups JSONB NOT NULL,
    group_count INTEGER DEFAULT 0,
    tab_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, name)
);

-- 创建订阅计划表
CREATE TABLE IF NOT EXISTS subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    tier VARCHAR(20) NOT NULL,
    price_cents INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    billing_interval VARCHAR(20) NOT NULL,
    paddle_price_id VARCHAR(255),
    ai_credits_monthly INTEGER DEFAULT 0,
    max_workspaces INTEGER,
    features JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建用户订阅表
CREATE TABLE IF NOT EXISTS user_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES subscription_plans(id),
    paddle_subscription_id VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    current_period_start TIMESTAMP WITH TIME ZONE,
    current_period_end TIMESTAMP WITH TIME ZONE,
    cancel_at_period_end BOOLEAN DEFAULT false,
    canceled_at TIMESTAMP WITH TIME ZONE,
    trial_start TIMESTAMP WITH TIME ZONE,
    trial_end TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建AI积分表
CREATE TABLE IF NOT EXISTS ai_credits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credits_total INTEGER NOT NULL DEFAULT 0,
    credits_used INTEGER NOT NULL DEFAULT 0,
    credits_remaining INTEGER NOT NULL DEFAULT 0,
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_snapshots_user_id ON snapshots(user_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_user_name ON snapshots(user_id, name);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_credits_user_id ON ai_credits(user_id);

-- 插入测试数据
-- Seed a test user with a valid bcrypt hash using pgcrypto
INSERT INTO users (email, password_hash, name, tier, created_at, updated_at) 
VALUES (
    'test@example.com',
    crypt('password', gen_salt('bf', 10)),
    'Test User', 
    'free',
    NOW() - INTERVAL '30 days',
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- 插入基础订阅计划
INSERT INTO subscription_plans (name, display_name, tier, price_cents, billing_interval, ai_credits_monthly)
VALUES 
    ('free', 'Free Plan', 'free', 0, 'monthly', 100),
    ('pro', 'Pro Plan', 'pro', 999, 'monthly', 1000),
    ('power', 'Power Plan', 'power', 1999, 'monthly', 5000)
ON CONFLICT DO NOTHING;

-- 创建更新时间触发器函数
-- Some GUI runners mishandle $$-quoted bodies; use E'' quoting
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER
LANGUAGE plpgsql
AS E'
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
';

-- 为所有表创建更新时间触发器
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_snapshots_updated_at ON snapshots;
CREATE TRIGGER update_snapshots_updated_at BEFORE UPDATE ON snapshots FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_subscription_plans_updated_at ON subscription_plans;
CREATE TRIGGER update_subscription_plans_updated_at BEFORE UPDATE ON subscription_plans FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_user_subscriptions_updated_at ON user_subscriptions;
CREATE TRIGGER update_user_subscriptions_updated_at BEFORE UPDATE ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_ai_credits_updated_at ON ai_credits;
CREATE TRIGGER update_ai_credits_updated_at BEFORE UPDATE ON ai_credits FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================
-- Organizations & Spaces schema
-- =============================

-- Organizations
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT,
    avatar TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Organization memberships (owner/admin/member)
CREATE TABLE IF NOT EXISTS organization_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(organization_id, user_id)
);

-- Spaces (browsable by all members by default)
CREATE TABLE IF NOT EXISTS spaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Space permissions (per-member edit control)
CREATE TABLE IF NOT EXISTS space_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_edit BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(space_id, user_id)
);

-- Invitations
CREATE TABLE IF NOT EXISTS organization_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(token)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations(owner_id);
CREATE INDEX IF NOT EXISTS idx_memberships_user ON organization_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_org ON organization_memberships(organization_id);
CREATE INDEX IF NOT EXISTS idx_spaces_org ON spaces(organization_id);
CREATE INDEX IF NOT EXISTS idx_space_permissions_space ON space_permissions(space_id);
CREATE INDEX IF NOT EXISTS idx_space_permissions_user ON space_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON organization_invitations(email);
CREATE INDEX IF NOT EXISTS idx_invitations_org ON organization_invitations(organization_id);

-- Triggers for updated_at
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_spaces_updated_at ON spaces;
CREATE TRIGGER update_spaces_updated_at BEFORE UPDATE ON spaces FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_space_permissions_updated_at ON space_permissions;
CREATE TRIGGER update_space_permissions_updated_at BEFORE UPDATE ON space_permissions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_org_invitations_updated_at ON organization_invitations;
CREATE TRIGGER update_org_invitations_updated_at BEFORE UPDATE ON organization_invitations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================
-- Collections schema
-- =============================

CREATE TABLE IF NOT EXISTS collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(20),
    icon VARCHAR(50),
    position INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collections_space ON collections(space_id);
CREATE INDEX IF NOT EXISTS idx_collections_space_pos ON collections(space_id, position);

DROP TRIGGER IF EXISTS update_collections_updated_at ON collections;
CREATE TRIGGER update_collections_updated_at BEFORE UPDATE ON collections FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Soft delete columns (idempotent)
ALTER TABLE IF EXISTS spaces ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE NULL;
ALTER TABLE IF EXISTS collections ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE NULL;

-- Helpful indexes for soft delete filtering
CREATE INDEX IF NOT EXISTS idx_spaces_org_deleted ON spaces(organization_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_collections_space_deleted ON collections(space_id, deleted_at);

-- =============================
-- Collection Items schema
-- =============================

CREATE TABLE IF NOT EXISTS collection_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    title VARCHAR(512) NOT NULL,
    url TEXT,
    fav_icon_url TEXT,
    original_title VARCHAR(512),
    ai_generated_title VARCHAR(512),
    domain VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    position INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE NULL
);

CREATE INDEX IF NOT EXISTS idx_items_collection ON collection_items(collection_id);
CREATE INDEX IF NOT EXISTS idx_items_collection_pos ON collection_items(collection_id, position);
CREATE INDEX IF NOT EXISTS idx_items_collection_deleted ON collection_items(collection_id, deleted_at);

DROP TRIGGER IF EXISTS update_collection_items_updated_at ON collection_items;
CREATE TRIGGER update_collection_items_updated_at BEFORE UPDATE ON collection_items FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
