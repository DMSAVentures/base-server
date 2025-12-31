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

// BlastEmailTemplateStore defines the database operations required by BlastEmailTemplateProcessor
type BlastEmailTemplateStore interface {
	CreateBlastEmailTemplate(ctx context.Context, params store.CreateBlastEmailTemplateParams) (store.BlastEmailTemplate, error)
	GetBlastEmailTemplateByID(ctx context.Context, templateID uuid.UUID) (store.BlastEmailTemplate, error)
	GetBlastEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) ([]store.BlastEmailTemplate, error)
	UpdateBlastEmailTemplate(ctx context.Context, templateID uuid.UUID, params store.UpdateBlastEmailTemplateParams) (store.BlastEmailTemplate, error)
	DeleteBlastEmailTemplate(ctx context.Context, templateID uuid.UUID) error
	CountBlastEmailTemplatesByAccount(ctx context.Context, accountID uuid.UUID) (int, error)
}

// EmailService defines the email operations required by BlastEmailTemplateProcessor
type EmailService interface {
	SendEmail(ctx context.Context, to, subject, htmlContent string) error
}

// TierChecker defines the tier checking operations required by BlastEmailTemplateProcessor
type TierChecker interface {
	HasFeatureByAccountID(ctx context.Context, accountID uuid.UUID, featureName string) (bool, error)
}

var (
	ErrBlastEmailTemplateNotFound = errors.New("blast email template not found")
	ErrUnauthorized               = errors.New("unauthorized access to template")
	ErrInvalidTemplateContent     = errors.New("invalid template content")
	ErrTestEmailFailed            = errors.New("failed to send test email")
	ErrBlastEmailTemplatesNotAvailable = errors.New("blast email templates are not available in your plan")
)

type BlastEmailTemplateProcessor struct {
	store        BlastEmailTemplateStore
	emailService EmailService
	tierChecker  TierChecker
	logger       *observability.Logger
}

func New(store BlastEmailTemplateStore, emailService EmailService, tierChecker TierChecker, logger *observability.Logger) BlastEmailTemplateProcessor {
	return BlastEmailTemplateProcessor{
		store:        store,
		emailService: emailService,
		tierChecker:  tierChecker,
		logger:       logger,
	}
}

// CreateBlastEmailTemplateRequest represents a request to create a blast email template
type CreateBlastEmailTemplateRequest struct {
	Name       string
	Subject    string
	HTMLBody   string
	BlocksJSON interface{}
}

// CreateBlastEmailTemplate creates a new blast email template for an account
func (p *BlastEmailTemplateProcessor) CreateBlastEmailTemplate(ctx context.Context, accountID uuid.UUID, req CreateBlastEmailTemplateRequest) (store.BlastEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	// Check if account has email blasts feature (blast templates require email blasts feature)
	hasFeature, err := p.tierChecker.HasFeatureByAccountID(ctx, accountID, "email_blasts")
	if err != nil {
		p.logger.Error(ctx, "failed to check email blasts feature", err)
		return store.BlastEmailTemplate{}, err
	}
	if !hasFeature {
		return store.BlastEmailTemplate{}, ErrBlastEmailTemplatesNotAvailable
	}

	// Validate template content (check if it's valid Go template syntax)
	if err := validateTemplateContent(req.HTMLBody); err != nil {
		p.logger.Error(ctx, "invalid HTML template content", err)
		return store.BlastEmailTemplate{}, ErrInvalidTemplateContent
	}

	params := store.CreateBlastEmailTemplateParams{
		AccountID:  accountID,
		Name:       req.Name,
		Subject:    req.Subject,
		HTMLBody:   req.HTMLBody,
		BlocksJSON: convertToJSONB(req.BlocksJSON),
	}

	template, err := p.store.CreateBlastEmailTemplate(ctx, params)
	if err != nil {
		p.logger.Error(ctx, "failed to create blast email template", err)
		return store.BlastEmailTemplate{}, err
	}

	p.logger.Info(ctx, "blast email template created successfully")
	return template, nil
}

