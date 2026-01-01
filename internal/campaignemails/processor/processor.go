package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/tiers"
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

// CampaignEmailTemplateStore defines the database operations required by CampaignEmailTemplateProcessor
type CampaignEmailTemplateStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	CreateCampaignEmailTemplate(ctx context.Context, params store.CreateCampaignEmailTemplateParams) (store.CampaignEmailTemplate, error)
	GetCampaignEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (store.CampaignEmailTemplate, error)
	GetCampaignEmailTemplatesByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.CampaignEmailTemplate, error)
	GetCampaignEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) ([]store.CampaignEmailTemplate, error)
	UpdateCampaignEmailTemplate(ctx context.Context, templateID uuid.UUID, params store.UpdateCampaignEmailTemplateParams) (store.CampaignEmailTemplate, error)
	DeleteCampaignEmailTemplate(ctx context.Context, templateID uuid.UUID) error
}

// EmailService defines the email operations required by CampaignEmailTemplateProcessor
type EmailService interface {
	SendEmail(ctx context.Context, to, subject, htmlContent string) error
}

var (
	ErrCampaignEmailTemplateNotFound   = errors.New("campaign email template not found")
	ErrCampaignNotFound                = errors.New("campaign not found")
	ErrUnauthorized                    = errors.New("unauthorized access to template")
	ErrInvalidTemplateType             = errors.New("invalid template type")
	ErrInvalidTemplateContent          = errors.New("invalid template content")
	ErrTestEmailFailed                 = errors.New("failed to send test email")
	ErrVisualEmailBuilderNotAvailable  = errors.New("visual email builder is not available in your plan")
)

type CampaignEmailTemplateProcessor struct {
	store        CampaignEmailTemplateStore
	emailService EmailService
	tierService  *tiers.TierService
	logger       *observability.Logger
}

func New(store CampaignEmailTemplateStore, emailService EmailService, tierService *tiers.TierService, logger *observability.Logger) CampaignEmailTemplateProcessor {
	return CampaignEmailTemplateProcessor{
		store:        store,
		emailService: emailService,
		tierService:  tierService,
		logger:       logger,
	}
}

// CreateCampaignEmailTemplateRequest represents a request to create a campaign email template
type CreateCampaignEmailTemplateRequest struct {
	Name              string
	Type              string
	Subject           string
	HTMLBody          string
	BlocksJSON        interface{}
	Enabled           *bool
	SendAutomatically *bool
}

// CreateCampaignEmailTemplate creates a new email template for a campaign
func (p *CampaignEmailTemplateProcessor) CreateCampaignEmailTemplate(ctx context.Context, accountID, campaignID uuid.UUID, req CreateCampaignEmailTemplateRequest) (store.CampaignEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_type", Value: req.Type},
	)

	// Check if account has visual_email_builder feature
	hasFeature, err := p.tierService.HasFeatureByAccountID(ctx, accountID, "visual_email_builder")
	if err != nil {
		p.logger.Error(ctx, "failed to check visual_email_builder feature", err)
		return store.CampaignEmailTemplate{}, err
	}
	if !hasFeature {
		return store.CampaignEmailTemplate{}, ErrVisualEmailBuilderNotAvailable
	}

	// Validate template type
	if !isValidTemplateType(req.Type) {
		return store.CampaignEmailTemplate{}, ErrInvalidTemplateType
	}

	// Validate template content (check if it's valid Go template syntax)
	if err := validateTemplateContent(req.HTMLBody); err != nil {
		p.logger.Error(ctx, "invalid HTML template content", err)
		return store.CampaignEmailTemplate{}, ErrInvalidTemplateContent
	}

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.CampaignEmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.CampaignEmailTemplate{}, ErrUnauthorized
	}

	// Set defaults
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	sendAutomatically := true
	if req.SendAutomatically != nil {
		sendAutomatically = *req.SendAutomatically
	}

	params := store.CreateCampaignEmailTemplateParams{
		CampaignID:        campaignID,
		Name:              req.Name,
		Type:              req.Type,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        convertToJSONB(req.BlocksJSON),
		Enabled:           enabled,
		SendAutomatically: sendAutomatically,
	}

	emailTemplate, err := p.store.CreateCampaignEmailTemplate(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create campaign email template", err)
		return store.CampaignEmailTemplate{}, err
	}

	p.logger.Info(ctx, "campaign email template created successfully")
	return emailTemplate, nil
}

