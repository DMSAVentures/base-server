package processor

import (
	"context"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/stripe/stripe-go/v79/tax/calculation"
)

func (p *BillingProcessor) CreateStripePaymentIntent(ctx context.Context, items []PaymentIntentItem) (string, error) {
	// Create a Tax Calculation for the items being sold
	taxCalculation, err := p.calculateTax(ctx, items, stripe.CurrencyCAD, stripe.Address{
		Line1:      "147 Scout St",
		City:       "Ottawa",
		PostalCode: "K2C 4E3",
		Country:    "CA",
		State:      "Ontario",
	})
	if err != nil {
		p.logger.Error(ctx, "failed to calculate tax", err)
		return "", err
	}

	totalAmount := p.calculateOrderAmount(taxCalculation)

	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(totalAmount),
		Currency: stripe.String(string(stripe.CurrencyCAD)),
		// In the latest version of the API, specifying the `automatic_payment_methods` parameter is optional because
		// Stripe enables its functionality by default.
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	params.AddMetadata("tax_calculation", taxCalculation.ID)

	pi, err := paymentintent.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create payment intent", err)
		return "", err
	}
	return pi.ClientSecret, nil
}

func (p *BillingProcessor) calculateTax(ctx context.Context, items []PaymentIntentItem, currency stripe.Currency,
	customerAddress stripe.Address) (*stripe.TaxCalculation, error) {
	var lineItems []*stripe.TaxCalculationLineItemParams
	for _, item := range items {
		lineItems = append(lineItems, p.buildLineItem(item))
	}

	taxCalculationParams := &stripe.TaxCalculationParams{
		Currency: stripe.String(string(currency)),
		CustomerDetails: &stripe.TaxCalculationCustomerDetailsParams{
			Address: &stripe.AddressParams{
				Line1:      stripe.String(customerAddress.Line1),
				City:       stripe.String(customerAddress.City),
				State:      stripe.String(customerAddress.State),
				PostalCode: stripe.String(customerAddress.PostalCode),
				Country:    stripe.String(customerAddress.Country),
			},
			AddressSource: stripe.String("shipping"),
		},
		LineItems: lineItems,
	}

	taxCalculation, err := calculation.New(taxCalculationParams)
	if err != nil {
		p.logger.Error(ctx, "failed to create tax calculation", err)
		return nil, err
	}

	return taxCalculation, nil
}

func (p *BillingProcessor) buildLineItem(
	i PaymentIntentItem) *stripe.TaxCalculationLineItemParams {
	return &stripe.TaxCalculationLineItemParams{
		Amount:    stripe.Int64(i.Amount), // Amount in cents
		Reference: stripe.String(i.Id),    // Unique reference for the PaymentIntentItem in the scope of the calculation
	}
}

// Securely calculate the order amount, including tax
func (p *BillingProcessor) calculateOrderAmount(taxCalculation *stripe.TaxCalculation) int64 {
	// Calculate the order total with any exclusive taxes on the server to prevent
	// people from directly manipulating the amount on the client
	return taxCalculation.AmountTotal
}
