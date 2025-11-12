package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ============================================================================
// Form Config Operations
// ============================================================================

// CreateCampaignFormConfigParams represents parameters for creating a form config
type CreateCampaignFormConfigParams struct {
	CampaignID     uuid.UUID
	CaptchaEnabled bool
	DoubleOptIn    bool
	CustomCSS      *string
	Fields         []CreateFormFieldParams
}

// CreateFormFieldParams represents parameters for creating a form field
type CreateFormFieldParams struct {
	Name         string
	Type         string
	Label        string
	Placeholder  *string
	Required     bool
	Options      []string
	Validation   JSONB
	DisplayOrder int
}

const sqlCreateFormConfig = `
INSERT INTO campaign_form_configs (campaign_id, captcha_enabled, double_opt_in, custom_css)
VALUES ($1, $2, $3, $4)
RETURNING id, campaign_id, captcha_enabled, double_opt_in, custom_css, created_at, updated_at
`

// CreateCampaignFormConfig creates a new form configuration
func (s *Store) CreateCampaignFormConfig(ctx context.Context, params CreateCampaignFormConfigParams) (CampaignFormConfig, error) {
	var config CampaignFormConfig
	err := s.db.GetContext(ctx, &config, sqlCreateFormConfig,
		params.CampaignID,
		params.CaptchaEnabled,
		params.DoubleOptIn,
		params.CustomCSS)
	if err != nil {
		s.logger.Error(ctx, "failed to create campaign form config", err)
		return CampaignFormConfig{}, fmt.Errorf("failed to create campaign form config: %w", err)
	}

	// Create form fields if provided
	if len(params.Fields) > 0 {
		fields, err := s.CreateCampaignFormFields(ctx, config.ID, params.Fields)
		if err != nil {
			s.logger.Error(ctx, "failed to create form fields", err)
			return CampaignFormConfig{}, err
		}
		config.Fields = fields
	}

	return config, nil
}

const sqlCreateFormField = `
INSERT INTO campaign_form_fields (form_config_id, name, type, label, placeholder, required, options, validation, display_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, form_config_id, name, type, label, placeholder, required, options, validation, display_order, created_at, updated_at
`

// CreateCampaignFormFields creates multiple form fields
func (s *Store) CreateCampaignFormFields(ctx context.Context, formConfigID uuid.UUID, fields []CreateFormFieldParams) ([]CampaignFormField, error) {
	result := make([]CampaignFormField, 0, len(fields))

	for _, fieldParam := range fields {
		var field CampaignFormField
		err := s.db.GetContext(ctx, &field, sqlCreateFormField,
			formConfigID,
			fieldParam.Name,
			fieldParam.Type,
			fieldParam.Label,
			fieldParam.Placeholder,
			fieldParam.Required,
			StringArray(fieldParam.Options),
			fieldParam.Validation,
			fieldParam.DisplayOrder)
		if err != nil {
			s.logger.Error(ctx, "failed to create form field", err)
			return nil, fmt.Errorf("failed to create form field: %w", err)
		}
		result = append(result, field)
	}

	return result, nil
}

const sqlGetFormConfigByCampaignID = `
SELECT id, campaign_id, captcha_enabled, double_opt_in, custom_css, created_at, updated_at
FROM campaign_form_configs
WHERE campaign_id = $1
`

// GetFormConfigByCampaignID retrieves form config for a campaign
func (s *Store) GetFormConfigByCampaignID(ctx context.Context, campaignID uuid.UUID) (CampaignFormConfig, error) {
	var config CampaignFormConfig
	err := s.db.GetContext(ctx, &config, sqlGetFormConfigByCampaignID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get form config", err)
		return CampaignFormConfig{}, fmt.Errorf("failed to get form config: %w", err)
	}

	// Load form fields
	fields, err := s.GetFormFieldsByConfigID(ctx, config.ID)
	if err != nil {
		s.logger.Error(ctx, "failed to get form fields", err)
		return CampaignFormConfig{}, err
	}
	config.Fields = fields

	return config, nil
}

const sqlGetFormFieldsByConfigID = `
SELECT id, form_config_id, name, type, label, placeholder, required, options, validation, display_order, created_at, updated_at
FROM campaign_form_fields
WHERE form_config_id = $1
ORDER BY display_order ASC
`