// GetCampaignEmailTemplate retrieves a campaign email template by ID
func (p *CampaignEmailTemplateProcessor) GetCampaignEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID) (store.CampaignEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.CampaignEmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.CampaignEmailTemplate{}, ErrUnauthorized
	}

	emailTemplate, err := p.store.GetCampaignEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get campaign email template", err)
		return store.CampaignEmailTemplate{}, err
	}

	// Verify template belongs to the campaign
	if emailTemplate.CampaignID != campaignID {
		return store.CampaignEmailTemplate{}, ErrUnauthorized
	}

	return emailTemplate, nil
}

// ListCampaignEmailTemplates retrieves all email templates for a campaign
func (p *CampaignEmailTemplateProcessor) ListCampaignEmailTemplates(ctx context.Context, accountID, campaignID uuid.UUID, templateType *string) ([]store.CampaignEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return nil, err
	}

	if campaign.AccountID != accountID {
		return nil, ErrUnauthorized
	}

	// Validate template type filter if provided
	if templateType != nil && !isValidTemplateType(*templateType) {
		return nil, ErrInvalidTemplateType
	}

	templates, err := p.store.GetCampaignEmailTemplatesByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to list campaign email templates", err)
		return nil, err
	}

	// Ensure templates is never null - return empty array instead
	if templates == nil {
		templates = []store.CampaignEmailTemplate{}
	}

	// Filter by type if specified
	if templateType != nil {
		filtered := []store.CampaignEmailTemplate{}
		for _, t := range templates {
			if t.Type == *templateType {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}

	return templates, nil
}

// ListAllCampaignEmailTemplates retrieves all email templates for an account across all campaigns
func (p *CampaignEmailTemplateProcessor) ListAllCampaignEmailTemplates(ctx context.Context, accountID uuid.UUID) ([]store.CampaignEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	templates, err := p.store.GetCampaignEmailTemplatesByAccount(ctx, accountID)
	if err != nil {
		p.logger.Error(ctx, "failed to list all campaign email templates", err)
		return nil, err
	}

	// Ensure templates is never null - return empty array instead
	if templates == nil {
		templates = []store.CampaignEmailTemplate{}
	}

	return templates, nil
}

// UpdateCampaignEmailTemplateRequest represents a request to update a campaign email template
type UpdateCampaignEmailTemplateRequest struct {
	Name              *string
	Subject           *string
	HTMLBody          *string
	BlocksJSON        interface{}
	Enabled           *bool
	SendAutomatically *bool
}

// UpdateCampaignEmailTemplate updates a campaign email template
func (p *CampaignEmailTemplateProcessor) UpdateCampaignEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID, req UpdateCampaignEmailTemplateRequest) (store.CampaignEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Check if account has visual_email_builder feature
	hasFeature, err := p.tierService.HasFeatureByAccountID(ctx, accountID, "visual_email_builder")
	if err != nil {
		p.logger.Error(ctx, "failed to check visual_email_builder feature", err)
		return store.CampaignEmailTemplate{}, err
	}
	if !hasFeature {
		return store.CampaignEmailTemplate{}, ErrVisualEmailBuilderNotAvailable
	}

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.CampaignEmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.CampaignEmailTemplate{}, ErrUnauthorized
	}

	// Verify template exists and belongs to campaign
	existingTemplate, err := p.store.GetCampaignEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get campaign email template", err)
		return store.CampaignEmailTemplate{}, err
	}

	if existingTemplate.CampaignID != campaignID {
		return store.CampaignEmailTemplate{}, ErrUnauthorized
	}

	// Validate template content if provided
	if req.HTMLBody != nil {
		if err := validateTemplateContent(*req.HTMLBody); err != nil {
			p.logger.Error(ctx, "invalid HTML template content", err)
			return store.CampaignEmailTemplate{}, ErrInvalidTemplateContent
		}
	}

	params := store.UpdateCampaignEmailTemplateParams{
		Name:              req.Name,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        convertToJSONB(req.BlocksJSON),
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	emailTemplate, err := p.store.UpdateCampaignEmailTemplate(ctx, templateID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.CampaignEmailTemplate{}, ErrCampaignEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to update campaign email template", err)
		return store.CampaignEmailTemplate{}, err
	}

	p.logger.Info(ctx, "campaign email template updated successfully")
	return emailTemplate, nil
}