// GetBlastEmailTemplate retrieves a blast email template by ID
func (p *BlastEmailTemplateProcessor) GetBlastEmailTemplate(ctx context.Context, accountID, templateID uuid.UUID) (store.BlastEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	template, err := p.store.GetBlastEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.BlastEmailTemplate{}, ErrBlastEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get blast email template", err)
		return store.BlastEmailTemplate{}, err
	}

	// Verify template belongs to the account
	if template.AccountID != accountID {
		return store.BlastEmailTemplate{}, ErrUnauthorized
	}

	return template, nil
}

// ListBlastEmailTemplates retrieves all blast email templates for an account
func (p *BlastEmailTemplateProcessor) ListBlastEmailTemplates(ctx context.Context, accountID uuid.UUID) ([]store.BlastEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
	)

	templates, err := p.store.GetBlastEmailTemplatesByAccount(ctx, accountID)
	if err != nil {
		p.logger.Error(ctx, "failed to list blast email templates", err)
		return nil, err
	}

	// Ensure templates is never null - return empty array instead
	if templates == nil {
		templates = []store.BlastEmailTemplate{}
	}

	return templates, nil
}

// UpdateBlastEmailTemplateRequest represents a request to update a blast email template
type UpdateBlastEmailTemplateRequest struct {
	Name       *string
	Subject    *string
	HTMLBody   *string
	BlocksJSON interface{}
}

// UpdateBlastEmailTemplate updates a blast email template
func (p *BlastEmailTemplateProcessor) UpdateBlastEmailTemplate(ctx context.Context, accountID, templateID uuid.UUID, req UpdateBlastEmailTemplateRequest) (store.BlastEmailTemplate, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify template exists and belongs to account
	existingTemplate, err := p.store.GetBlastEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.BlastEmailTemplate{}, ErrBlastEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get blast email template", err)
		return store.BlastEmailTemplate{}, err
	}

	if existingTemplate.AccountID != accountID {
		return store.BlastEmailTemplate{}, ErrUnauthorized
	}

	// Validate template content if provided
	if req.HTMLBody != nil {
		if err := validateTemplateContent(*req.HTMLBody); err != nil {
			p.logger.Error(ctx, "invalid HTML template content", err)
			return store.BlastEmailTemplate{}, ErrInvalidTemplateContent
		}
	}

	params := store.UpdateBlastEmailTemplateParams{
		Name:       req.Name,
		Subject:    req.Subject,
		HTMLBody:   req.HTMLBody,
		BlocksJSON: convertToJSONB(req.BlocksJSON),
	}

	template, err := p.store.UpdateBlastEmailTemplate(ctx, templateID, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.BlastEmailTemplate{}, ErrBlastEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to update blast email template", err)
		return store.BlastEmailTemplate{}, err
	}

	p.logger.Info(ctx, "blast email template updated successfully")
	return template, nil
}

// DeleteBlastEmailTemplate soft deletes a blast email template
func (p *BlastEmailTemplateProcessor) DeleteBlastEmailTemplate(ctx context.Context, accountID, templateID uuid.UUID) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
	)

	// Verify template exists and belongs to account
	existingTemplate, err := p.store.GetBlastEmailTemplateByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBlastEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to get blast email template", err)
		return err
	}

	if existingTemplate.AccountID != accountID {
		return ErrUnauthorized
	}

	err = p.store.DeleteBlastEmailTemplate(ctx, templateID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBlastEmailTemplateNotFound
		}
		p.logger.Error(ctx, "failed to delete blast email template", err)
		return err
	}

	p.logger.Info(ctx, "blast email template deleted successfully")
	return nil
}

// SendTestEmailRequest represents a request to send a test email
type SendTestEmailRequest struct {
	RecipientEmail string
	TestData       map[string]interface{}
}

// SendTestEmail sends a test email using a blast email template
func (p *BlastEmailTemplateProcessor) SendTestEmail(ctx context.Context, accountID, templateID uuid.UUID, req SendTestEmailRequest) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "account_id", Value: accountID.String()},
		observability.Field{Key: "template_id", Value: templateID.String()},
		observability.Field{Key: "recipient", Value: req.RecipientEmail},
	)

	// Get the template
	emailTemplate, err := p.GetBlastEmailTemplate(ctx, accountID, templateID)
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
			"referral_link":  "https://example.com/ref/ABC123",
			"referral_count": 5,
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
