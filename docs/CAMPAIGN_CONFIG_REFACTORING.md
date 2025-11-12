# Campaign Configuration Refactoring Documentation

## Overview

This document describes the refactoring of campaign JSONB configurations into normalized database tables. This refactoring improves queryability, data integrity, type safety, and maintainability.

---

## What Changed

### Database Schema

**Before:**
```sql
CREATE TABLE campaigns (
    ...
    form_config JSONB,
    referral_config JSONB,
    email_config JSONB,
    branding_config JSONB,
    ...
);
```

**After:**
```sql
CREATE TABLE campaigns (
    ... (no config JSONB columns)
);

CREATE TABLE campaign_form_configs (
    id UUID PRIMARY KEY,
    campaign_id UUID UNIQUE REFERENCES campaigns(id),
    captcha_enabled BOOLEAN,
    double_opt_in BOOLEAN,
    custom_css TEXT,
    ...
);

CREATE TABLE campaign_form_fields (
    id UUID PRIMARY KEY,
    form_config_id UUID REFERENCES campaign_form_configs(id),
    name VARCHAR(255),
    type form_field_type,
    label VARCHAR(255),
    required BOOLEAN,
    display_order INTEGER,
    ...
);

CREATE TABLE campaign_referral_configs (...);
CREATE TABLE campaign_email_configs (...);
CREATE TABLE campaign_branding_configs (...);
```

### Go Code Changes

**New Files Created:**
- `/internal/store/campaign_config.go` - CRUD operations for all config tables
- `/internal/campaign/processor/processor_v2.go` - Updated processor with typed configs
- `/internal/campaign/handler/handler_v2.go` - Updated handlers with typed configs

**Modified Files:**
- `/internal/store/models.go` - Updated Campaign struct and added new config structs
- `/internal/store/campaign.go` - Updated all queries to remove JSONB columns

---

## Migration Steps

### Step 1: Run Database Migration

```bash
# Build migration container
docker build -t flyway-migrate -f dbmigrator.dockerfile .

# Run migration V0019
docker run --platform linux/amd64 --rm \
  -e DB_HOST=$DB_HOST \
  -e DB_USERNAME=$DB_USERNAME \
  -e DB_PASSWORD=$DB_PASSWORD \
  flyway-migrate
```

**What it does:**
1. Creates 5 new tables (form_configs, form_fields, referral_configs, email_configs, branding_configs)
2. Migrates all existing JSONB data to normalized tables
3. Marks old JSONB columns as DEPRECATED (but keeps them for rollback safety)

### Step 2: Update Application Code

**Option A: Gradual Migration (Recommended)**

Keep both old and new endpoints running simultaneously:

1. Deploy code with both `processor.go` and `processor_v2.go`
2. Add new routes alongside old ones:
   ```go
   // Old routes (still work)
   apiGroup.POST("/campaigns", handler.HandleCreateCampaign)
   apiGroup.PUT("/campaigns/:id", handler.HandleUpdateCampaign)

   // New routes (use normalized configs)
   apiGroup.POST("/v2/campaigns", handler.HandleCreateCampaignV2)
   apiGroup.PUT("/v2/campaigns/:id", handler.HandleUpdateCampaignV2)
   ```
3. Update frontend to use `/v2/` endpoints
4. After verifying, remove old endpoints

**Option B: Big Bang Migration**

Replace all at once:
1. Remove old handler methods
2. Rename `HandleCreateCampaignV2` → `HandleCreateCampaign`
3. Update all routes
4. Deploy

### Step 3: Update Frontend/API Clients

**Old API Format:**
```json
{
  "name": "My Campaign",
  "slug": "my-campaign",
  "form_config": {
    "fields": [
      {
        "name": "email",
        "type": "email",
        "label": "Email",
        "required": true
      }
    ],
    "captcha_enabled": true,
    "double_opt_in": true
  },
  "referral_config": {
    "enabled": true,
    "points_per_referral": 1
  },
  "email_config": {
    "from_email": "hello@example.com"
  },
  "branding_config": {
    "primary_color": "#2563EB"
  }
}
```

**New API Format (Same Structure!):**
The JSON structure remains **exactly the same**, but the backend now stores it in normalized tables instead of JSONB columns. No frontend changes required for data format.

---

## Benefits

### 1. **Query Performance**
```sql
-- Before: Slow JSON querying
SELECT * FROM campaigns WHERE form_config->>'captcha_enabled' = 'true';

-- After: Fast indexed queries
SELECT c.*
FROM campaigns c
JOIN campaign_form_configs fc ON fc.campaign_id = c.id
WHERE fc.captcha_enabled = true;
```

### 2. **Data Integrity**
- **Before:** No validation, any JSON structure allowed
- **After:** Database constraints enforce valid data:
  - `points_per_referral > 0`
  - `primary_color` must match hex color pattern
  - Foreign key constraints prevent orphaned configs