// GetFormFieldsByConfigID retrieves all form fields for a config
func (s *Store) GetFormFieldsByConfigID(ctx context.Context, formConfigID uuid.UUID) ([]CampaignFormField, error) {
	var fields []CampaignFormField
	err := s.db.SelectContext(ctx, &fields, sqlGetFormFieldsByConfigID, formConfigID)
	if err != nil {
		s.logger.Error(ctx, "failed to get form fields", err)
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}
	return fields, nil
}

// UpdateCampaignFormConfigParams represents parameters for updating a form config
type UpdateCampaignFormConfigParams struct {
	CaptchaEnabled *bool
	DoubleOptIn    *bool
	CustomCSS      *string
}

const sqlUpdateFormConfig = `
UPDATE campaign_form_configs
SET captcha_enabled = COALESCE($2, captcha_enabled),
    double_opt_in = COALESCE($3, double_opt_in),
    custom_css = COALESCE($4, custom_css),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, captcha_enabled, double_opt_in, custom_css, created_at, updated_at
`

// UpdateCampaignFormConfig updates a form configuration
func (s *Store) UpdateCampaignFormConfig(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignFormConfigParams) (CampaignFormConfig, error) {
	var config CampaignFormConfig
	err := s.db.GetContext(ctx, &config, sqlUpdateFormConfig,
		campaignID,
		params.CaptchaEnabled,
		params.DoubleOptIn,
		params.CustomCSS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignFormConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update form config", err)
		return CampaignFormConfig{}, fmt.Errorf("failed to update form config: %w", err)
	}
	return config, nil
}

// ============================================================================
// Referral Config Operations
// ============================================================================

// CreateCampaignReferralConfigParams represents parameters for creating a referral config
type CreateCampaignReferralConfigParams struct {
	CampaignID          uuid.UUID
	Enabled             bool
	PointsPerReferral   int
	VerifiedOnly        bool
	SharingChannels     []string
	CustomShareMessages JSONB
}

const sqlCreateReferralConfig = `
INSERT INTO campaign_referral_configs (campaign_id, enabled, points_per_referral, verified_only, sharing_channels, custom_share_messages)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, campaign_id, enabled, points_per_referral, verified_only, sharing_channels, custom_share_messages, created_at, updated_at
`

// CreateCampaignReferralConfig creates a new referral configuration
func (s *Store) CreateCampaignReferralConfig(ctx context.Context, params CreateCampaignReferralConfigParams) (CampaignReferralConfig, error) {
	var config CampaignReferralConfig
	err := s.db.GetContext(ctx, &config, sqlCreateReferralConfig,
		params.CampaignID,
		params.Enabled,
		params.PointsPerReferral,
		params.VerifiedOnly,
		StringArray(params.SharingChannels),
		params.CustomShareMessages)
	if err != nil {
		s.logger.Error(ctx, "failed to create referral config", err)
		return CampaignReferralConfig{}, fmt.Errorf("failed to create referral config: %w", err)
	}
	return config, nil
}

const sqlGetReferralConfigByCampaignID = `
SELECT id, campaign_id, enabled, points_per_referral, verified_only, sharing_channels, custom_share_messages, created_at, updated_at
FROM campaign_referral_configs
WHERE campaign_id = $1
`

// GetReferralConfigByCampaignID retrieves referral config for a campaign
func (s *Store) GetReferralConfigByCampaignID(ctx context.Context, campaignID uuid.UUID) (CampaignReferralConfig, error) {
	var config CampaignReferralConfig
	err := s.db.GetContext(ctx, &config, sqlGetReferralConfigByCampaignID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignReferralConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get referral config", err)
		return CampaignReferralConfig{}, fmt.Errorf("failed to get referral config: %w", err)
	}
	return config, nil
}

// UpdateCampaignReferralConfigParams represents parameters for updating a referral config
type UpdateCampaignReferralConfigParams struct {
	Enabled             *bool
	PointsPerReferral   *int
	VerifiedOnly        *bool
	SharingChannels     []string
	CustomShareMessages JSONB
}

