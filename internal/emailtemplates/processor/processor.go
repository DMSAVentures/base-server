package processor

//go:generate go run go.uber.org/mock/mockgen@latest -source=processor.go -destination=mocks_test.go -package=processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

// EmailTemplateStore defines the database operations required by EmailTemplateProcessor
type EmailTemplateStore interface {
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (store.Campaign, error)
	CreateEmailTemplate(ctx context.Context, params store.CreateEmailTemplateParams) (store.EmailTemplate, error)
	GetEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (store.EmailTemplate, error)
	GetEmailTemplatesByCampaign(ctx context.Context, campaignID uuid.UUID) ([]store.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, templateID uuid.UUID, params store.UpdateEmailTemplateParams) (store.EmailTemplate, error)
	DeleteEmailTemplate(ctx context.Context, templateID uuid.UUID) error
}

// EmailService defines the email operations required by EmailTemplateProcessor
type EmailService interface {
	SendEmail(ctx context.Context, to, subject, htmlContent string) error
}

var (
	ErrTemplateNotFound        = errors.New("email template not found")
	ErrCampaignNotFound        = errors.New("campaign not found")
	ErrUnauthorized            = errors.New("unauthorized access to template")
	ErrInvalidTemplateType     = errors.New("invalid template type")
	ErrInvalidTemplateContent  = errors.New("invalid template content")
	ErrTestEmailFailed         = errors.New("failed to send test email")
)

type EmailTemplateProcessor struct {
	store        EmailTemplateStore
	emailService EmailService
	logger       *observability.Logger
}

func New(store EmailTemplateStore, emailService EmailService, logger *observability.Logger) EmailTemplateProcessor {
	return EmailTemplateProcessor{
		store:        store,
		emailService: emailService,
		logger:       logger,
	}
}

// CreateEmailTemplateRequest represents a request to create an email template
type CreateEmailTemplateRequest struct {
	Name              string
	Type              string
	Subject           string
	HTMLBody          string
	BlocksJSON        interface{}
	Enabled           *bool
	SendAutomatically *bool
}

// CreateEmailTemplate creates a new email template for a campaign
func (p *EmailTemplateProcessor) CreateEmailTemplate(ctx context.Context, accountID, campaignID uuid.UUID, req CreateEmailTemplateRequest) (store.EmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_type", Value: req.Type},
	)

	// Validate template type
	if !isValidTemplateType(req.Type) {
		return store.EmailTemplate{}, ErrInvalidTemplateType
	}

	// Validate template content (check if it's valid Go template syntax)
	if err := validateTemplateContent(req.HTMLBody); err != nil {
		p.logger.Error(ctx, "invalid HTML template content", err)
		return store.EmailTemplate{}, ErrInvalidTemplateContent
	}

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailTemplate{}, ErrUnauthorized
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

	params := store.CreateEmailTemplateParams{
		CampaignID:        campaignID,
		Name:              req.Name,
		Type:              req.Type,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        convertToJSONB(req.BlocksJSON),
		Enabled:           enabled,
		SendAutomatically: sendAutomatically,
	}

	emailTemplate, err := p.store.CreateEmailTemplate(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create email template", err)
		return store.EmailTemplate{}, err
	}

	p.logger.Info(ctx, "email template created successfully")
	return emailTemplate, nil
}

// GetEmailTemplate retrieves an email template by ID
func (p *EmailTemplateProcessor) GetEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID) (store.EmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailTemplate{}, ErrUnauthorized
	}

	emailTemplate, err := p.store.GetEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get email template", err)
		return store.EmailTemplate{}, err
	}

	// Verify template belongs to the campaign
	if emailTemplate.CampaignID != campaignID {
		return store.EmailTemplate{}, ErrUnauthorized
	}

	return emailTemplate, nil
}

// ListEmailTemplates retrieves all email templates for a campaign
func (p *EmailTemplateProcessor) ListEmailTemplates(ctx context.Context, accountID, campaignID uuid.UUID, templateType *string) ([]store.EmailTemplate, error) {
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

	templates, err := p.store.GetEmailTemplatesByCampaign(ctx, campaignID)
	if err != nil {
		p.logger.Error(ctx, "failed to list email templates", err)
		return nil, err
	}

	// Ensure templates is never null - return empty array instead
	if templates == nil {
		templates = []store.EmailTemplate{}
	}

	// Filter by type if specified
	if templateType != nil {
		filtered := []store.EmailTemplate{}
		for _, t := range templates {
			if t.Type == *templateType {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}

	return templates, nil
}

// UpdateEmailTemplateRequest represents a request to update an email template
type UpdateEmailTemplateRequest struct {
	Name              *string
	Subject           *string
	HTMLBody          *string
	BlocksJSON        interface{}
	Enabled           *bool
	SendAutomatically *bool
}

// UpdateEmailTemplate updates an email template
func (p *EmailTemplateProcessor) UpdateEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID, req UpdateEmailTemplateRequest) (store.EmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify campaign exists and belongs to account
	campaign, err := p.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrCampaignNotFound
		}
		p.logger.Error(ctx, "failed to get campaign", err)
		return store.EmailTemplate{}, err
	}

	if campaign.AccountID != accountID {
		return store.EmailTemplate{}, ErrUnauthorized
	}

	// Verify template exists and belongs to campaign
	existingTemplate, err := p.store.GetEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get email template", err)
		return store.EmailTemplate{}, err
	}

	if existingTemplate.CampaignID != campaignID {
		return store.EmailTemplate{}, ErrUnauthorized
	}

	// Validate template content if provided
	if req.HTMLBody != nil {
		if err := validateTemplateContent(*req.HTMLBody); err != nil {
			p.logger.Error(ctx, "invalid HTML template content", err)
			return store.EmailTemplate{}, ErrInvalidTemplateContent
		}
	}

	params := store.UpdateEmailTemplateParams{
		Name:              req.Name,
		Subject:           req.Subject,
		HTMLBody:          req.HTMLBody,
		BlocksJSON:        convertToJSONB(req.BlocksJSON),
		Enabled:           req.Enabled,
		SendAutomatically: req.SendAutomatically,
	}

	emailTemplate, err := p.store.UpdateEmailTemplate(ctx, templateID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.EmailTemplate{}, ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to update email template", err)
		return store.EmailTemplate{}, err
	}

	p.logger.Info(ctx, "email template updated successfully")
	return emailTemplate, nil
}

// DeleteEmailTemplate soft deletes an email template
func (p *EmailTemplateProcessor) DeleteEmailTemplate(ctx context.Context, accountID, campaignID, templateID uuid.UUID) error {
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
	existingTemplate, err := p.store.GetEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get email template", err)
		return err
	}

	if existingTemplate.CampaignID != campaignID {
		return ErrUnauthorized
	}

	err = p.store.DeleteEmailTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrTemplateNotFound
		}
		p.logger.Error(ctx, "failed to delete email template", err)
		return err
	}

	p.logger.Info(ctx, "email template deleted successfully")
	return nil
}

// SendTestEmailRequest represents a request to send a test email
type SendTestEmailRequest struct {
	RecipientEmail string
	TestData       map[string]interface{}
}

// SendTestEmail sends a test email using a template
func (p *EmailTemplateProcessor) SendTestEmail(ctx context.Context, accountID, campaignID, templateID uuid.UUID, req SendTestEmailRequest) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "campaign_id", Value: campaignID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
		observability.Field{Key: "recipient", Value: req.RecipientEmail},
	)

	// Get the template
	emailTemplate, err := p.GetEmailTemplate(ctx, accountID, campaignID, templateID)
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