### 3. **Type Safety**
```go
// Before: Untyped map
formConfig := campaign.FormConfig // map[string]interface{}
captchaEnabled := formConfig["captcha_enabled"].(bool) // Runtime panic risk

// After: Strongly typed
formConfig := campaign.FormConfig // *CampaignFormConfig
captchaEnabled := formConfig.CaptchaEnabled // bool, safe
```

### 4. **Complex Form Validation**
Form fields are now separate entities that can be:
- Queried individually
- Validated with proper types
- Reordered without JSON manipulation
- Extended with additional metadata

### 5. **Simplified Queries**
```go
// Find all campaigns with captcha enabled
campaigns, err := store.GetCampaignsWithCaptcha(ctx)

// Find campaigns by referral points
campaigns, err := store.GetCampaignsByReferralPoints(ctx, minPoints)
```

---

## API Examples

### Create Campaign

**Endpoint:** `POST /api/campaigns` (v2)

**Request:**
```json
{
  "name": "Product Launch Waitlist",
  "slug": "product-launch-2025",
  "type": "waitlist",
  "description": "Sign up for early access",
  "form_config": {
    "captcha_enabled": true,
    "double_opt_in": true,
    "custom_css": ".form { border: 1px solid #ccc; }",
    "fields": [
      {
        "name": "email",
        "type": "email",
        "label": "Email Address",
        "placeholder": "you@example.com",
        "required": true,
        "display_order": 0
      },
      {
        "name": "company",
        "type": "text",
        "label": "Company Name",
        "required": false,
        "display_order": 1
      }
    ]
  },
  "referral_config": {
    "enabled": true,
    "points_per_referral": 5,
    "verified_only": true,
    "sharing_channels": ["email", "twitter", "linkedin"],
    "custom_share_messages": {
      "twitter": "Check out this amazing product! {{referral_link}}"
    }
  },
  "email_config": {
    "from_name": "Product Team",
    "from_email": "product@example.com",
    "reply_to": "support@example.com",
    "verification_required": true
  },
  "branding_config": {
    "logo_url": "https://example.com/logo.png",
    "primary_color": "#FF6B6B",
    "font_family": "Helvetica",
    "custom_domain": "waitlist.example.com"
  },
  "privacy_policy_url": "https://example.com/privacy",
  "terms_url": "https://example.com/terms",
  "max_signups": 10000
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "account_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "Product Launch Waitlist",
  "slug": "product-launch-2025",
  "status": "draft",
  "type": "waitlist",
  "form_config": {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "campaign_id": "550e8400-e29b-41d4-a716-446655440000",
    "captcha_enabled": true,
    "double_opt_in": true,
    "custom_css": ".form { border: 1px solid #ccc; }",
    "fields": [
      {
        "id": "880e8400-e29b-41d4-a716-446655440000",
        "name": "email",
        "type": "email",
        "label": "Email Address",
        "placeholder": "you@example.com",
        "required": true,
        "display_order": 0
      },
      {
        "id": "990e8400-e29b-41d4-a716-446655440000",
        "name": "company",
        "type": "text",
        "label": "Company Name",
        "required": false,
        "display_order": 1
      }
    ]
  },
  "referral_config": { ... },
  "email_config": { ... },
  "branding_config": { ... },
  "total_signups": 0,
  "total_verified": 0,
  "total_referrals": 0,
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:00:00Z"
}
```

### Update Campaign

**Endpoint:** `PUT /api/campaigns/:id` (v2)

**Request (Partial Update):**
```json
{
  "name": "Updated Campaign Name",
  "form_config": {
    "captcha_enabled": false
  },
  "branding_config": {
    "primary_color": "#00FF00"
  }
}
```

Only provided fields are updated. Configs can be updated independently.

---

## Querying Examples

### Store Layer

```go
// Get campaign with all configs
campaign, err := store.GetCampaignByID(ctx, campaignID)
// campaign.FormConfig, campaign.ReferralConfig, etc. are loaded automatically

// Update only form config
err := store.UpdateCampaignFormConfig(ctx, campaignID, store.UpdateCampaignFormConfigParams{
    CaptchaEnabled: &trueBool,
})

// Get all campaigns with captcha enabled
formConfigs, err := db.Query(`
    SELECT c.*
    FROM campaigns c
    JOIN campaign_form_configs fc ON fc.campaign_id = c.id
    WHERE fc.captcha_enabled = true
`)

// Get campaigns by referral points
referralConfigs, err := db.Query(`
    SELECT c.*
    FROM campaigns c
    JOIN campaign_referral_configs rc ON rc.campaign_id = c.id
    WHERE rc.points_per_referral >= $1
