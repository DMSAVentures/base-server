-- Create ENUMs
CREATE TYPE account_plan AS ENUM ('free', 'starter', 'pro', 'enterprise');
CREATE TYPE account_status AS ENUM ('active', 'suspended', 'canceled');
CREATE TYPE team_member_role AS ENUM ('owner', 'admin', 'editor', 'viewer');

-- Accounts Table
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    plan account_plan NOT NULL DEFAULT 'free',
    status account_status NOT NULL DEFAULT 'active',
    stripe_customer_id VARCHAR(255),
    trial_ends_at TIMESTAMPTZ,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_accounts_owner ON accounts(owner_user_id);
CREATE INDEX idx_accounts_slug ON accounts(slug);
CREATE INDEX idx_accounts_stripe ON accounts(stripe_customer_id);

-- Team Members Table
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role team_member_role NOT NULL,
    permissions JSONB DEFAULT '{}',
    invited_by UUID REFERENCES users(id),
    invited_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, user_id)
);

CREATE INDEX idx_team_members_account ON team_members(account_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);
