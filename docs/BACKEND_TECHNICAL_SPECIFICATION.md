# Backend Technical Specification
## Viral Waitlist & Referral Marketing Platform

**Version:** 1.0
**Last Updated:** November 5, 2025
**Status:** Architecture Design

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [Database Schema](#2-database-schema)
3. [Data Models (Go Structs)](#3-data-models-go-structs)
4. [API Specification](#4-api-specification)
5. [Background Jobs & Queue System](#5-background-jobs--queue-system)
6. [Email System Architecture](#6-email-system-architecture)
7. [Webhook System](#7-webhook-system)
8. [Analytics & Reporting](#8-analytics--reporting)
9. [Security Implementation](#9-security-implementation)
10. [Performance & Scalability](#10-performance--scalability)

---

## 1. System Architecture

### 1.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLIENT LAYER                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐ │
│  │ Web App  │  │ Embed SDK│  │  Mobile  │  │ Third-Party│ │
│  │(React/TS)│  │   (JS)   │  │   Web    │  │  Services  │ │
│  └──────────┘  └──────────┘  └──────────┘  └────────────┘ │
└─────────────────────────────────────────────────────────────┘
                            │
                    ┌───────▼────────┐
                    │   CDN/Edge     │
                    │  (CloudFlare)  │
                    └───────┬────────┘
                            │
                    ┌───────▼────────┐
                    │  API Gateway   │
                    │ Rate Limiting  │
                    │ Auth / CORS    │
                    └───────┬────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
   ┌────▼─────┐      ┌─────▼──────┐     ┌─────▼────────┐
   │   API    │      │   WebSocket│     │   Webhook    │
   │  Service │      │   Service  │     │   Service    │
   │(Go/Gin)  │      │  (Go/Gin)  │     │   (Go)       │
   └────┬─────┘      └─────┬──────┘     └─────┬────────┘
        │                  │                   │
        └──────────────────┼───────────────────┘
                           │
        ┌──────────────────┼───────────────────┐
        │                  │                   │
   ┌────▼─────┐      ┌─────▼──────┐     ┌─────▼────────┐
   │PostgreSQL│      │   Redis    │     │   BullMQ     │
   │ Primary  │      │Cache/Queue │     │ Job Workers  │
   └──────────┘      └────────────┘     └──────────────┘
        │
   ┌────▼─────┐      ┌────────────┐     ┌──────────────┐
   │TimescaleDB│     │     S3     │     │  SendGrid    │
   │ Analytics │     │   Assets   │     │    Email     │
   └───────────┘     └────────────┘     └──────────────┘
```

### 1.2 Service Breakdown

#### Core Services

**API Service** (Port 8080)
- Campaign management
- User/waitlist management
- Referral tracking
- Reward processing
- Analytics queries

**WebSocket Service** (Port 8081)
- Real-time position updates
- Live dashboard metrics
- Team collaboration

**Webhook Delivery Service**
- Event dispatching
- Retry logic with exponential backoff
- Delivery tracking

**Background Job Workers**
- Email sending
- Position recalculation
- Reward fulfillment
- Analytics aggregation
- Fraud detection

### 1.3 Technology Stack

**Backend:**
- Language: Go 1.21+
- Framework: Gin (HTTP routing)
- ORM: sqlx (raw SQL for performance)
- Queue: BullMQ / Asynq (Redis-backed)

**Data Storage:**
- Primary DB: PostgreSQL 15+
- Cache: Redis 7+
- Analytics: TimescaleDB (PostgreSQL extension)
- Object Storage: AWS S3 / CloudFlare R2

**External Services:**
- Email: SendGrid / Mailgun
- Email Validation: ZeroBounce / Kickbox
- Payments: Stripe
- CDN: CloudFlare

---

## 2. Database Schema

### 2.1 Core Tables

#### Accounts Table
```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    plan VARCHAR(50) NOT NULL DEFAULT 'free', -- free, starter, pro, enterprise
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, suspended, canceled
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
```

#### Team Members Table
```sql
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL, -- owner, admin, editor, viewer
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
```

#### Campaigns Table
```sql
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'draft', -- draft, active, paused, completed

    -- Campaign type
    type VARCHAR(50) NOT NULL DEFAULT 'waitlist', -- waitlist, referral, contest

    -- Launch settings
    launch_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,

    -- Form settings
    form_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "fields": [{"name": "email", "type": "email", "required": true, "label": "Email"}],
    --   "captcha_enabled": true,
    --   "double_opt_in": true,
    --   "custom_css": ""
    -- }

    -- Referral settings
    referral_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "enabled": true,
    --   "points_per_referral": 1,
    --   "verified_only": true,
    --   "sharing_channels": ["email", "twitter", "facebook", "linkedin", "whatsapp"],
    --   "custom_share_messages": {}
    -- }

    -- Email settings
    email_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "from_name": "Company Name",
    --   "from_email": "hello@example.com",
    --   "reply_to": "support@example.com",
    --   "verification_required": true
    -- }

    -- Branding
    branding_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "logo_url": "",
    --   "primary_color": "#2563EB",
    --   "font_family": "Inter",
    --   "custom_domain": ""
    -- }

    -- Privacy & legal
    privacy_policy_url VARCHAR(500),
    terms_url VARCHAR(500),

    -- Limits
    max_signups INTEGER,

    -- Stats (denormalized for performance)
    total_signups INTEGER NOT NULL DEFAULT 0,
    total_verified INTEGER NOT NULL DEFAULT 0,
    total_referrals INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    UNIQUE(account_id, slug)
);

CREATE INDEX idx_campaigns_account ON campaigns(account_id);
CREATE INDEX idx_campaigns_slug ON campaigns(account_id, slug);
CREATE INDEX idx_campaigns_status ON campaigns(status);
CREATE INDEX idx_campaigns_launch_date ON campaigns(launch_date);
```

#### Waitlist Users Table
```sql
CREATE TABLE waitlist_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Basic info
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, verified, converted, removed, blocked

    -- Position tracking
    position INTEGER NOT NULL,
    original_position INTEGER NOT NULL,

    -- Referral tracking
    referral_code VARCHAR(50) UNIQUE NOT NULL,
    referred_by_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,
    referral_count INTEGER NOT NULL DEFAULT 0,
    verified_referral_count INTEGER NOT NULL DEFAULT 0,

    -- Points & rewards
    points INTEGER NOT NULL DEFAULT 0,

    -- Verification
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    verification_token VARCHAR(255),
    verification_sent_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,

    -- Source tracking
    source VARCHAR(100), -- direct, referral, social, ad
    utm_source VARCHAR(255),
    utm_medium VARCHAR(255),
    utm_campaign VARCHAR(255),
    utm_term VARCHAR(255),
    utm_content VARCHAR(255),

    -- Device & location
    ip_address INET,
    user_agent TEXT,
    country_code VARCHAR(2),
    city VARCHAR(100),
    device_fingerprint VARCHAR(255),

    -- Custom fields (flexible)
    metadata JSONB DEFAULT '{}',

    -- Consent tracking
    marketing_consent BOOLEAN DEFAULT FALSE,
    marketing_consent_at TIMESTAMPTZ,
    terms_accepted BOOLEAN DEFAULT FALSE,
    terms_accepted_at TIMESTAMPTZ,

    -- Engagement
    last_activity_at TIMESTAMPTZ,
    share_count INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    UNIQUE(campaign_id, email)
);

CREATE INDEX idx_waitlist_users_campaign ON waitlist_users(campaign_id);
CREATE INDEX idx_waitlist_users_email ON waitlist_users(email);
CREATE INDEX idx_waitlist_users_position ON waitlist_users(campaign_id, position);
CREATE INDEX idx_waitlist_users_referral_code ON waitlist_users(referral_code);
CREATE INDEX idx_waitlist_users_referred_by ON waitlist_users(referred_by_id);
CREATE INDEX idx_waitlist_users_status ON waitlist_users(campaign_id, status);
CREATE INDEX idx_waitlist_users_verified ON waitlist_users(campaign_id, email_verified);
CREATE INDEX idx_waitlist_users_created ON waitlist_users(campaign_id, created_at DESC);
```

#### Referrals Table
```sql
CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    referrer_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,
    referred_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,

    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, verified, converted, invalid

    -- Tracking
    source VARCHAR(100), -- email, twitter, facebook, linkedin, whatsapp, direct
    ip_address INET,

    verified_at TIMESTAMPTZ,
    converted_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(referrer_id, referred_id)
);

CREATE INDEX idx_referrals_campaign ON referrals(campaign_id);
CREATE INDEX idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX idx_referrals_referred ON referrals(referred_id);
CREATE INDEX idx_referrals_status ON referrals(status);
CREATE INDEX idx_referrals_created ON referrals(created_at DESC);
```

#### Rewards Table
```sql
CREATE TABLE rewards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(100) NOT NULL, -- early_access, discount, premium_feature, merchandise, custom

    -- Reward configuration
    config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "value": "20% off",
    --   "code_template": "EARLY{random}",
    --   "instructions": "Use this code at checkout",
    --   "expiry_days": 30
    -- }

    -- Trigger configuration
    trigger_type VARCHAR(50) NOT NULL, -- referral_count, position, milestone, manual
    trigger_config JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "referral_count": 5,
    --   "verified_only": true
    -- }

    -- Delivery configuration
    delivery_method VARCHAR(50) NOT NULL, -- email, webhook, manual
    delivery_config JSONB NOT NULL DEFAULT '{}',

    -- Limits
    total_available INTEGER,
    total_claimed INTEGER NOT NULL DEFAULT 0,
    user_limit INTEGER DEFAULT 1, -- how many times one user can earn this

    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, paused, expired

    -- Scheduling
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_rewards_campaign ON rewards(campaign_id);
CREATE INDEX idx_rewards_status ON rewards(status);
CREATE INDEX idx_rewards_type ON rewards(type);
```

#### User Rewards Table
```sql
CREATE TABLE user_rewards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,
    reward_id UUID NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, earned, delivered, redeemed, revoked, expired

    -- Reward details (snapshot at time of earning)
    reward_data JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "code": "EARLY2024ABC",
    --   "value": "20% off",
    --   "instructions": "..."
    -- }

    -- Lifecycle timestamps
    earned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ,
    redeemed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    -- Delivery tracking
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    last_delivery_attempt_at TIMESTAMPTZ,
    delivery_error TEXT,

    -- Revocation
    revoked_reason TEXT,
    revoked_by UUID REFERENCES users(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_rewards_user ON user_rewards(user_id);
CREATE INDEX idx_user_rewards_reward ON user_rewards(reward_id);
CREATE INDEX idx_user_rewards_campaign ON user_rewards(campaign_id);
CREATE INDEX idx_user_rewards_status ON user_rewards(status);
CREATE INDEX idx_user_rewards_earned ON user_rewards(earned_at DESC);
```

#### Email Templates Table
```sql
CREATE TABLE email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL, -- verification, welcome, position_update, reward_earned, milestone, custom
    subject VARCHAR(255) NOT NULL,

    -- Email content
    html_body TEXT NOT NULL,
    text_body TEXT NOT NULL,

    -- Template variables available:
    -- {{first_name}}, {{last_name}}, {{email}}, {{position}}, {{referral_link}},
    -- {{referral_count}}, {{reward_details}}, {{company_name}}, etc.

    -- Settings
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    send_automatically BOOLEAN NOT NULL DEFAULT TRUE,

    -- A/B testing
    variant_name VARCHAR(100),
    variant_weight INTEGER DEFAULT 100,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_email_templates_campaign ON email_templates(campaign_id);
CREATE INDEX idx_email_templates_type ON email_templates(campaign_id, type);
```

#### Email Logs Table
```sql
CREATE TABLE email_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,
    template_id UUID REFERENCES email_templates(id) ON DELETE SET NULL,

    recipient_email VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,

    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, sent, delivered, opened, clicked, bounced, failed

    -- SendGrid/Mailgun message ID
    provider_message_id VARCHAR(255),

    -- Tracking
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    clicked_at TIMESTAMPTZ,
    bounced_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,

    -- Error tracking
    error_message TEXT,
    bounce_reason TEXT,

    -- Engagement
    open_count INTEGER NOT NULL DEFAULT 0,
    click_count INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_email_logs_campaign ON email_logs(campaign_id);
CREATE INDEX idx_email_logs_user ON email_logs(user_id);
CREATE INDEX idx_email_logs_status ON email_logs(status);
CREATE INDEX idx_email_logs_created ON email_logs(created_at DESC);
CREATE INDEX idx_email_logs_provider ON email_logs(provider_message_id);
```

### 2.2 Analytics Tables (TimescaleDB)

#### Campaign Analytics Table (Hypertable)
```sql
CREATE TABLE campaign_analytics (
    time TIMESTAMPTZ NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Metrics
    new_signups INTEGER NOT NULL DEFAULT 0,
    new_verified INTEGER NOT NULL DEFAULT 0,
    new_referrals INTEGER NOT NULL DEFAULT 0,
    new_conversions INTEGER NOT NULL DEFAULT 0,

    emails_sent INTEGER NOT NULL DEFAULT 0,
    emails_opened INTEGER NOT NULL DEFAULT 0,
    emails_clicked INTEGER NOT NULL DEFAULT 0,

    rewards_earned INTEGER NOT NULL DEFAULT 0,
    rewards_delivered INTEGER NOT NULL DEFAULT 0,

    -- Aggregated for quick queries
    total_signups INTEGER NOT NULL DEFAULT 0,
    total_verified INTEGER NOT NULL DEFAULT 0,
    total_referrals INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (time, campaign_id)
);

-- Convert to hypertable (TimescaleDB)
SELECT create_hypertable('campaign_analytics', 'time');

CREATE INDEX idx_campaign_analytics_campaign ON campaign_analytics(campaign_id, time DESC);
```

#### User Activity Logs Table
```sql
CREATE TABLE user_activity_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,

    event_type VARCHAR(100) NOT NULL, -- signup, verify, share, referral, reward, click, etc.
    event_data JSONB DEFAULT '{}',

    -- Context
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_activity_logs_campaign ON user_activity_logs(campaign_id, created_at DESC);
CREATE INDEX idx_user_activity_logs_user ON user_activity_logs(user_id, created_at DESC);
CREATE INDEX idx_user_activity_logs_event ON user_activity_logs(event_type, created_at DESC);
```

### 2.3 Webhook & Integration Tables

#### Webhooks Table
```sql
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE, -- NULL = account-level

    url VARCHAR(500) NOT NULL,
    secret VARCHAR(255) NOT NULL, -- For HMAC signature

    -- Event subscriptions
    events TEXT[] NOT NULL, -- ['user.created', 'user.verified', 'referral.created', etc.]

    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, paused, failed

    -- Delivery settings
    retry_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    max_retries INTEGER NOT NULL DEFAULT 5,

    -- Stats
    total_sent INTEGER NOT NULL DEFAULT 0,
    total_failed INTEGER NOT NULL DEFAULT 0,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_webhooks_account ON webhooks(account_id);
CREATE INDEX idx_webhooks_campaign ON webhooks(campaign_id);
CREATE INDEX idx_webhooks_status ON webhooks(status);
```

#### Webhook Deliveries Table
```sql
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,

    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,

    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, success, failed

    -- Request/Response
    request_headers JSONB,
    response_status INTEGER,
    response_body TEXT,
    response_headers JSONB,

    -- Timing
    duration_ms INTEGER,

    -- Retry tracking
    attempt_number INTEGER NOT NULL DEFAULT 1,
    next_retry_at TIMESTAMPTZ,

    error_message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries(status, next_retry_at) WHERE status = 'failed';
```

### 2.4 Security & Audit Tables

#### API Keys Table
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE, -- SHA-256 hash of the key
    key_prefix VARCHAR(20) NOT NULL, -- First 8 chars for identification (e.g., "sk_live_")

    -- Permissions
    scopes TEXT[] NOT NULL DEFAULT '{}', -- ['campaigns:read', 'users:write', etc.]

    -- Rate limiting
    rate_limit_tier VARCHAR(50) NOT NULL DEFAULT 'standard', -- standard, pro, enterprise

    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, revoked

    -- Usage tracking
    last_used_at TIMESTAMPTZ,
    total_requests INTEGER NOT NULL DEFAULT 0,

    expires_at TIMESTAMPTZ,

    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    revoked_by UUID REFERENCES users(id)
);

CREATE INDEX idx_api_keys_account ON api_keys(account_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_status ON api_keys(status);
```

#### Audit Logs Table
```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID REFERENCES accounts(id) ON DELETE CASCADE,

    -- Actor
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_type VARCHAR(50) NOT NULL, -- user, api_key, system
    actor_identifier VARCHAR(255), -- email, API key prefix, etc.

    -- Action
    action VARCHAR(100) NOT NULL, -- campaign.created, user.deleted, settings.updated, etc.
    resource_type VARCHAR(100) NOT NULL, -- campaign, user, reward, etc.
    resource_id UUID,

    -- Changes
    changes JSONB, -- {"field": {"old": "value", "new": "value"}}

    -- Context
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_account ON audit_logs(account_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action, created_at DESC);
```

#### Fraud Detection Table
```sql
CREATE TABLE fraud_detections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE CASCADE,

    detection_type VARCHAR(100) NOT NULL, -- self_referral, fake_email, bot, suspicious_ip, velocity, etc.
    confidence_score DECIMAL(3, 2) NOT NULL, -- 0.00 to 1.00

    details JSONB NOT NULL DEFAULT '{}',
    -- {
    --   "reason": "Same IP as referrer",
    --   "evidence": {...}
    -- }

    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, confirmed, false_positive, resolved

    -- Review
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    review_notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_fraud_detections_campaign ON fraud_detections(campaign_id, created_at DESC);
CREATE INDEX idx_fraud_detections_user ON fraud_detections(user_id);
CREATE INDEX idx_fraud_detections_status ON fraud_detections(status);
CREATE INDEX idx_fraud_detections_confidence ON fraud_detections(confidence_score DESC);
```

---

## 3. Data Models (Go Structs)

### 3.1 Core Domain Models

```go
package store

import (
    "time"
    "github.com/google/uuid"
)

// Account represents a customer account
type Account struct {
    ID               uuid.UUID  `db:"id" json:"id"`
    Name             string     `db:"name" json:"name"`
    Slug             string     `db:"slug" json:"slug"`
    OwnerUserID      uuid.UUID  `db:"owner_user_id" json:"owner_user_id"`
    Plan             string     `db:"plan" json:"plan"`
    Status           string     `db:"status" json:"status"`
    StripeCustomerID *string    `db:"stripe_customer_id" json:"stripe_customer_id,omitempty"`
    TrialEndsAt      *time.Time `db:"trial_ends_at" json:"trial_ends_at,omitempty"`
    Settings         JSONB      `db:"settings" json:"settings"`
    CreatedAt        time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
    DeletedAt        *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Campaign represents a waitlist campaign
type Campaign struct {
    ID              uuid.UUID  `db:"id" json:"id"`
    AccountID       uuid.UUID  `db:"account_id" json:"account_id"`
    Name            string     `db:"name" json:"name"`
    Slug            string     `db:"slug" json:"slug"`
    Description     *string    `db:"description" json:"description,omitempty"`
    Status          string     `db:"status" json:"status"`
    Type            string     `db:"type" json:"type"`
    LaunchDate      *time.Time `db:"launch_date" json:"launch_date,omitempty"`
    EndDate         *time.Time `db:"end_date" json:"end_date,omitempty"`

    FormConfig      FormConfig      `db:"form_config" json:"form_config"`
    ReferralConfig  ReferralConfig  `db:"referral_config" json:"referral_config"`
    EmailConfig     EmailConfig     `db:"email_config" json:"email_config"`
    BrandingConfig  BrandingConfig  `db:"branding_config" json:"branding_config"`

    PrivacyPolicyURL *string `db:"privacy_policy_url" json:"privacy_policy_url,omitempty"`
    TermsURL         *string `db:"terms_url" json:"terms_url,omitempty"`
    MaxSignups       *int    `db:"max_signups" json:"max_signups,omitempty"`

    TotalSignups   int `db:"total_signups" json:"total_signups"`
    TotalVerified  int `db:"total_verified" json:"total_verified"`
    TotalReferrals int `db:"total_referrals" json:"total_referrals"`

    CreatedAt time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
    DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// WaitlistUser represents a user on a waitlist
type WaitlistUser struct {
    ID             uuid.UUID  `db:"id" json:"id"`
    CampaignID     uuid.UUID  `db:"campaign_id" json:"campaign_id"`
    Email          string     `db:"email" json:"email"`
    FirstName      *string    `db:"first_name" json:"first_name,omitempty"`
    LastName       *string    `db:"last_name" json:"last_name,omitempty"`
    Status         string     `db:"status" json:"status"`

    Position         int `db:"position" json:"position"`
    OriginalPosition int `db:"original_position" json:"original_position"`

    ReferralCode           string     `db:"referral_code" json:"referral_code"`
    ReferredByID           *uuid.UUID `db:"referred_by_id" json:"referred_by_id,omitempty"`
    ReferralCount          int        `db:"referral_count" json:"referral_count"`
    VerifiedReferralCount  int        `db:"verified_referral_count" json:"verified_referral_count"`
    Points                 int        `db:"points" json:"points"`

    EmailVerified        bool       `db:"email_verified" json:"email_verified"`
    VerificationToken    *string    `db:"verification_token" json:"-"`
    VerificationSentAt   *time.Time `db:"verification_sent_at" json:"verification_sent_at,omitempty"`
    VerifiedAt           *time.Time `db:"verified_at" json:"verified_at,omitempty"`

    Source      *string `db:"source" json:"source,omitempty"`
    UTMSource   *string `db:"utm_source" json:"utm_source,omitempty"`
    UTMMedium   *string `db:"utm_medium" json:"utm_medium,omitempty"`
    UTMCampaign *string `db:"utm_campaign" json:"utm_campaign,omitempty"`
    UTMTerm     *string `db:"utm_term" json:"utm_term,omitempty"`
    UTMContent  *string `db:"utm_content" json:"utm_content,omitempty"`

    IPAddress         *string `db:"ip_address" json:"-"`
    UserAgent         *string `db:"user_agent" json:"-"`
    CountryCode       *string `db:"country_code" json:"country_code,omitempty"`
    City              *string `db:"city" json:"city,omitempty"`
    DeviceFingerprint *string `db:"device_fingerprint" json:"-"`

    Metadata JSONB `db:"metadata" json:"metadata,omitempty"`

    MarketingConsent   bool       `db:"marketing_consent" json:"marketing_consent"`
    MarketingConsentAt *time.Time `db:"marketing_consent_at" json:"marketing_consent_at,omitempty"`
    TermsAccepted      bool       `db:"terms_accepted" json:"terms_accepted"`
    TermsAcceptedAt    *time.Time `db:"terms_accepted_at" json:"terms_accepted_at,omitempty"`

    LastActivityAt *time.Time `db:"last_activity_at" json:"last_activity_at,omitempty"`
    ShareCount     int        `db:"share_count" json:"share_count"`

    CreatedAt time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
    DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Referral represents a referral relationship
type Referral struct {
    ID         uuid.UUID  `db:"id" json:"id"`
    CampaignID uuid.UUID  `db:"campaign_id" json:"campaign_id"`
    ReferrerID uuid.UUID  `db:"referrer_id" json:"referrer_id"`
    ReferredID uuid.UUID  `db:"referred_id" json:"referred_id"`
    Status     string     `db:"status" json:"status"`
    Source     *string    `db:"source" json:"source,omitempty"`
    IPAddress  *string    `db:"ip_address" json:"-"`

    VerifiedAt  *time.Time `db:"verified_at" json:"verified_at,omitempty"`
    ConvertedAt *time.Time `db:"converted_at" json:"converted_at,omitempty"`

    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Reward represents a reward definition
type Reward struct {
    ID          uuid.UUID  `db:"id" json:"id"`
    CampaignID  uuid.UUID  `db:"campaign_id" json:"campaign_id"`
    Name        string     `db:"name" json:"name"`
    Description *string    `db:"description" json:"description,omitempty"`
    Type        string     `db:"type" json:"type"`

    Config        RewardConfig  `db:"config" json:"config"`
    TriggerType   string        `db:"trigger_type" json:"trigger_type"`
    TriggerConfig TriggerConfig `db:"trigger_config" json:"trigger_config"`
    DeliveryMethod string       `db:"delivery_method" json:"delivery_method"`
    DeliveryConfig DeliveryConfig `db:"delivery_config" json:"delivery_config"`

    TotalAvailable *int `db:"total_available" json:"total_available,omitempty"`
    TotalClaimed   int  `db:"total_claimed" json:"total_claimed"`
    UserLimit      int  `db:"user_limit" json:"user_limit"`

    Status string `db:"status" json:"status"`

    StartsAt  *time.Time `db:"starts_at" json:"starts_at,omitempty"`
    ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`

    CreatedAt time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
    DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// UserReward represents a reward earned by a user
type UserReward struct {
    ID         uuid.UUID  `db:"id" json:"id"`
    UserID     uuid.UUID  `db:"user_id" json:"user_id"`
    RewardID   uuid.UUID  `db:"reward_id" json:"reward_id"`
    CampaignID uuid.UUID  `db:"campaign_id" json:"campaign_id"`
    Status     string     `db:"status" json:"status"`

    RewardData JSONB `db:"reward_data" json:"reward_data"`

    EarnedAt   time.Time  `db:"earned_at" json:"earned_at"`
    DeliveredAt *time.Time `db:"delivered_at" json:"delivered_at,omitempty"`
    RedeemedAt  *time.Time `db:"redeemed_at" json:"redeemed_at,omitempty"`
    RevokedAt   *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
    ExpiresAt   *time.Time `db:"expires_at" json:"expires_at,omitempty"`

    DeliveryAttempts        int        `db:"delivery_attempts" json:"delivery_attempts"`
    LastDeliveryAttemptAt   *time.Time `db:"last_delivery_attempt_at" json:"last_delivery_attempt_at,omitempty"`
    DeliveryError           *string    `db:"delivery_error" json:"delivery_error,omitempty"`

    RevokedReason *string    `db:"revoked_reason" json:"revoked_reason,omitempty"`
    RevokedBy     *uuid.UUID `db:"revoked_by" json:"revoked_by,omitempty"`

    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
```

### 3.2 Configuration Structs

```go
// FormConfig represents form configuration
type FormConfig struct {
    Fields         []FormField `json:"fields"`
    CaptchaEnabled bool        `json:"captcha_enabled"`
    DoubleOptIn    bool        `json:"double_opt_in"`
    CustomCSS      string      `json:"custom_css,omitempty"`
}

type FormField struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"` // email, text, select, checkbox, etc.
    Label       string   `json:"label"`
    Placeholder string   `json:"placeholder,omitempty"`
    Required    bool     `json:"required"`
    Options     []string `json:"options,omitempty"` // for select fields
    Validation  string   `json:"validation,omitempty"` // regex pattern
}

// ReferralConfig represents referral configuration
type ReferralConfig struct {
    Enabled            bool              `json:"enabled"`
    PointsPerReferral  int               `json:"points_per_referral"`
    VerifiedOnly       bool              `json:"verified_only"`
    SharingChannels    []string          `json:"sharing_channels"`
    CustomShareMessages map[string]string `json:"custom_share_messages"`
}

// EmailConfig represents email configuration
type EmailConfig struct {
    FromName             string `json:"from_name"`
    FromEmail            string `json:"from_email"`
    ReplyTo              string `json:"reply_to,omitempty"`
    VerificationRequired bool   `json:"verification_required"`
}

// BrandingConfig represents branding configuration
type BrandingConfig struct {
    LogoURL      string `json:"logo_url,omitempty"`
    PrimaryColor string `json:"primary_color"`
    FontFamily   string `json:"font_family"`
    CustomDomain string `json:"custom_domain,omitempty"`
}

// RewardConfig represents reward-specific configuration
type RewardConfig struct {
    Value        string `json:"value"`
    CodeTemplate string `json:"code_template,omitempty"`
    Instructions string `json:"instructions,omitempty"`
    ExpiryDays   int    `json:"expiry_days,omitempty"`
}

// TriggerConfig represents when a reward is triggered
type TriggerConfig struct {
    ReferralCount int  `json:"referral_count,omitempty"`
    VerifiedOnly  bool `json:"verified_only"`
    Position      int  `json:"position,omitempty"`
}

// DeliveryConfig represents how a reward is delivered
type DeliveryConfig struct {
    EmailTemplateID *uuid.UUID        `json:"email_template_id,omitempty"`
    WebhookURL      string            `json:"webhook_url,omitempty"`
    CustomData      map[string]string `json:"custom_data,omitempty"`
}

// JSONB is a custom type for JSONB fields
type JSONB map[string]interface{}
```

### 3.3 API Request/Response Structs

```go
package handler

// Campaign creation
type CreateCampaignRequest struct {
    Name        string  `json:"name" binding:"required,min=1,max=255"`
    Slug        string  `json:"slug" binding:"required,min=1,max=255,alphanum"`
    Description *string `json:"description"`
    Type        string  `json:"type" binding:"required,oneof=waitlist referral contest"`

    FormConfig     *FormConfig     `json:"form_config"`
    ReferralConfig *ReferralConfig `json:"referral_config"`
    EmailConfig    *EmailConfig    `json:"email_config"`
    BrandingConfig *BrandingConfig `json:"branding_config"`
}

type CreateCampaignResponse struct {
    Campaign Campaign `json:"campaign"`
}

// User signup
type SignupRequest struct {
    Email        string            `json:"email" binding:"required,email"`
    FirstName    string            `json:"first_name" binding:"required,min=1,max=100"`
    LastName     string            `json:"last_name" binding:"required,min=1,max=100"`
    ReferralCode *string           `json:"referral_code"`
    CustomFields map[string]string `json:"custom_fields"`

    // Tracking
    UTMSource   *string `json:"utm_source"`
    UTMMedium   *string `json:"utm_medium"`
    UTMCampaign *string `json:"utm_campaign"`

    // Consent
    MarketingConsent bool `json:"marketing_consent"`
    TermsAccepted    bool `json:"terms_accepted" binding:"required"`
}

type SignupResponse struct {
    User         WaitlistUser `json:"user"`
    Position     int          `json:"position"`
    ReferralLink string       `json:"referral_link"`
    Message      string       `json:"message"`
}

// Referral tracking
type TrackReferralRequest struct {
    ReferralCode string `json:"referral_code" binding:"required"`
}

// List users with filters
type ListUsersRequest struct {
    Page   int    `form:"page" binding:"min=1"`
    Limit  int    `form:"limit" binding:"min=1,max=100"`
    Status string `form:"status"`
    Sort   string `form:"sort" binding:"oneof=position created_at referral_count"`
    Order  string `form:"order" binding:"oneof=asc desc"`
}

type ListUsersResponse struct {
    Users      []WaitlistUser `json:"users"`
    TotalCount int            `json:"total_count"`
    Page       int            `json:"page"`
    PageSize   int            `json:"page_size"`
    TotalPages int            `json:"total_pages"`
}

// Analytics
type AnalyticsOverviewResponse struct {
    TotalSignups        int     `json:"total_signups"`
    TotalVerified       int     `json:"total_verified"`
    TotalReferrals      int     `json:"total_referrals"`
    VerificationRate    float64 `json:"verification_rate"`
    AverageReferrals    float64 `json:"average_referrals"`
    ViralCoefficient    float64 `json:"viral_coefficient"`
    TopReferrers        []TopReferrer `json:"top_referrers"`
    SignupsOverTime     []TimeSeriesData `json:"signups_over_time"`
    ReferralSources     []SourceBreakdown `json:"referral_sources"`
}

type TopReferrer struct {
    UserID        uuid.UUID `json:"user_id"`
    Email         string    `json:"email"`
    Name          string    `json:"name"`
    ReferralCount int       `json:"referral_count"`
}

type TimeSeriesData struct {
    Date   string `json:"date"`
    Count  int    `json:"count"`
}

type SourceBreakdown struct {
    Source string `json:"source"`
    Count  int    `json:"count"`
}
```

---

## 4. API Specification

### 4.1 Authentication

All API requests require authentication via:
- **API Key:** Header `X-API-Key: sk_live_xxxxx` or `Authorization: Bearer sk_live_xxxxx`
- **JWT Token:** For dashboard users, `Authorization: Bearer <jwt>`

API Key Format:
- Test: `sk_test_` (test mode)
- Live: `sk_live_` (production)

### 4.2 REST API Endpoints

#### Campaigns

```
POST   /api/v1/campaigns
GET    /api/v1/campaigns
GET    /api/v1/campaigns/{campaign_id}
PUT    /api/v1/campaigns/{campaign_id}
DELETE /api/v1/campaigns/{campaign_id}
PATCH  /api/v1/campaigns/{campaign_id}/status
```

#### Waitlist Users

```
POST   /api/v1/campaigns/{campaign_id}/users
GET    /api/v1/campaigns/{campaign_id}/users
GET    /api/v1/campaigns/{campaign_id}/users/{user_id}
PUT    /api/v1/campaigns/{campaign_id}/users/{user_id}
DELETE /api/v1/campaigns/{campaign_id}/users/{user_id}
POST   /api/v1/campaigns/{campaign_id}/users/search
POST   /api/v1/campaigns/{campaign_id}/users/import
POST   /api/v1/campaigns/{campaign_id}/users/export
POST   /api/v1/campaigns/{campaign_id}/users/{user_id}/verify
POST   /api/v1/campaigns/{campaign_id}/users/{user_id}/resend-verification
```

#### Referrals

```
GET    /api/v1/campaigns/{campaign_id}/referrals
POST   /api/v1/campaigns/{campaign_id}/referrals/track
GET    /api/v1/campaigns/{campaign_id}/users/{user_id}/referrals
GET    /api/v1/campaigns/{campaign_id}/users/{user_id}/referral-link
```

#### Rewards

```
POST   /api/v1/campaigns/{campaign_id}/rewards
GET    /api/v1/campaigns/{campaign_id}/rewards
GET    /api/v1/campaigns/{campaign_id}/rewards/{reward_id}
PUT    /api/v1/campaigns/{campaign_id}/rewards/{reward_id}
DELETE /api/v1/campaigns/{campaign_id}/rewards/{reward_id}
POST   /api/v1/campaigns/{campaign_id}/users/{user_id}/rewards
GET    /api/v1/campaigns/{campaign_id}/users/{user_id}/rewards
```

#### Email Templates

```
POST   /api/v1/campaigns/{campaign_id}/email-templates
GET    /api/v1/campaigns/{campaign_id}/email-templates
GET    /api/v1/campaigns/{campaign_id}/email-templates/{template_id}
PUT    /api/v1/campaigns/{campaign_id}/email-templates/{template_id}
DELETE /api/v1/campaigns/{campaign_id}/email-templates/{template_id}
POST   /api/v1/campaigns/{campaign_id}/email-templates/{template_id}/send-test
```

#### Analytics

```
GET    /api/v1/campaigns/{campaign_id}/analytics/overview
GET    /api/v1/campaigns/{campaign_id}/analytics/conversions
GET    /api/v1/campaigns/{campaign_id}/analytics/referrals
GET    /api/v1/campaigns/{campaign_id}/analytics/time-series
GET    /api/v1/campaigns/{campaign_id}/analytics/sources
GET    /api/v1/campaigns/{campaign_id}/analytics/funnel
```

#### Webhooks

```
POST   /api/v1/webhooks
GET    /api/v1/webhooks
GET    /api/v1/webhooks/{webhook_id}
PUT    /api/v1/webhooks/{webhook_id}
DELETE /api/v1/webhooks/{webhook_id}
GET    /api/v1/webhooks/{webhook_id}/deliveries
POST   /api/v1/webhooks/{webhook_id}/test
```

### 4.3 Rate Limiting

Rate limits by tier (per minute):
- **Free:** 60 requests
- **Starter:** 60 requests
- **Pro:** 180 requests
- **Enterprise:** 600+ requests (custom)

Response headers:
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1699564800
```

429 Too Many Requests response:
```json
{
  "error": "rate_limit_exceeded",
  "message": "Too many requests. Please try again in 30 seconds.",
  "retry_after": 30
}
```

### 4.4 Pagination

Cursor-based pagination for large datasets:

Request:
```
GET /api/v1/campaigns/{id}/users?limit=100&cursor=eyJpZCI6IjEyMyJ9
```

Response:
```json
{
  "data": [...],
  "pagination": {
    "next_cursor": "eyJpZCI6IjIyMyJ9",
    "has_more": true,
    "total_count": 5000
  }
}
```

### 4.5 Error Responses

Standard error format:
```json
{
  "error": {
    "code": "validation_error",
    "message": "Invalid email format",
    "details": {
      "field": "email",
      "value": "invalid-email"
    },
    "request_id": "req_abc123"
  }
}
```

Error codes:
- `validation_error` (400)
- `unauthorized` (401)
- `forbidden` (403)
- `not_found` (404)
- `conflict` (409)
- `rate_limit_exceeded` (429)
- `internal_error` (500)

---

## 5. Background Jobs & Queue System

### 5.1 Job Types

**High Priority Queue:**
- Email verification sending
- Welcome email sending
- Position update notifications
- Reward delivery

**Medium Priority Queue:**
- Analytics aggregation
- Position recalculation
- Fraud detection
- Webhook delivery

**Low Priority Queue:**
- Email open/click tracking processing
- Data exports
- Report generation

### 5.2 Job Definitions

```go
package jobs

type EmailJob struct {
    Type         string    // verification, welcome, reward, etc.
    CampaignID   uuid.UUID
    UserID       uuid.UUID
    TemplateID   uuid.UUID
    TemplateData map[string]interface{}
    Priority     int
}

type PositionRecalcJob struct {
    CampaignID uuid.UUID
    UserIDs    []uuid.UUID // specific users to recalc, or nil for all
}

type RewardDeliveryJob struct {
    UserRewardID uuid.UUID
    RetryAttempt int
}

type WebhookDeliveryJob struct {
    WebhookID  uuid.UUID
    EventType  string
    Payload    map[string]interface{}
    Attempt    int
    MaxRetries int
}

type FraudDetectionJob struct {
    CampaignID uuid.UUID
    UserID     uuid.UUID
    CheckTypes []string // self_referral, velocity, fake_email, etc.
}

type AnalyticsAggregationJob struct {
    CampaignID uuid.UUID
    StartTime  time.Time
    EndTime    time.Time
    Granularity string // hour, day, week, month
}
```

### 5.3 Job Processing

**Retry Strategy:**
- Max retries: 5
- Backoff: Exponential (2s, 4s, 8s, 16s, 32s)
- Dead letter queue for failed jobs after max retries

**Concurrency:**
- High priority: 10 workers
- Medium priority: 5 workers
- Low priority: 2 workers

---

## 6. Email System Architecture

### 6.1 Email Flow

```
User Action → Trigger Event → Job Queue → Email Worker → SendGrid API → User Inbox
                                                ↓
                                         Email Log Created
                                                ↓
                                    Webhook from SendGrid → Update Status
```

### 6.2 Email Types

1. **Verification Email**
   - Trigger: User signup
   - Template variables: `{{first_name}}`, `{{verification_link}}`, `{{campaign_name}}`

2. **Welcome Email**
   - Trigger: Email verified
   - Template variables: `{{first_name}}`, `{{position}}`, `{{referral_link}}`, `{{total_signups}}`

3. **Position Update Email**
   - Trigger: Position improved by X positions
   - Template variables: `{{first_name}}`, `{{old_position}}`, `{{new_position}}`, `{{referral_count}}`

4. **Reward Earned Email**
   - Trigger: Reward earned
   - Template variables: `{{first_name}}`, `{{reward_name}}`, `{{reward_code}}`, `{{reward_instructions}}`

5. **Milestone Email**
   - Trigger: Campaign reaches milestone (100, 1000, 10000 users)
   - Template variables: `{{first_name}}`, `{{milestone}}`, `{{position}}`

### 6.3 Email Template Engine

Template syntax: Go templates or Handlebars

Example:
```html
<h1>Welcome, {{first_name}}!</h1>
<p>You're #{{position}} on the waitlist for {{campaign_name}}.</p>
<p>Share your unique link to move up:</p>
<a href="{{referral_link}}">{{referral_link}}</a>
```

### 6.4 Unsubscribe Management

Every email includes unsubscribe link:
```
https://app.example.com/unsubscribe/{user_id}/{token}
```

Unsubscribe preferences:
- All emails
- Marketing emails only
- Keep transactional emails (verification, rewards)

---

## 7. Webhook System

### 7.1 Webhook Events

**User Events:**
- `user.created`
- `user.updated`
- `user.verified`
- `user.deleted`
- `user.position_changed`
- `user.converted`

**Referral Events:**
- `referral.created`
- `referral.verified`
- `referral.converted`

**Reward Events:**
- `reward.earned`
- `reward.delivered`
- `reward.redeemed`

**Campaign Events:**
- `campaign.milestone` (100, 1000, 10000 signups)
- `campaign.launched`
- `campaign.completed`

**Email Events:**
- `email.sent`
- `email.delivered`
- `email.opened`
- `email.clicked`
- `email.bounced`

### 7.2 Webhook Payload Format

```json
{
  "id": "evt_1a2b3c4d",
  "type": "user.created",
  "created_at": "2025-11-05T10:30:00Z",
  "data": {
    "campaign_id": "550e8400-e29b-41d4-a716-446655440000",
    "user": {
      "id": "660e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "first_name": "John",
      "last_name": "Doe",
      "position": 1234,
      "referral_code": "JOHN123",
      "referral_link": "https://example.com/join/JOHN123",
      "created_at": "2025-11-05T10:30:00Z"
    }
  },
  "account_id": "770e8400-e29b-41d4-a716-446655440000"
}
```

### 7.3 Webhook Security

**HMAC Signature Verification:**

Header: `X-Webhook-Signature`

Algorithm:
```
signature = HMAC_SHA256(webhook_secret, payload)
header_value = "t=" + timestamp + ",v1=" + signature
```

Verification (Go):
```go
func VerifyWebhookSignature(payload []byte, signature string, secret string) bool {
    parts := strings.Split(signature, ",")
    timestamp := strings.TrimPrefix(parts[0], "t=")
    providedSig := strings.TrimPrefix(parts[1], "v1=")

    signedPayload := timestamp + "." + string(payload)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(signedPayload))
    expectedSig := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(providedSig), []byte(expectedSig))
}
```

### 7.4 Webhook Retry Logic

**Retry schedule:**
- Attempt 1: Immediate
- Attempt 2: 2 seconds later
- Attempt 3: 10 seconds later
- Attempt 4: 1 minute later
- Attempt 5: 10 minutes later

**Success criteria:**
- HTTP status 200-299
- Response within 10 seconds

**Failure handling:**
- After 5 failed attempts, webhook is marked as failed
- Email notification sent to account owner
- Webhook can be manually retried from dashboard

---

## 8. Analytics & Reporting

### 8.1 Real-time Metrics (Redis)

Stored in Redis with 24-hour TTL:
```
campaign:{id}:signups:today -> 145
campaign:{id}:verified:today -> 98
campaign:{id}:referrals:today -> 67
campaign:{id}:viral_coefficient -> 1.34
```

### 8.2 Time-Series Analytics (TimescaleDB)

Aggregated hourly, daily, weekly, monthly:

**Query examples:**

Daily signups for last 30 days:
```sql
SELECT
    time_bucket('1 day', time) AS day,
    SUM(new_signups) as signups
FROM campaign_analytics
WHERE campaign_id = $1
    AND time > NOW() - INTERVAL '30 days'
GROUP BY day
ORDER BY day;
```

Viral coefficient calculation:
```sql
SELECT
    campaign_id,
    CAST(total_referrals AS FLOAT) / NULLIF(total_signups, 0) as viral_coefficient
FROM campaign_analytics
WHERE time = (SELECT MAX(time) FROM campaign_analytics WHERE campaign_id = $1)
```

### 8.3 Analytics Aggregation Job

Runs every hour:
1. Query raw data from last hour
2. Calculate metrics:
   - New signups
   - Verification rate
   - Referral rate
   - Email engagement
   - Reward delivery
3. Insert into `campaign_analytics` table
4. Update Redis cache

### 8.4 Reporting API

**Dashboard Overview:**
```
GET /api/v1/campaigns/{id}/analytics/overview
```

Response:
```json
{
  "total_signups": 5432,
  "total_verified": 4321,
  "total_referrals": 2876,
  "verification_rate": 0.7955,
  "average_referrals_per_user": 0.53,
  "viral_coefficient": 1.34,
  "growth_rate_7d": 0.23,
  "top_referrers": [...],
  "signups_by_day": [...],
  "referral_sources": [...]
}
```

---

## 9. Security Implementation

### 9.1 Position Calculation Integrity

**Algorithm:**
```go
func CalculatePosition(campaignID uuid.UUID, userID uuid.UUID) (int, error) {
    // Position = Original Position - (Verified Referral Count * Points Per Referral)
    // Then rank all users by this calculated position

    query := `
        WITH ranked_users AS (
            SELECT
                id,
                original_position - (verified_referral_count * $2) as calculated_position,
                ROW_NUMBER() OVER (ORDER BY original_position - (verified_referral_count * $2), created_at) as new_position
            FROM waitlist_users
            WHERE campaign_id = $1
                AND deleted_at IS NULL
                AND status != 'blocked'
        )
        SELECT new_position
        FROM ranked_users
        WHERE id = $3
    `

    var position int
    err := db.QueryRow(query, campaignID, pointsPerReferral, userID).Scan(&position)
    return position, err
}
```

**Recalculation triggers:**
- New user verified
- Referral verified
- User removed/blocked
- Fraud detected (bulk recalc)

### 9.2 Fraud Detection Algorithm

```go
type FraudDetector struct {
    store  Store
    logger Logger
}

func (fd *FraudDetector) CheckUser(ctx context.Context, userID uuid.UUID) ([]FraudDetection, error) {
    var detections []FraudDetection

    // Check 1: Self-referral (same email domain + similar IP)
    if fd.checkSelfReferral(ctx, userID) {
        detections = append(detections, FraudDetection{
            Type: "self_referral",
            Confidence: 0.95,
            Details: map[string]interface{}{
                "reason": "Referrer and referred user share IP address",
            },
        })
    }

    // Check 2: Fake/disposable email
    if fd.checkDisposableEmail(ctx, userID) {
        detections = append(detections, FraudDetection{
            Type: "fake_email",
            Confidence: 0.85,
        })
    }

    // Check 3: Velocity check (too many referrals too quickly)
    if fd.checkVelocity(ctx, userID) {
        detections = append(detections, FraudDetection{
            Type: "velocity",
            Confidence: 0.80,
        })
    }

    // Check 4: Bot detection (user agent, behavior patterns)
    if fd.checkBotBehavior(ctx, userID) {
        detections = append(detections, FraudDetection{
            Type: "bot",
            Confidence: 0.90,
        })
    }

    return detections, nil
}
```

### 9.3 API Key Management

**Key generation:**
```go
func GenerateAPIKey(accountID uuid.UUID, scopes []string) (string, error) {
    // Generate random 32-byte key
    randomBytes := make([]byte, 32)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", err
    }

    // Encode to base62
    encoded := base62.Encode(randomBytes)

    // Format: sk_live_1234567890abcdef
    prefix := "sk_live_"
    key := prefix + encoded

    // Store SHA-256 hash in database
    hash := sha256.Sum256([]byte(key))
    hashStr := hex.EncodeToString(hash[:])

    err := store.CreateAPIKey(APIKey{
        AccountID: accountID,
        KeyHash: hashStr,
        KeyPrefix: key[:16], // for identification
        Scopes: scopes,
    })

    // Return plaintext key only once (not stored)
    return key, err
}
```

### 9.4 Rate Limiting Implementation

**Redis-based sliding window:**
```go
func CheckRateLimit(ctx context.Context, apiKey string, tier string) (bool, error) {
    limit := GetLimitForTier(tier) // 60, 180, or 600
    window := 60 // seconds

    key := fmt.Sprintf("ratelimit:%s", apiKey)
    now := time.Now().Unix()
    windowStart := now - int64(window)

    pipe := redisClient.Pipeline()

    // Remove old entries outside window
    pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprint(windowStart))

    // Count entries in current window
    pipe.ZCard(ctx, key)

    // Add current request
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: fmt.Sprint(now, rand.Int())})

    // Set expiry
    pipe.Expire(ctx, key, time.Duration(window)*time.Second)

    results, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }

    count := results[1].(*redis.IntCmd).Val()

    return count <= int64(limit), nil
}
```

---

## 10. Performance & Scalability

### 10.1 Database Optimization

**Partitioning Strategy:**
```sql
-- Partition email_logs by month
CREATE TABLE email_logs_2025_11 PARTITION OF email_logs
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');

-- Partition user_activity_logs by campaign_id range
CREATE TABLE user_activity_logs_part1 PARTITION OF user_activity_logs
    FOR VALUES FROM ('00000000-0000-0000-0000-000000000000')
                 TO ('80000000-0000-0000-0000-000000000000');
```

**Connection Pooling:**
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(5 * time.Minute)
```

**Query Optimization:**
- Use `SELECT` with specific columns (no `SELECT *`)
- Proper indexing on foreign keys and query filters
- Use `EXPLAIN ANALYZE` for slow queries
- Materialized views for complex analytics

### 10.2 Caching Strategy

**Redis Cache Layers:**

**L1: Request-scoped cache** (5 minutes TTL)
```
campaign:{id} -> Campaign object
user:{id} -> WaitlistUser object
```

**L2: Aggregated metrics** (1 hour TTL)
```
analytics:{campaign_id}:overview -> Analytics summary
analytics:{campaign_id}:top_referrers -> Top 10 referrers
```

**L3: Rate limiting** (60 seconds sliding window)
```
ratelimit:{api_key} -> Sorted set of timestamps
```

**Cache invalidation:**
- Write-through: Update cache on every write
- Event-driven: Invalidate on specific events (user verified, campaign updated)
- TTL: All caches have expiry

### 10.3 Horizontal Scaling

**Stateless API Servers:**
- No session state stored in memory
- JWT tokens for authentication
- Redis for session storage (if needed)
- Load balancer (ALB) distributes traffic

**Database Read Replicas:**
- Primary: Writes only
- Replicas (2-3): Analytics queries, reporting, exports
- Connection pooling with read/write split

**Background Workers:**
- Separate worker pools for each queue
- Auto-scaling based on queue depth
- Kubernetes HPA (Horizontal Pod Autoscaler)

### 10.4 Performance Targets

| Metric | Target | Monitoring |
|--------|--------|------------|
| API Response Time (p95) | <200ms | DataDog APM |
| API Response Time (p99) | <500ms | DataDog APM |
| Database Query Time (p95) | <50ms | PostgreSQL logs |
| Waitlist Form Load | <1s | Real User Monitoring |
| Dashboard Load | <2s | Real User Monitoring |
| Email Delivery | <30s | Job queue metrics |
| Webhook Delivery | <5s | Job queue metrics |
| Position Calculation | <100ms | Custom metrics |
| Cache Hit Rate | >80% | Redis metrics |
| API Uptime | 99.95% | Pingdom |

### 10.5 Scaling Milestones

**Phase 1: MVP (0-10K total users)**
- Single region deployment
- 2 API servers
- 1 PostgreSQL instance (no replicas)
- 1 Redis instance
- 3 background workers

**Phase 2: Growth (10K-100K total users)**
- Multi-region deployment (US, EU)
- 4-6 API servers (auto-scaling)
- 1 Primary + 2 Read replicas
- Redis cluster (3 nodes)
- 10 background workers (auto-scaling)

**Phase 3: Scale (100K-1M total users)**
- Global CDN
- 10-20 API servers (auto-scaling)
- Database sharding by account_id
- Redis cluster (6+ nodes)
- 20+ background workers
- Elasticsearch for search
- Separate analytics database (TimescaleDB)

---

## Appendix A: Environment Variables

```bash
# Database
DB_HOST=localhost
DB_USERNAME=postgres
DB_PASSWORD=secretpassword
DB_NAME=waitlist_platform

# Redis
REDIS_HOST=localhost:6379
REDIS_PASSWORD=

# Application
SERVER_PORT=8080
WEBSOCKET_PORT=8081
GO_ENV=development # or production
WEBAPP_URI=http://localhost:3000

# JWT
JWT_SECRET=your-super-secret-jwt-key

# Email
EMAIL_PROVIDER=sendgrid # or mailgun
SENDGRID_API_KEY=SG.xxxxx
EMAIL_FROM_ADDRESS=noreply@example.com
EMAIL_FROM_NAME=Waitlist Platform

# Email Validation
EMAIL_VALIDATION_PROVIDER=zerobounce # or kickbox
ZEROBOUNCE_API_KEY=xxxxx

# Stripe
STRIPE_SECRET_KEY=sk_test_xxxxx
STRIPE_WEBHOOK_SECRET=whsec_xxxxx

# External Services
RECAPTCHA_SECRET_KEY=xxxxx

# Monitoring
DATADOG_API_KEY=xxxxx
SENTRY_DSN=https://xxxxx@sentry.io/xxxxx

# Feature Flags
ENABLE_FRAUD_DETECTION=true
ENABLE_WEBHOOKS=true
ENABLE_EMAIL_VALIDATION=true
```

---

**End of Backend Technical Specification**

**Document Version:** 1.0
**Last Updated:** November 5, 2025
**Next Review:** Monthly or on major architecture changes
