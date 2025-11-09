package email

import (
	"context"
)

// EmailSender defines the interface for sending emails
type EmailSender interface {
	// SendWelcomeEmail sends a welcome email to a new user
	SendWelcomeEmail(ctx context.Context, to, firstName string) error

	// SendPasswordResetEmail sends a password reset email with a reset link
	SendPasswordResetEmail(ctx context.Context, to, resetLink string) error

	// SendSubscriptionConfirmationEmail sends a confirmation email when a user subscribes to a plan
	SendSubscriptionConfirmationEmail(ctx context.Context, to, planName string) error

	// SendEmail sends a generic email with custom content
	SendEmail(ctx context.Context, to, subject, htmlContent string) error

	// RegisterTemplate adds a new template to the email service
	RegisterTemplate(name, templateContent string) error

	// SendEmailWithTemplate sends an email using a template and custom data
	SendEmailWithTemplate(ctx context.Context, to, subject, templateName string, data TemplateData) error

	// SendWaitlistVerificationEmail sends a verification email for waitlist signup
	SendWaitlistVerificationEmail(ctx context.Context, to, firstName, campaignName, verificationLink, referralLink string, position int) error

	// SendWaitlistWelcomeEmail sends a welcome email after joining the waitlist
	SendWaitlistWelcomeEmail(ctx context.Context, to, firstName, campaignName, referralLink string, position, referralCount int) error

	// SendWaitlistPositionUpdateEmail sends an email when a user's position improves
	SendWaitlistPositionUpdateEmail(ctx context.Context, to, firstName, campaignName, referralLink string, newPosition, referralCount int) error
}
