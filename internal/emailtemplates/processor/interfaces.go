package processor

import (
	"base-server/internal/store"
	"context"

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
