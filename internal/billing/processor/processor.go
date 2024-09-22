package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/stripe/stripe-go/v79/subscription"
	"github.com/stripe/stripe-go/v79/tax/calculation"
	"github.com/stripe/stripe-go/v79/tax/transaction"
)

type BillingProcessor struct {
	stripKey      string
	WebhookSecret string
	logger        *observability.Logger
	store         store.Store
}

type PaymentIntentItem struct {
	Id     string `json:"id" binding:"required"`
	Amount int64  `json:"amount" binding:"required"`
}

func New(stripKey string, webhookSecret string, store store.Store, logger *observability.Logger) BillingProcessor {
	stripe.Key = stripKey
	return BillingProcessor{
		stripKey:      stripKey,
		WebhookSecret: webhookSecret,
		store:         store,
		logger:        logger,
	}

}

func (p *BillingProcessor) CreateStripePaymentIntent(ctx context.Context, items []PaymentIntentItem) (string, error) {
	// Create a Tax Calculation for the items being sold
	taxCalculation, err := p.calculateTax(items, stripe.CurrencyCAD, stripe.Address{
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

func (p *BillingProcessor) calculateTax(items []PaymentIntentItem, currency stripe.Currency,
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
		p.logger.Error(context.Background(), "failed to create tax calculation", err)
		return nil, err
	}

	return taxCalculation, nil
}

func (p *BillingProcessor) buildLineItem(i PaymentIntentItem) *stripe.TaxCalculationLineItemParams {
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

func (p *BillingProcessor) CreateSubscription(ctx context.Context, userID uuid.UUID, priceID string) (string, error) {
	ctx = observability.WithFields(ctx, observability.Field{"user_id", userID})
	p.logger.Info(ctx, "Creating subscription for user")

	stripeCustomerID, err := p.store.GetStripeCustomerIDByUserExternalID(ctx, userID)
	if err != nil {
		p.logger.Error(ctx, "failed to get stripe customer id from db", err)
		return "", err
	}

	// Create the subscription
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID), // The customer ID
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID), // The recurring price ID
			},
		},
		PaymentBehavior: stripe.String("default_incomplete"), // Handle payment confirmation manually
		Expand: []*string{
			stripe.String("customer"),                      // Expand the customer object to include email and other details
			stripe.String("latest_invoice.payment_intent"), // Expand the latest invoice object to include the payment intent
		},
	}

	subscriptionInitialized, err := subscription.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create subscription", err)
		return "", err
	}

	// Check if a PaymentIntent exists on the latest invoice
	var clientSecret string
	if subscriptionInitialized.LatestInvoice != nil && subscriptionInitialized.LatestInvoice.PaymentIntent != nil {
		clientSecret = subscriptionInitialized.LatestInvoice.PaymentIntent.ClientSecret
	}

	if clientSecret == "" {
		p.logger.Error(ctx, "failed to get client secret for payment intent", nil)
		return "", errors.New("failed to create payment intent")
	}

	return clientSecret, nil
}

func (p *BillingProcessor) CreateStripeCustomer(ctx context.Context, email string) (string, error) {
	ctx = observability.WithFields(ctx, observability.Field{"email", email})
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	customer, err := customer.New(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create customer", err)
		return "", err
	}

	return customer.ID, nil

}

// Invoke this method in your webhook handler when `payment_intent.succeeded` webhook is received
func (p *BillingProcessor) PaymentIntentSucceeded(ctx context.Context, paymentIntent stripe.PaymentIntent) {
	// Create a Tax Transaction for the successful payment
	params := &stripe.TaxTransactionCreateFromCalculationParams{
		Calculation: stripe.String(paymentIntent.Metadata["tax_calculation"]),
		Reference:   stripe.String("123"),
	}
	params.AddExpand("line_items")

	txn, err := transaction.CreateFromCalculation(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create tax transaction", err)
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{"tax_transaction_id", txn.ID})
	p.logger.Info(ctx, "Tax transaction created")

	//// 2. Update order status to 'Paid' in the system (pseudo-code)
	//orderID := paymentIntent.Metadata["order_id"]
	//err = p.orderService.MarkOrderAsPaid(ctx, orderID, txnID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to mark order as paid", err)
	//	return
	//}
	//
	//p.logger.Info(ctx, "Order marked as paid")
	//
	//// 3. Send confirmation email or notification to the customer (pseudo-code)
	//customerEmail := paymentIntent.ReceiptEmail // From the payment intent
	//err = p.emailService.SendPaymentConfirmation(ctx, customerEmail, orderID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to send confirmation email", err)
	//	return
	//}
	//
	//p.logger.Info(ctx, "Payment confirmation sent to", customerEmail)
	//
	//// 4. Fulfill the order (pseudo-code)
	//err = p.fulfillmentService.FulfillOrder(ctx, orderID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to fulfill order", err)
	//	return
	//}

	//p.logger.Info(ctx, "Order fulfilled", orderID)
}

func (p *BillingProcessor) HandleWebhook(ctx context.Context, event stripe.Event) error {

	// Handle the event
	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal payment intent", err)
			return err
		}
		p.PaymentIntentSucceeded(ctx, paymentIntent)
	}

	return nil
}