`, minPoints)
```

---

## Testing

### Unit Tests

Update test fixtures:

```go
// Before
campaign := store.CreateCampaignParams{
    FormConfig: store.JSONB{"captcha_enabled": true},
}

// After
campaign := store.CreateCampaignParams{
    FormConfig: store.CreateCampaignFormConfigParams{
        CaptchaEnabled: true,
        DoubleOptIn: true,
        Fields: []store.CreateFormFieldParams{
            {
                Name: "email",
                Type: "email",
                Label: "Email",
                Required: true,
            },
        },
    },
}
```

### Integration Tests

```bash
# Run all tests
go test ./...

# Run campaign-specific tests
go test ./internal/store -run TestCampaign
go test ./internal/campaign/processor -run TestCampaign
go test ./tests -run TestCampaign
```

### Manual Testing

1. Create a campaign via API
2. Verify data in new tables:
   ```sql
   SELECT * FROM campaign_form_configs WHERE campaign_id = 'xxx';
   SELECT * FROM campaign_form_fields WHERE form_config_id = 'xxx';
   ```
3. Update campaign configs
4. Query campaigns by config values
5. Delete campaign and verify CASCADE deletes

---

## Rollback Plan

If issues arise, rollback is straightforward:

### Option 1: Database Rollback
```sql
-- The old JSONB columns are still present (marked as DEPRECATED)
-- They contain the original data and can be used immediately

-- Revert routes to use old handlers
-- No data migration needed - old columns still have data
```

### Option 2: Forward Fix
```sql
-- If new tables have issues, repopulate JSONB from normalized tables
UPDATE campaigns c
SET form_config = (
    SELECT jsonb_build_object(
        'captcha_enabled', fc.captcha_enabled,
        'double_opt_in', fc.double_opt_in,
        'custom_css', fc.custom_css,
        'fields', (
            SELECT jsonb_agg(jsonb_build_object(
                'name', ff.name,
                'type', ff.type,
                'label', ff.label,
                'required', ff.required
            ) ORDER BY ff.display_order)
            FROM campaign_form_fields ff
            WHERE ff.form_config_id = fc.id
        )
    )
    FROM campaign_form_configs fc
    WHERE fc.campaign_id = c.id
);
```

---

## Future Enhancements

With normalized configs, these features become easy to implement:

1. **Config Templates**: Reusable form/referral/branding templates
2. **Config Versioning**: Track changes over time
3. **A/B Testing**: Different configs for the same campaign
4. **Advanced Queries**: Complex filtering and reporting
5. **Form Builder UI**: Visual editor for form fields
6. **Field Validation Rules**: Complex validation without JSONB parsing

---

## Performance Considerations

### N+1 Query Issue

The current implementation loads configs one at a time. For list endpoints with many campaigns, this could be slow.

**Current (may be slow for 100+ campaigns):**
```go
for i := range campaigns {
    store.LoadCampaignConfigs(ctx, &campaigns[i]) // N queries
}
```

**Future Optimization (batch loading):**
```go
campaignIDs := extractIDs(campaigns)
formConfigs, _ := store.GetFormConfigsByCampaignIDs(ctx, campaignIDs)
referralConfigs, _ := store.GetReferralConfigsByCampaignIDs(ctx, campaignIDs)
// ... attach to campaigns
```

For now, pagination limits (20 per page) keep this manageable.

### Index Usage

The migration creates indexes on frequently-queried fields:
- `campaign_form_configs(captcha_enabled)` - Partial index WHERE TRUE
- `campaign_referral_configs(points_per_referral)` - B-tree index
- `campaign_branding_configs(custom_domain)` - Partial index WHERE NOT NULL

Monitor query performance and add indexes as needed.

---

## Deployment Checklist

- [ ] Review migration SQL (`V0019__refactor_campaign_configs_to_normalized_tables.sql`)
- [ ] Test migration on staging database
- [ ] Verify data integrity after migration
- [ ] Deploy application code with new handlers
- [ ] Update API documentation
- [ ] Update frontend/client code (if needed)
- [ ] Monitor error logs for issues
- [ ] Run performance tests on list endpoints
- [ ] After 1 week of stability, consider dropping old JSONB columns

---

## Support

For questions or issues:
1. Check existing campaigns work correctly with GET endpoints
2. Test create/update with new format
3. Review logs for any store-level errors
4. Check database for orphaned config records
5. Verify CASCADE deletes work correctly

---

## Summary

This refactoring transforms loosely-typed JSONB configurations into a properly normalized, type-safe schema that supports:
- ✅ Fast querying and filtering
- ✅ Data integrity constraints
- ✅ Type safety in Go code
- ✅ Complex form field validation
- ✅ Future feature extensibility

The migration is backwards-compatible and includes a rollback plan for safety.