const sqlUpdateReferralConfig = `
UPDATE campaign_referral_configs
SET enabled = COALESCE($2, enabled),
    points_per_referral = COALESCE($3, points_per_referral),
    verified_only = COALESCE($4, verified_only),
    sharing_channels = COALESCE($5, sharing_channels),
    custom_share_messages = COALESCE($6, custom_share_messages),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, enabled, points_per_referral, verified_only, sharing_channels, custom_share_messages, created_at, updated_at
`

// UpdateCampaignReferralConfig updates a referral configuration
func (s *Store) UpdateCampaignReferralConfig(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignReferralConfigParams) (CampaignReferralConfig, error) {
	var config CampaignReferralConfig

	var sharingChannels interface{} = nil
	if len(params.SharingChannels) > 0 {
		sharingChannels = StringArray(params.SharingChannels)
	}

	err := s.db.GetContext(ctx, &config, sqlUpdateReferralConfig,
		campaignID,
		params.Enabled,
		params.PointsPerReferral,
		params.VerifiedOnly,
		sharingChannels,
		params.CustomShareMessages)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignReferralConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update referral config", err)
		return CampaignReferralConfig{}, fmt.Errorf("failed to update referral config: %w", err)
	}
	return config, nil
}

// ============================================================================
// Email Config Operations
// ============================================================================

// CreateCampaignEmailConfigParams represents parameters for creating an email config
type CreateCampaignEmailConfigParams struct {
	CampaignID           uuid.UUID
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired bool
}

const sqlCreateEmailConfig = `
INSERT INTO campaign_email_configs (campaign_id, from_name, from_email, reply_to, verification_required)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, from_name, from_email, reply_to, verification_required, created_at, updated_at
`

// CreateCampaignEmailConfig creates a new email configuration
func (s *Store) CreateCampaignEmailConfig(ctx context.Context, params CreateCampaignEmailConfigParams) (CampaignEmailConfig, error) {
	var config CampaignEmailConfig
	err := s.db.GetContext(ctx, &config, sqlCreateEmailConfig,
		params.CampaignID,
		params.FromName,
		params.FromEmail,
		params.ReplyTo,
		params.VerificationRequired)
	if err != nil {
		s.logger.Error(ctx, "failed to create email config", err)
		return CampaignEmailConfig{}, fmt.Errorf("failed to create email config: %w", err)
	}
	return config, nil
}

const sqlGetEmailConfigByCampaignID = `
SELECT id, campaign_id, from_name, from_email, reply_to, verification_required, created_at, updated_at
FROM campaign_email_configs
WHERE campaign_id = $1
`

// GetEmailConfigByCampaignID retrieves email config for a campaign
func (s *Store) GetEmailConfigByCampaignID(ctx context.Context, campaignID uuid.UUID) (CampaignEmailConfig, error) {
	var config CampaignEmailConfig
	err := s.db.GetContext(ctx, &config, sqlGetEmailConfigByCampaignID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get email config", err)
		return CampaignEmailConfig{}, fmt.Errorf("failed to get email config: %w", err)
	}
	return config, nil
}

// UpdateCampaignEmailConfigParams represents parameters for updating an email config
type UpdateCampaignEmailConfigParams struct {
	FromName             *string
	FromEmail            *string
	ReplyTo              *string
	VerificationRequired *bool
}

const sqlUpdateEmailConfig = `
UPDATE campaign_email_configs
SET from_name = COALESCE($2, from_name),
    from_email = COALESCE($3, from_email),
    reply_to = COALESCE($4, reply_to),
    verification_required = COALESCE($5, verification_required),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, from_name, from_email, reply_to, verification_required, created_at, updated_at
`

// UpdateCampaignEmailConfig updates an email configuration
func (s *Store) UpdateCampaignEmailConfig(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignEmailConfigParams) (CampaignEmailConfig, error) {
	var config CampaignEmailConfig
	err := s.db.GetContext(ctx, &config, sqlUpdateEmailConfig,
		campaignID,
		params.FromName,
		params.FromEmail,
		params.ReplyTo,
		params.VerificationRequired)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignEmailConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update email config", err)
		return CampaignEmailConfig{}, fmt.Errorf("failed to update email config: %w", err)
	}
	return config, nil
}

