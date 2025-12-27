-- Create ENUMs for segments and email blasts
CREATE TYPE segment_status AS ENUM ('active', 'archived');
CREATE TYPE email_blast_status AS ENUM ('draft', 'scheduled', 'processing', 'sending', 'completed', 'paused', 'cancelled', 'failed');
CREATE TYPE blast_recipient_status AS ENUM ('pending', 'queued', 'sending', 'sent', 'delivered', 'opened', 'clicked', 'bounced', 'failed');

-- Segments Table
-- Stores reusable filter criteria for targeting waitlist users
CREATE TABLE segments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,

    -- Basic info
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Filter criteria stored as JSONB for flexibility
    -- Structure: {
    --   "statuses": ["pending", "verified"],
    --   "sources": ["direct", "referral"],
    --   "email_verified": true,
    --   "has_referrals": true,
    --   "min_referrals": 5,
    --   "min_position": 1,
    --   "max_position": 100,
    --   "date_from": "2024-01-01T00:00:00Z",
    --   "date_to": "2024-12-31T23:59:59Z",
    --   "custom_fields": {"company": "Acme"}
    -- }
    filter_criteria JSONB NOT NULL DEFAULT '{}',

    -- Cached count (refreshed on demand)
    cached_user_count INTEGER NOT NULL DEFAULT 0,
    cached_at TIMESTAMPTZ,

    status segment_status NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_segments_campaign ON segments(campaign_id);
CREATE INDEX idx_segments_status ON segments(campaign_id, status) WHERE deleted_at IS NULL;

-- Email Blasts Table
-- Stores email blast campaigns sent to segments
CREATE TABLE email_blasts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    segment_id UUID NOT NULL REFERENCES segments(id) ON DELETE RESTRICT,
    template_id UUID NOT NULL REFERENCES email_templates(id) ON DELETE RESTRICT,

    -- Basic info
    name VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,

    -- Scheduling
    scheduled_at TIMESTAMPTZ,  -- NULL = immediate send
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Status tracking
    status email_blast_status NOT NULL DEFAULT 'draft',

    -- Progress tracking
    total_recipients INTEGER NOT NULL DEFAULT 0,
    sent_count INTEGER NOT NULL DEFAULT 0,
    delivered_count INTEGER NOT NULL DEFAULT 0,
    opened_count INTEGER NOT NULL DEFAULT 0,
    clicked_count INTEGER NOT NULL DEFAULT 0,
    bounced_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,

    -- Processing metadata
    batch_size INTEGER NOT NULL DEFAULT 100,
    current_batch INTEGER NOT NULL DEFAULT 0,
    last_batch_at TIMESTAMPTZ,
    error_message TEXT,

    -- Rate limiting (emails per second)
    send_throttle_per_second INTEGER,

    -- Audit
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_email_blasts_campaign ON email_blasts(campaign_id);
CREATE INDEX idx_email_blasts_segment ON email_blasts(segment_id);
CREATE INDEX idx_email_blasts_status ON email_blasts(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_email_blasts_scheduled ON email_blasts(scheduled_at) WHERE status = 'scheduled' AND deleted_at IS NULL;

-- Blast Recipients Table
-- Tracks individual recipients and their status within a blast
CREATE TABLE blast_recipients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    blast_id UUID NOT NULL REFERENCES email_blasts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES waitlist_users(id) ON DELETE CASCADE,

    -- Denormalized for performance
    email VARCHAR(255) NOT NULL,

    status blast_recipient_status NOT NULL DEFAULT 'pending',

    -- Email log reference (once email is sent)
    email_log_id UUID REFERENCES email_logs(id) ON DELETE SET NULL,

    -- Tracking timestamps
    queued_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    clicked_at TIMESTAMPTZ,
    bounced_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,

    error_message TEXT,

    -- Batch processing
    batch_number INTEGER,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(blast_id, user_id)
);

CREATE INDEX idx_blast_recipients_blast ON blast_recipients(blast_id);
CREATE INDEX idx_blast_recipients_status ON blast_recipients(blast_id, status);
CREATE INDEX idx_blast_recipients_batch ON blast_recipients(blast_id, batch_number) WHERE status = 'pending';
CREATE INDEX idx_blast_recipients_user ON blast_recipients(user_id);

-- Add blast_id reference to email_logs for tracking blast emails
ALTER TABLE email_logs ADD COLUMN blast_id UUID REFERENCES email_blasts(id) ON DELETE SET NULL;
CREATE INDEX idx_email_logs_blast ON email_logs(blast_id) WHERE blast_id IS NOT NULL;
