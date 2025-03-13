package mail

import (
	"base-server/internal/observability"
	"context"
	"fmt"

	"github.com/resendlabs/resend-go"
)

type ResendClient struct {
	client *resend.Client
	logger *observability.Logger
}

func NewResendClient(apiKey string, logger *observability.Logger) (*ResendClient, error) {
	client := resend.NewClient(apiKey)
	if client == nil {
		return nil, fmt.Errorf("failed to create Resend client")
	}

	return &ResendClient{
		client: client,
		logger: logger,
	}, nil
}

func (c *ResendClient) SendEmail(ctx context.Context, from, to, subject, htmlContent string) (string, error) {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "email_to", Value: to},
		observability.Field{Key: "email_subject", Value: subject},
	)

	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlContent,
	}

	res, err := c.client.Emails.Send(params)
	if err != nil {
		c.logger.Error(ctx, "failed to send email", err)
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	c.logger.Info(ctx, "email sent successfully")
	return res.Id, nil
}
