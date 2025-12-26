package email

import (
	"base-server/internal/clients/mail"
	"base-server/internal/observability"
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"
)

var (
	ErrInvalidEmailAddress = errors.New("invalid email address")
	ErrSendingEmail        = errors.New("error sending email")
	ErrEmptyTemplate       = errors.New("email template is empty")
)

// EmailService handles sending emails
type EmailService struct {
	mailClient    *mail.ResendClient
	logger        *observability.Logger
	defaultSender string
	templates     map[string]string
}

// TemplateData represents the data that can be used in templates
type TemplateData struct {
	FirstName        string
	Email            string
	ResetLink        string
	SubscriptionID   string
	PlanName         string
	VerificationLink string
	Position         int
	ReferralLink     string
	ReferralCount    int
	CampaignName     string
	// Add more fields as needed
}

// New creates a new EmailService
func New(mailClient *mail.ResendClient, defaultSender string, logger *observability.Logger) *EmailService {
	return &EmailService{
		mailClient:    mailClient,
		logger:        logger,
		defaultSender: defaultSender,
		templates: map[string]string{
			"welcome": `
			<html>
				<body>
					<h1>Welcome, {{.FirstName}}!</h1>
					<p>Thank you for joining our platform. We're excited to have you on board.</p>
					<p>If you have any questions, please don't hesitate to reach out to our support team.</p>
				</body>
			</html>
			`,
			"password_reset": `
			<html>
				<body>
					<h1>Password Reset</h1>
					<p>You requested a password reset for your account.</p>
					<p>To reset your password, please click the link below:</p>
					<p><a href="{{.ResetLink}}">Reset your password</a></p>
					<p>If you didn't request this, you can safely ignore this email.</p>
				</body>
			</html>
			`,
			"subscription_confirmation": `
			<html>
				<body>
					<h1>Subscription Confirmed</h1>
					<p>Thank you for subscribing to our {{.PlanName}} plan.</p>
					<p>Your subscription is now active and you have full access to all features included in this plan.</p>
					<p>If you have any questions about your subscription, please contact our support team.</p>
				</body>
			</html>
			`,
			"waitlist_verification": `
			<html>
				<body>
					<h1>Verify Your Email</h1>
					<p>Hi {{.FirstName}},</p>
					<p>Thank you for joining the {{.CampaignName}} waitlist! You're currently at position <strong>#{{.Position}}</strong>.</p>
					<p>Please verify your email address to secure your spot:</p>
					<p><a href="{{.VerificationLink}}" style="background-color: #2563EB; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Verify Email</a></p>
					<p>Your unique referral link:</p>
					<p><a href="{{.ReferralLink}}">{{.ReferralLink}}</a></p>
					<p>Share this link to move up the waitlist!</p>
					<p>If you didn't sign up for this waitlist, you can safely ignore this email.</p>
				</body>
			</html>
			`,
			"waitlist_welcome": `
			<html>
				<body>
					<h1>Welcome to the Waitlist!</h1>
					<p>Hi {{.FirstName}},</p>
					<p>You've successfully joined the {{.CampaignName}} waitlist!</p>
					<p>Your current position: <strong>#{{.Position}}</strong></p>
					<p>Want to move up? Share your unique referral link:</p>
					<p><a href="{{.ReferralLink}}" style="background-color: #2563EB; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">{{.ReferralLink}}</a></p>
					<p>For every person who joins through your link, you'll move up the waitlist!</p>
					<p>Current referrals: <strong>{{.ReferralCount}}</strong></p>
					<p>We'll notify you when it's your turn. Stay tuned!</p>
				</body>
			</html>
			`,
			"waitlist_position_update": `
			<html>
				<body>
					<h1>Your Position Has Changed!</h1>
					<p>Hi {{.FirstName}},</p>
					<p>Great news! Your position on the {{.CampaignName}} waitlist has improved!</p>
					<p>New position: <strong>#{{.Position}}</strong></p>
					<p>Total referrals: <strong>{{.ReferralCount}}</strong></p>
					<p>Keep sharing your referral link to move up even faster:</p>
					<p><a href="{{.ReferralLink}}">{{.ReferralLink}}</a></p>
					<p>Thanks for spreading the word!</p>
				</body>
			</html>
			`,
		},
	}
}

// renderTemplate renders a template with the provided data
func (s *EmailService) renderTemplate(templateName string, data TemplateData) (string, error) {
	tmplStr, ok := s.templates[templateName]
	if !ok {
		return "", fmt.Errorf("template %s not found", templateName)
	}

	tmpl, err := template.New(templateName).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// SendWelcomeEmail sends a welcome email to a new user
func (s *EmailService) SendWelcomeEmail(ctx context.Context, to, firstName string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "welcome"},
		observability.Field{Key: "recipient", Value: to},
	)

	subject := "Welcome to our platform"

	data := TemplateData{
		FirstName: firstName,
		Email:     to,
	}

	htmlContent, err := s.renderTemplate("welcome", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render welcome email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send welcome email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendPasswordResetEmail sends a password reset email with a reset link
func (s *EmailService) SendPasswordResetEmail(ctx context.Context, to, resetLink string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "password_reset"},
		observability.Field{Key: "recipient", Value: to},
	)

	subject := "Password Reset Request"

	data := TemplateData{
		Email:     to,
		ResetLink: resetLink,
	}

	htmlContent, err := s.renderTemplate("password_reset", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render password reset email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send password reset email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendSubscriptionConfirmationEmail sends a confirmation email when a user subscribes to a plan
func (s *EmailService) SendSubscriptionConfirmationEmail(ctx context.Context, to, planName string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "subscription_confirmation"},
		observability.Field{Key: "recipient", Value: to},
		observability.Field{Key: "plan_name", Value: planName},
	)

	subject := "Subscription Confirmation"

	data := TemplateData{
		Email:    to,
		PlanName: planName,
	}

	htmlContent, err := s.renderTemplate("subscription_confirmation", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render subscription confirmation email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send subscription confirmation email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendEmail sends a generic email with custom content
func (s *EmailService) SendEmail(ctx context.Context, to, subject, htmlContent string) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "custom"},
		observability.Field{Key: "recipient", Value: to},
	)

	if htmlContent == "" {
		s.logger.Error(ctx, "empty email content", ErrEmptyTemplate)
		return ErrEmptyTemplate
	}

	_, err := s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send custom email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// RegisterTemplate adds a new template to the email service
