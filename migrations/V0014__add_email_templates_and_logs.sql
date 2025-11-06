-- Create ENUMs
CREATE TYPE email_template_type AS ENUM ('verification', 'welcome', 'position_update', 'reward_earned', 'milestone', 'custom');
CREATE TYPE email_log_status AS ENUM ('pending', 'sent', 'delivered', 'opened', 'clicked', 'bounced', 'failed');

-- Email Templates Table
CREATE TABLE email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    type email_template_type NOT NULL,
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

-- Email Logs Table
CREATE TABLE email_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id UUID REFERENCES waitlist_users(id) ON DELETE SET NULL,
    template_id UUID REFERENCES email_templates(id) ON DELETE SET NULL,

    recipient_email VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    type email_template_type NOT NULL,

    status email_log_status NOT NULL DEFAULT 'pending',

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
