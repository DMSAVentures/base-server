package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PaymentMethod represents a payment method.
type PaymentMethod struct {
	ID           uuid.UUID `db:"id"`
	UserID       uuid.UUID `db:"user_id"`
	StripeID     string    `db:"stripe_id"`
	CardBrand    string    `db:"card_brand"`
	CardLast4    string    `db:"card_last4"`
	CardExpMonth int       `db:"card_exp_month"`
	CardExpYear  int       `db:"card_exp_year"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

const sqlCreatePaymentMethod = `
	INSERT INTO payment_method (user_id, stripe_id, card_brand, card_last4, card_exp_month, card_exp_year)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, user_id, stripe_id, card_brand, card_last4, card_exp_month, card_exp_year, created_at, updated_at;
`

// CreatePaymentMethodParams represents the parameters used to create a payment method.
type CreatePaymentMethodParams struct {
	UserID       uuid.UUID
	StripeID     string
	CardBrand    string
	CardLast4    string
	CardExpMonth int
	CardExpYear  int
}

// CreatePaymentMethod creates a new payment method.
func (s *Store) CreatePaymentMethod(ctx context.Context, params CreatePaymentMethodParams) (*PaymentMethod, error) {
	var paymentMethod PaymentMethod
	err := s.db.GetContext(ctx, &paymentMethod, sqlCreatePaymentMethod, params.UserID, params.StripeID, params.CardBrand, params.CardLast4, params.CardExpMonth, params.CardExpYear)
	if err != nil {
		s.logger.Error(ctx, "failed to create payment method", err)
		return nil, fmt.Errorf("failed to create payment method: %w", err)
	}
	return &paymentMethod, nil
}

const sqlGetPaymentMethodByUserID = `
	SELECT id, user_id, stripe_id, card_brand, card_last4, card_exp_month, card_exp_year, created_at, updated_at
	FROM payment_method
	WHERE user_id = $1;
`

// GetPaymentMethodByUserID returns a payment method by User ID.
func (s *Store) GetPaymentMethodByUserID(ctx context.Context, userID uuid.UUID) (*PaymentMethod, error) {
	var paymentMethod PaymentMethod
	err := s.db.GetContext(ctx, &paymentMethod, sqlGetPaymentMethodByUserID, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to get payment method by user id", err)
		return nil, fmt.Errorf("failed to get payment method by user id: %w", err)
	}
	return &paymentMethod, nil
}

const sqlUpdatePaymentMethodByUserID = `
	UPDATE payment_method
	SET stripe_id = $1, card_brand = $2, card_last4 = $3, card_exp_month = $4, card_exp_year = $5, updated_at = NOW()
	WHERE user_id = $6;
`

// UpdatePaymentMethodByStripeID updates a payment method by Stripe ID.
func (s *Store) UpdatePaymentMethodByUserID(ctx context.Context, userID uuid.UUID, stripeID string, cardBrand,
	cardLast4 string,
	cardExpMonth, cardExpYear int64) error {
	res, err := s.db.ExecContext(ctx, sqlUpdatePaymentMethodByUserID, stripeID, cardBrand, cardLast4, cardExpMonth,
		cardExpYear,
		userID)
	if err != nil {
		s.logger.Error(ctx, "failed to update payment method by user id", err)
		return fmt.Errorf("failed to update payment method by user id: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		s.logger.Error(ctx, "payment method not found", nil)
		return fmt.Errorf("payment method not found")
	}
	return nil
}