func (s *EmailService) RegisterTemplate(name, templateContent string) error {
	// Validate the template by attempting to parse it
	_, err := template.New(name).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	s.templates[name] = templateContent
	return nil
}

// SendEmailWithTemplate sends an email using a template and custom data
func (s *EmailService) SendEmailWithTemplate(ctx context.Context, to, subject, templateName string, data TemplateData) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: templateName},
		observability.Field{Key: "recipient", Value: to},
	)

	htmlContent, err := s.renderTemplate(templateName, data)
	if err != nil {
		s.logger.Error(ctx, fmt.Sprintf("failed to render %s template", templateName), err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, fmt.Sprintf("failed to send %s email", templateName), err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendWaitlistVerificationEmail sends a verification email for waitlist signup
func (s *EmailService) SendWaitlistVerificationEmail(ctx context.Context, to, firstName, campaignName, verificationLink, referralLink string, position int) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "waitlist_verification"},
		observability.Field{Key: "recipient", Value: to},
		observability.Field{Key: "campaign", Value: campaignName},
	)

	subject := fmt.Sprintf("Verify your email - You're #%d on the %s waitlist", position, campaignName)

	data := TemplateData{
		FirstName:        firstName,
		Email:            to,
		CampaignName:     campaignName,
		VerificationLink: verificationLink,
		ReferralLink:     referralLink,
		Position:         position,
	}

	htmlContent, err := s.renderTemplate("waitlist_verification", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render waitlist verification email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send waitlist verification email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendWaitlistWelcomeEmail sends a welcome email after joining the waitlist
func (s *EmailService) SendWaitlistWelcomeEmail(ctx context.Context, to, firstName, campaignName, referralLink string, position, referralCount int) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "waitlist_welcome"},
		observability.Field{Key: "recipient", Value: to},
		observability.Field{Key: "campaign", Value: campaignName},
	)

	subject := fmt.Sprintf("Welcome to the %s waitlist!", campaignName)

	data := TemplateData{
		FirstName:     firstName,
		Email:         to,
		CampaignName:  campaignName,
		ReferralLink:  referralLink,
		Position:      position,
		ReferralCount: referralCount,
	}

	htmlContent, err := s.renderTemplate("waitlist_welcome", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render waitlist welcome email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send waitlist welcome email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// SendWaitlistPositionUpdateEmail sends an email when a user's position improves
func (s *EmailService) SendWaitlistPositionUpdateEmail(ctx context.Context, to, firstName, campaignName, referralLink string, newPosition, referralCount int) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "waitlist_position_update"},
		observability.Field{Key: "recipient", Value: to},
		observability.Field{Key: "campaign", Value: campaignName},
		observability.Field{Key: "new_position", Value: newPosition},
	)

	subject := fmt.Sprintf("You moved up to position #%d on %s!", newPosition, campaignName)

	data := TemplateData{
		FirstName:     firstName,
		Email:         to,
		CampaignName:  campaignName,
		ReferralLink:  referralLink,
		Position:      newPosition,
		ReferralCount: referralCount,
	}

	htmlContent, err := s.renderTemplate("waitlist_position_update", data)
	if err != nil {
		s.logger.Error(ctx, "failed to render waitlist position update email template", err)
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send waitlist position update email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}

// RenderCustomTemplate renders a custom template string with the provided data
func (s *EmailService) RenderCustomTemplate(ctx context.Context, templateContent string, data TemplateData) (string, error) {
	if templateContent == "" {
		return "", ErrEmptyTemplate
	}

	tmpl, err := template.New("custom").Parse(templateContent)
	if err != nil {
		s.logger.Error(ctx, "failed to parse custom template", err)
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		s.logger.Error(ctx, "failed to execute custom template", err)
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// SendCustomTemplateEmail renders a custom template and sends it
func (s *EmailService) SendCustomTemplateEmail(ctx context.Context, to, subject, templateContent string, data TemplateData) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_type", Value: "custom_template"},
		observability.Field{Key: "recipient", Value: to},
	)

	htmlContent, err := s.RenderCustomTemplate(ctx, templateContent, data)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrEmptyTemplate, err.Error())
	}

	_, err = s.mailClient.SendEmail(ctx, s.defaultSender, to, subject, htmlContent)
	if err != nil {
		s.logger.Error(ctx, "failed to send custom template email", err)
		return fmt.Errorf("%w: %s", ErrSendingEmail, err.Error())
	}

	return nil
}
