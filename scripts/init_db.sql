-- 初始化数据库脚本
-- 用于创建基础表结构和测试数据

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
INSERT INTO users (id, email, name, tier, created_at, updated_at) 
VALUES (
    'test-user-123', 
    'test@example.com', 
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
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

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