// ============================================================================
// Branding Config Operations
// ============================================================================

// CreateCampaignBrandingConfigParams represents parameters for creating a branding config
type CreateCampaignBrandingConfigParams struct {
	CampaignID   uuid.UUID
	LogoURL      *string
	PrimaryColor string
	FontFamily   string
	CustomDomain *string
}

const sqlCreateBrandingConfig = `
INSERT INTO campaign_branding_configs (campaign_id, logo_url, primary_color, font_family, custom_domain)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
`

// CreateCampaignBrandingConfig creates a new branding configuration
func (s *Store) CreateCampaignBrandingConfig(ctx context.Context, params CreateCampaignBrandingConfigParams) (CampaignBrandingConfig, error) {
	var config CampaignBrandingConfig
	err := s.db.GetContext(ctx, &config, sqlCreateBrandingConfig,
		params.CampaignID,
		params.LogoURL,
		params.PrimaryColor,
		params.FontFamily,
		params.CustomDomain)
	if err != nil {
		s.logger.Error(ctx, "failed to create branding config", err)
		return CampaignBrandingConfig{}, fmt.Errorf("failed to create branding config: %w", err)
	}
	return config, nil
}

const sqlGetBrandingConfigByCampaignID = `
SELECT id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
FROM campaign_branding_configs
WHERE campaign_id = $1
`

// GetBrandingConfigByCampaignID retrieves branding config for a campaign
func (s *Store) GetBrandingConfigByCampaignID(ctx context.Context, campaignID uuid.UUID) (CampaignBrandingConfig, error) {
	var config CampaignBrandingConfig
	err := s.db.GetContext(ctx, &config, sqlGetBrandingConfigByCampaignID, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignBrandingConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get branding config", err)
		return CampaignBrandingConfig{}, fmt.Errorf("failed to get branding config: %w", err)
	}
	return config, nil
}

// UpdateCampaignBrandingConfigParams represents parameters for updating a branding config
type UpdateCampaignBrandingConfigParams struct {
	LogoURL      *string
	PrimaryColor *string
	FontFamily   *string
	CustomDomain *string
}

const sqlUpdateBrandingConfig = `
UPDATE campaign_branding_configs
SET logo_url = COALESCE($2, logo_url),
    primary_color = COALESCE($3, primary_color),
    font_family = COALESCE($4, font_family),
    custom_domain = COALESCE($5, custom_domain),
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_id = $1
RETURNING id, campaign_id, logo_url, primary_color, font_family, custom_domain, created_at, updated_at
`

// UpdateCampaignBrandingConfig updates a branding configuration
func (s *Store) UpdateCampaignBrandingConfig(ctx context.Context, campaignID uuid.UUID, params UpdateCampaignBrandingConfigParams) (CampaignBrandingConfig, error) {
	var config CampaignBrandingConfig
	err := s.db.GetContext(ctx, &config, sqlUpdateBrandingConfig,
		campaignID,
		params.LogoURL,
		params.PrimaryColor,
		params.FontFamily,
		params.CustomDomain)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignBrandingConfig{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to update branding config", err)
		return CampaignBrandingConfig{}, fmt.Errorf("failed to update branding config: %w", err)
	}
	return config, nil
}

// ============================================================================
// Helper: Load All Configs for a Campaign
// ============================================================================

// LoadCampaignConfigs loads all config types for a campaign and attaches them to the Campaign struct
func (s *Store) LoadCampaignConfigs(ctx context.Context, campaign *Campaign) error {
	// Load form config
	formConfig, err := s.GetFormConfigByCampaignID(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if err == nil {
		campaign.FormConfig = &formConfig
	}

	// Load referral config
	referralConfig, err := s.GetReferralConfigByCampaignID(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if err == nil {
		campaign.ReferralConfig = &referralConfig
	}

	// Load email config
	emailConfig, err := s.GetEmailConfigByCampaignID(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if err == nil {
		campaign.EmailConfig = &emailConfig
	}

	// Load branding config
	brandingConfig, err := s.GetBrandingConfigByCampaignID(ctx, campaign.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if err == nil {
		campaign.BrandingConfig = &brandingConfig
	}

	return nil
}
