-- Migration: Seed features, limits, and plan_feature_limits
-- References existing prices by description: free, lc_pro_monthly, lc_pro_annual, lc_team_monthly, lc_team_annual

-- ============================================================================
-- FEATURES
-- ============================================================================

INSERT INTO features (name, description) VALUES
    ('email_verification', 'Verify user emails before adding to waitlist'),
    ('referral_system', 'Enable referral tracking and rewards'),
    ('visual_form_builder', 'Drag-and-drop form customization'),
    ('visual_email_builder', 'Design emails with visual editor'),
    ('all_widget_types', 'Access to all widget types'),
    ('remove_branding', 'Remove platform branding from forms'),
    ('anti_spam_protection', 'Advanced spam and bot protection'),
    ('enhanced_lead_data', 'Collect additional lead information'),
    ('tracking_pixels', 'Add Facebook, Google, and other tracking pixels'),
    ('webhooks_zapier', 'Integrate with external services'),
    ('email_blasts', 'Send bulk emails to waitlist'),
    ('json_export', 'Export data in JSON format'),
    ('campaigns', 'Number of waitlist campaigns'),
    ('leads', 'Maximum leads per account'),
    ('team_members', 'Number of team members');

-- ============================================================================
-- LIMITS (for resource features)
-- ============================================================================

-- Campaign limits
INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'campaigns_free', 1 FROM features WHERE name = 'campaigns';

-- Lead limits
INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'leads_free', 200 FROM features WHERE name = 'leads';

INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'leads_pro', 5000 FROM features WHERE name = 'leads';

INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'leads_team', 100000 FROM features WHERE name = 'leads';

-- Team member limits
INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'team_members_free', 1 FROM features WHERE name = 'team_members';

INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'team_members_pro', 1 FROM features WHERE name = 'team_members';

INSERT INTO limits (feature_id, limit_name, limit_value)
SELECT id, 'team_members_team', 5 FROM features WHERE name = 'team_members';

-- ============================================================================
-- PLAN_FEATURE_LIMITS (maps prices to features and limits)
-- ============================================================================

-- FREE TIER (description = 'free')
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT
    p.id,
    f.id,
    NULL,
    CASE f.name
        WHEN 'visual_form_builder' THEN true
        ELSE false
    END
FROM prices p
CROSS JOIN features f
WHERE p.description = 'free'
AND f.name IN ('email_verification', 'referral_system', 'visual_form_builder', 'visual_email_builder',
               'all_widget_types', 'remove_branding', 'anti_spam_protection', 'enhanced_lead_data',
               'tracking_pixels', 'webhooks_zapier', 'email_blasts', 'json_export');

-- FREE TIER - Resource limits
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'free'
AND f.name = 'campaigns' AND l.limit_name = 'campaigns_free';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'free'
AND f.name = 'leads' AND l.limit_name = 'leads_free';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'free'
AND f.name = 'team_members' AND l.limit_name = 'team_members_free';

-- PRO MONTHLY (description = 'lc_pro_monthly')
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT
    p.id,
    f.id,
    NULL,
    CASE f.name
        WHEN 'tracking_pixels' THEN false
        WHEN 'webhooks_zapier' THEN false
        WHEN 'email_blasts' THEN false
        ELSE true
    END
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_pro_monthly'
AND f.name IN ('email_verification', 'referral_system', 'visual_form_builder', 'visual_email_builder',
               'all_widget_types', 'remove_branding', 'anti_spam_protection', 'enhanced_lead_data',
               'tracking_pixels', 'webhooks_zapier', 'email_blasts', 'json_export');

-- PRO MONTHLY - Resource limits (campaigns unlimited = NULL limit_id)
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_pro_monthly'
AND f.name = 'campaigns';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_pro_monthly'
AND f.name = 'leads' AND l.limit_name = 'leads_pro';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_pro_monthly'
AND f.name = 'team_members' AND l.limit_name = 'team_members_pro';

-- PRO ANNUAL (description = 'lc_pro_annual')
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT
    p.id,
    f.id,
    NULL,
    CASE f.name
        WHEN 'tracking_pixels' THEN false
        WHEN 'webhooks_zapier' THEN false
        WHEN 'email_blasts' THEN false
        ELSE true
    END
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_pro_annual'
AND f.name IN ('email_verification', 'referral_system', 'visual_form_builder', 'visual_email_builder',
               'all_widget_types', 'remove_branding', 'anti_spam_protection', 'enhanced_lead_data',
               'tracking_pixels', 'webhooks_zapier', 'email_blasts', 'json_export');

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_pro_annual'
AND f.name = 'campaigns';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_pro_annual'
AND f.name = 'leads' AND l.limit_name = 'leads_pro';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_pro_annual'
AND f.name = 'team_members' AND l.limit_name = 'team_members_pro';

-- TEAM MONTHLY (description = 'lc_team_monthly')
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_team_monthly'
AND f.name IN ('email_verification', 'referral_system', 'visual_form_builder', 'visual_email_builder',
               'all_widget_types', 'remove_branding', 'anti_spam_protection', 'enhanced_lead_data',
               'tracking_pixels', 'webhooks_zapier', 'email_blasts', 'json_export');

-- TEAM MONTHLY - Resource limits (campaigns unlimited)
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_team_monthly'
AND f.name = 'campaigns';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_team_monthly'
AND f.name = 'leads' AND l.limit_name = 'leads_team';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_team_monthly'
AND f.name = 'team_members' AND l.limit_name = 'team_members_team';

-- TEAM ANNUAL (description = 'lc_team_annual')
INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_team_annual'
AND f.name IN ('email_verification', 'referral_system', 'visual_form_builder', 'visual_email_builder',
               'all_widget_types', 'remove_branding', 'anti_spam_protection', 'enhanced_lead_data',
               'tracking_pixels', 'webhooks_zapier', 'email_blasts', 'json_export');

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, NULL, true
FROM prices p
CROSS JOIN features f
WHERE p.description = 'lc_team_annual'
AND f.name = 'campaigns';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_team_annual'
AND f.name = 'leads' AND l.limit_name = 'leads_team';

INSERT INTO plan_feature_limits (plan_id, feature_id, limit_id, enabled)
SELECT p.id, f.id, l.id, true
FROM prices p
CROSS JOIN features f
JOIN limits l ON l.feature_id = f.id
WHERE p.description = 'lc_team_annual'
AND f.name = 'team_members' AND l.limit_name = 'team_members_team';