// DeleteCampaignEmailTemplate soft deletes a campaign email template
func (p *CampaignEmailTemplateProcessor) DeleteCampaignEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return err
	}

	if campaign.AccountID != accountID {
		return ErrUnauthorized
	}

	// Verify template exists and belongs to campaign
	existingTemplate, err := p.store.GetCampaignEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get campaign email template", err)
		return err
	}

	if existingTemplate.CampaignID != campaignID {
		return ErrUnauthorized
	}

	err = p.store.DeleteCampaignEmailTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCampaignEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to delete campaign email template", err)
		return err
	}

	p.logger.Info(ctx, "campaign email template deleted successfully")
	return nil
}

// SendTestEmailRequest represents a request to send a test email
type SendTestEmailRequest struct {
	RecipientEmail string
	TestData       map[string]interface{}
}

// SendTestEmail sends a test email using a campaign email template
func (p *CampaignEmailTemplateProcessor) SendTestEmail(ctx context.Context, accountID, campaignID, templateID uuid.UUID, req SendTestEmailRequest) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
		observability.Field{Key: "recipient", Value: req.RecipientEmail},
	)

	// Get the template
	emailTemplate, err := p.GetCampaignEmailTemplate(ctx, accountID, campaignID, templateID)
	if err != nil {
		return err
	}

	// Render the HTML body with test data
	htmlContent, err := renderTemplateWithData(emailTemplate.HTMLBody, req.TestData)
	if err != nil {
		p.logger.Error(ctx, "failed to render HTML template", err)
		return ErrTestEmailFailed
	}

	// Send the test email
	err = p.emailService.SendEmail(ctx, req.RecipientEmail, emailTemplate.Subject, htmlContent)
	if err != nil {
		p.logger.Error(ctx, "failed to send test email", err)
		return ErrTestEmailFailed
	}

	p.logger.Info(ctx, "test email sent successfully")
	return nil
}

// Helper functions

// convertToJSONB converts an interface{} to *store.JSONB
func convertToJSONB(data interface{}) *store.JSONB {
	if data == nil {
		return nil
	}

	// If it's already a map[string]interface{}, convert directly
	if m, ok := data.(map[string]interface{}); ok {
		jsonb := store.JSONB(m)
		return &jsonb
	}

	return nil
}

func isValidTemplateType(templateType string) bool {
	validTypes := map[string]bool{
		"verification":    true,
		"welcome":         true,
		"position_update": true,
		"reward_earned":   true,
		"milestone":       true,
		"custom":          true,
	}
	return validTypes[templateType]
}

func validateTemplateContent(content string) error {
	_, err := template.New("validate").Parse(content)
	return err
}

func renderTemplateWithData(templateContent string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("email").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Set default test data if not provided
	if data == nil {
		data = map[string]interface{}{
			"first_name":     "John",
			"last_name":      "Doe",
			"email":          "test@example.com",
			"position":       1,
			"referral_link":  "https://example.com/ref/ABC123",
			"referral_count": 5,
			"campaign_name":  "Test Campaign",
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
