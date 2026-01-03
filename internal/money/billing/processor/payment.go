package processor

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/billingportal/session"
	"github.com/stripe/stripe-go/v79/setupintent"
)

var (
	ErrFailedToGetPaymentMethod    = errors.New("failed to get payment method")
	ErrFailedToUpdatePaymentMethod = errors.New("failed to update payment method")
)

type PaymentMethod struct {
	ID           uuid.UUID `json:"id"`
	CardBrand    string    `json:"card_brand"`
	CardLast4    string    `json:"card_last4"`
	CardExpMonth int       `json:"card_exp_month"`
	CardExpYear  int       `json:"card_exp_year"`
}

func (p *BillingProcessor) SetupPaymentMethodUpdateIntent(ctx context.Context, userID uuid.UUID) (string, error) {

	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get user", err)
		return "", ErrFailedToUpdatePaymentMethod
	}

	params := &stripe.SetupIntentParams{
		Customer: stripe.String(stripeCustomerID),
	}
	intent, err := setupintent.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create setup intent", err)
		return "", ErrFailedToUpdatePaymentMethod
	}

	return intent.ClientSecret, nil
}

func (p *BillingProcessor) GetPaymentMethodForUser(ctx context.Context, userID uuid.UUID) (PaymentMethod,
	error) {
	paymentMethod, err := p.store.GetPaymentMethodByUserID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get payment method", err)
		return PaymentMethod{}, ErrFailedToGetPaymentMethod
	}
	return PaymentMethod{
		ID:           paymentMethod.ID,
		CardBrand:    paymentMethod.CardBrand,
		CardLast4:    paymentMethod.CardLast4,
		CardExpMonth: paymentMethod.CardExpMonth,
		CardExpYear:  paymentMethod.CardExpYear,
	}, nil
}

func (p *BillingProcessor) CreateCustomerPortal(ctx context.Context, userID uuid.UUID) (string, error) {
	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer ID by user ID", err)
		return "", err
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:      stripe.String(stripeCustomerID),
		ReturnURL:     stripe.String(fmt.Sprintf("%s/account", p.webhostURL)),
		Configuration: stripe.String("bpc_1Sld611OfTxF9BUaCFtj705M"), // Portal config with subscription updates enabled
	}

	result, err := session.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create billing portal session", err)
		return "", err
	}

	return result.URL, nil
}
